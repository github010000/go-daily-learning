// CPU 프로파일은 "측정값"이 아니라 "표본"이다.
// 100Hz 라는 말은 CPU 시간 10ms 마다 SIGPROF 가 한 번 날아와서
// 그 순간 CPU 위에 올라와 있던 스택 하나를 기록한다는 뜻이다.
// 이 프로그램은 그 표본이 무엇을 보여주고 무엇을 통째로 놓치는지를
// 실제로 프로파일을 떠서 숫자로 확인한다. 외부 모듈 없이 하려고
// pprof 바이너리 포맷(protobuf + gzip)을 아래에서 최소한만 직접 읽는다.

package main

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"runtime"
	"runtime/metrics"
	"runtime/pprof"
	"sort"
	"strings"
	"sync"
	"time"
)

// ---------------------------------------------------------------
// 자료구조
// ---------------------------------------------------------------

// funcStat 은 한 함수에 떨어진 표본 수다.
// Flat 은 그 함수 "몸통"에서 인터럽트된 표본, Cum 은 그 함수와 그 아래
// callee 어딘가에서 인터럽트된 표본이다. 둘을 헷갈리면 최적화 대상을
// 정반대로 고르게 된다. 호출 트리 위쪽 driver 함수는 Cum 이 항상 크지만
// Flat 은 0 에 가깝다. 느린 건 그 아래 어딘가이기 때문이다.
type funcStat struct {
	Flat int
	Cum  int
}

type profileSummary struct {
	Total int // 표본 가중치 합 == 관측된 "10ms 구간" 수
	Funcs map[string]*funcStat
}

func (p *profileSummary) stat(name string) *funcStat {
	st, ok := p.Funcs[name]
	if !ok {
		st = &funcStat{}
		p.Funcs[name] = st
	}
	return st
}

// shortName 은 프로파일에 찍힌 정규화된 이름에서 마지막 점 뒤만 남긴다.
// go run 으로 돌리면 "main.hotLeaf", go test 로 돌리면
// "adv033_cpu_profile.hotLeaf" 처럼 접두사가 달라진다. 접두사를 상수로
// 박아 두면 둘 중 하나에서 반드시 못 찾는다.
func shortName(full string) string {
	return full[strings.LastIndex(full, ".")+1:]
}

func statOf(sum *profileSummary, short string) *funcStat {
	if sum == nil {
		return nil
	}
	for name, st := range sum.Funcs {
		if shortName(name) == short {
			return st
		}
	}
	return nil
}

// flatCum 은 없는 함수를 만났을 때 nil 대신 0,0 을 돌려준다.
// "표본에 안 잡혔다"는 사실 자체가 이 데모에서는 정보이기 때문이다.
func flatCum(sum *profileSummary, short string) (int, int) {
	if st := statOf(sum, short); st != nil {
		return st.Flat, st.Cum
	}
	return 0, 0
}

// ---------------------------------------------------------------
// 시연용 작업 부하
// ---------------------------------------------------------------

// hotLeaf 는 실제로 CPU 를 태우는 잎 노드다. noinline 을 붙이는 이유는
// 인라인으로 접히면 hotMiddle/hotTop 이라는 중간 프레임이 스택에서
// 사라져서 flat 과 cum 의 차이를 보여줄 수 없기 때문이다.
//
//go:noinline
func hotLeaf(x uint64) uint64 {
	for i := 0; i < 400; i++ {
		x = x*6364136223846793005 + 1442695040888963407
	}
	return x
}

//go:noinline
func hotMiddle(x uint64) uint64 { return hotLeaf(x) + 1 }

//go:noinline
func hotTop(x uint64) uint64 { return hotMiddle(x) + 2 }

// tiny 는 호출 한 번이 수 ns 다. 20만 번을 불러도 전체 CPU 시간이
// 표본 주기 10ms 에 한참 못 미친다. "많이 불렸다"와 "표본에 잡힌다"는
// 전혀 다른 말이라는 걸 보여주려고 일부러 넣었다.
//
//go:noinline
func tiny(x uint64) uint64 { return x*3 + 1 }

func callTinyMany(times int) uint64 {
	x := uint64(7)
	for i := 0; i < times; i++ {
		x = tiny(x)
	}
	return x
}

// burnCPU 는 d 동안 CPU 를 태운다. 벽시계로 끊는 이유는 프로파일이
// 결국 "CPU 시간" 기준이라, 벽시계 d 동안 CPU 를 계속 물고 있으면
// 기대 표본 수를 대략 d*100 으로 예측할 수 있기 때문이다.
// 반환값을 남기는 이유는 컴파일러가 루프를 통째로 지우지 못하게 하려는 것.
func burnCPU(d time.Duration) uint64 {
	deadline := time.Now().Add(d)
	x := uint64(1)
	for time.Now().Before(deadline) {
		for i := 0; i < 64; i++ {
			x = hotTop(x)
		}
	}
	return x
}

// burnCPUParallel 은 n 개의 goroutine 으로 같은 시간만큼 CPU 를 태운다.
// 벽시계는 그대로인데 프로세스가 먹은 CPU 초는 n 배가 된다. 표본 수가
// 어느 쪽을 따라가는지 보려고 만들었다.
func burnCPUParallel(n int, d time.Duration) {
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			burnCPU(d)
		}()
	}
	wg.Wait()
}

// blockOnSyscall 은 파이프 read(2) 안에서 M 을 커널에 붙잡아 둔다.
// 이 구간에는 Go 코드가 CPU 위에 없다. SIGPROF 는 "지금 실행 중인
// Go 스택"을 찍는 도구이므로 여기서는 찍을 것이 없다.
func blockOnSyscall(d time.Duration) error {
	r, w, err := os.Pipe()
	if err != nil {
		return err
	}
	defer r.Close()
	go func() {
		time.Sleep(d)
		_, _ = w.Write([]byte{0})
		_ = w.Close()
	}()
	buf := make([]byte, 1)
	_, err = r.Read(buf) // 여기서 스레드가 syscall 에 블록된다
	return err
}

// cpuSeconds 는 이 프로세스가 실제로 태운 user CPU 시간(초)이다.
// 프로파일 표본 수와 대략 100:1 로 맞아야 정상이다. 맞지 않으면
// 그 차이가 곧 이 글의 주제다.
func cpuSeconds() float64 {
	s := []metrics.Sample{{Name: "/cpu/classes/user:cpu-seconds"}}
	metrics.Read(s)
	if s[0].Value.Kind() == metrics.KindFloat64 {
		return s[0].Value.Float64()
	}
	return -1
}

// runProfile 은 work 를 CPU 프로파일로 감싸 실행하고 요약을 돌려준다.
// StartCPUProfile 은 프로세스 전역 상태라 동시에 두 번 못 쓴다. 그래서
// 감싸는 단위를 함수 하나로 고정했다(테스트도 같은 경로를 쓴다).
// CPU_PROFILE_OUT 을 지정하면 원본 바이트를 파일로 떨어뜨려
// go tool pprof 로 직접 열어볼 수 있게 한다.
func runProfile(work func()) (*profileSummary, time.Duration, error) {
	var buf bytes.Buffer
	if err := pprof.StartCPUProfile(&buf); err != nil {
		return nil, 0, err
	}
	start := time.Now()
	work()
	elapsed := time.Since(start)
	pprof.StopCPUProfile() // 여기서 반환해야 프로파일이 flush 된다

	if path := os.Getenv("CPU_PROFILE_OUT"); path != "" {
		_ = os.WriteFile(path, buf.Bytes(), 0o644)
	}

	sum, err := decodeCPUProfile(buf.Bytes())
	if err != nil {
		return nil, elapsed, err
	}
	return sum, elapsed, nil
}

// ---------------------------------------------------------------
// 최소 protobuf 리더
// 여기서 필요한 건 "표본 총 개수"와 "함수별 flat/cum" 뿐이라
// 나머지 필드(mapping, label, comment)는 전부 건너뛴다.
// ---------------------------------------------------------------

type pbReader struct {
	buf []byte
	pos int
}

func (r *pbReader) done() bool { return r.pos >= len(r.buf) }

func (r *pbReader) uvarint() (uint64, error) {
	var x uint64
	var s uint
	for {
		if r.pos >= len(r.buf) {
			return 0, io.ErrUnexpectedEOF
		}
		b := r.buf[r.pos]
		r.pos++
		if b < 0x80 {
			return x | uint64(b)<<s, nil
		}
		x |= uint64(b&0x7f) << s
		s += 7
		if s >= 64 {
			return 0, fmt.Errorf("varint 이 64비트를 넘었다")
		}
	}
}

func (r *pbReader) tag() (field, wire int, err error) {
	k, err := r.uvarint()
	if err != nil {
		return 0, 0, err
	}
	return int(k >> 3), int(k & 7), nil
}

func (r *pbReader) bytes() ([]byte, error) {
	n, err := r.uvarint()
	if err != nil {
		return nil, err
	}
	if n > uint64(len(r.buf)-r.pos) {
		return nil, io.ErrUnexpectedEOF
	}
	b := r.buf[r.pos : r.pos+int(n)]
	r.pos += int(n)
	return b, nil
}

func (r *pbReader) skip(wire int) error {
	switch wire {
	case 0:
		_, err := r.uvarint()
		return err
	case 1:
		if r.pos+8 > len(r.buf) {
			return io.ErrUnexpectedEOF
		}
		r.pos += 8
		return nil
	case 2:
		_, err := r.bytes()
		return err
	case 5:
		if r.pos+4 > len(r.buf) {
			return io.ErrUnexpectedEOF
		}
		r.pos += 4
		return nil
	}
	return fmt.Errorf("모르는 wire type %d", wire)
}

// repeated uint64 는 proto3 에서 packed(wire 2)로도, 낱개(wire 0)로도
// 올 수 있다. Go 런타임은 packed 로 쓰지만 손으로 만든 입력이 낱개일 수
// 있으므로 양쪽을 다 받는다. 여기서 한쪽만 처리하면 표본이 조용히 0 이 된다.
func readRepeatedUint(b []byte, want int) ([]uint64, error) {
	var out []uint64
	r := &pbReader{buf: b}
	for !r.done() {
		field, wire, err := r.tag()
		if err != nil {
			return nil, err
		}
		switch {
		case field == want && wire == 0:
			v, err := r.uvarint()
			if err != nil {
				return nil, err
			}
			out = append(out, v)
		case field == want && wire == 2:
			body, err := r.bytes()
			if err != nil {
				return nil, err
			}
			p := &pbReader{buf: body}
			for !p.done() {
				v, err := p.uvarint()
				if err != nil {
					return nil, err
				}
				out = append(out, v)
			}
		default:
			if err := r.skip(wire); err != nil {
				return nil, err
			}
		}
	}
	return out, nil
}

func decodeLineMsg(b []byte) (uint64, error) {
	r := &pbReader{buf: b}
	var fnID uint64
	for !r.done() {
		field, wire, err := r.tag()
		if err != nil {
			return 0, err
		}
		if field == 1 && wire == 0 {
			fnID, err = r.uvarint()
			if err != nil {
				return 0, err
			}
			continue
		}
		if err := r.skip(wire); err != nil {
			return 0, err
		}
	}
	return fnID, nil
}

// Location 하나가 인라인된 여러 프레임을 담을 수 있다. 그래서 함수 id 를
// 하나가 아니라 슬라이스로 돌려준다. leaf 가 앞에 온다.
func decodeLocationMsg(b []byte) (uint64, []uint64, error) {
	r := &pbReader{buf: b}
	var id uint64
	var fns []uint64
	for !r.done() {
		field, wire, err := r.tag()
		if err != nil {
			return 0, nil, err
		}
		switch {
		case field == 1 && wire == 0:
			id, err = r.uvarint()
			if err != nil {
				return 0, nil, err
			}
		case field == 4 && wire == 2:
			body, err := r.bytes()
			if err != nil {
				return 0, nil, err
			}
			fnID, err := decodeLineMsg(body)
			if err != nil {
				return 0, nil, err
			}
			if fnID != 0 { // 0 은 "네이티브 프레임"이라 함수 이름이 없다
				fns = append(fns, fnID)
			}
		default:
			if err := r.skip(wire); err != nil {
				return 0, nil, err
			}
		}
	}
	return id, fns, nil
}

func decodeFunctionMsg(b []byte) (id, nameIdx uint64, err error) {
	r := &pbReader{buf: b}
	for !r.done() {
		var field, wire int
		field, wire, err = r.tag()
		if err != nil {
			return
		}
		switch {
		case field == 1 && wire == 0:
			id, err = r.uvarint()
		case field == 2 && wire == 0:
			nameIdx, err = r.uvarint()
		default:
			err = r.skip(wire)
		}
		if err != nil {
			return
		}
	}
	return
}

// decodeCPUProfile 은 gzip 압축된 protobuf 를 풀어 flat/cum 요약을 만든다.
// 핵심 아이디어: 표본 하나는 "스택 전체"를 들고 있고, flat/cum 구분은
// 저장 시점이 아니라 이렇게 집계 시점에 붙는다. 그래서 같은 파일로
// 보고 싶은 관점을 얼마든지 바꿔 볼 수 있다.
func decodeCPUProfile(raw []byte) (*profileSummary, error) {
	zr, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("gzip 헤더를 못 읽었다: %w", err)
	}
	msg, err := io.ReadAll(zr)
	if err != nil {
		return nil, err
	}
	if err := zr.Close(); err != nil {
		return nil, err
	}

	var (
		strs       []string
		funcNameAt = map[uint64]uint64{}   // function id -> string_table 인덱스
		locFuncs   = map[uint64][]uint64{} // location id -> function id (leaf 우선)
		samples    []rawSample
	)

	r := &pbReader{buf: msg}
	for !r.done() {
		field, wire, err := r.tag()
		if err != nil {
			return nil, err
		}
		if wire != 2 {
			if err := r.skip(wire); err != nil {
				return nil, err
			}
			continue
		}
		body, err := r.bytes()
		if err != nil {
			return nil, err
		}
		switch field {
		case 2: // Profile.sample
			s, err := decodeSampleMsg(body)
			if err != nil {
				return nil, err
			}
			samples = append(samples, s)
		case 4: // Profile.location
			id, fns, err := decodeLocationMsg(body)
			if err != nil {
				return nil, err
			}
			locFuncs[id] = fns
		case 5: // Profile.function
			id, nameIdx, err := decodeFunctionMsg(body)
			if err != nil {
				return nil, err
			}
			funcNameAt[id] = nameIdx
		case 6: // Profile.string_table
			strs = append(strs, string(body))
		}
	}

	nameOf := func(fnID uint64) string {
		idx, ok := funcNameAt[fnID]
		if !ok || idx >= uint64(len(strs)) {
			return fmt.Sprintf("fn#%d", fnID)
		}
		return strs[idx]
	}

	sum := &profileSummary{Funcs: map[string]*funcStat{}}
	for _, s := range samples {
		if len(s.locs) == 0 || len(s.vals) == 0 {
			continue
		}
		w := int(s.vals[0]) // sample_type[0] 은 samples/count
		if w <= 0 {
			continue
		}
		sum.Total += w

		var stack []string
		for _, locID := range s.locs { // s.locs 는 leaf 가 먼저
			for _, fnID := range locFuncs[locID] {
				stack = append(stack, nameOf(fnID))
			}
		}
		if len(stack) == 0 {
			continue
		}

		// flat 은 leaf 한 곳에만. cum 은 스택에 등장하는 모든 함수에.
		sum.stat(stack[0]).Flat += w
		seen := make(map[string]bool, len(stack))
		for _, name := range stack {
			if seen[name] {
				continue // 재귀 호출은 한 표본에서 한 번만 센다
			}
			seen[name] = true
			sum.stat(name).Cum += w
		}
	}
	return sum, nil
}

type rawSample struct {
	locs []uint64 // leaf 가 먼저, root 가 마지막
	vals []int64
}

func decodeSampleMsg(b []byte) (rawSample, error) {
	var s rawSample
	locs, err := readRepeatedUint(b, 1)
	if err != nil {
		return s, err
	}
	vals, err := readRepeatedUint(b, 2)
	if err != nil {
		return s, err
	}
	s.locs = locs
	for _, v := range vals {
		s.vals = append(s.vals, int64(v))
	}
	return s, nil
}

// ---------------------------------------------------------------
// 출력
// ---------------------------------------------------------------

func printTable(sum *profileSummary, limit int) {
	type row struct {
		name string
		st   funcStat
	}
	rows := make([]row, 0, len(sum.Funcs))
	for name, st := range sum.Funcs {
		rows = append(rows, row{shortName(name), *st})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].st.Cum != rows[j].st.Cum {
			return rows[i].st.Cum > rows[j].st.Cum
		}
		if rows[i].st.Flat != rows[j].st.Flat {
			return rows[i].st.Flat > rows[j].st.Flat
		}
		return rows[i].name < rows[j].name
	})
	fmt.Printf("  %6s %6s  %s\n", "flat", "cum", "함수")
	for i, r := range rows {
		if i >= limit {
			fmt.Printf("  ... (나머지 %d 개 함수 생략)\n", len(rows)-limit)
			break
		}
		fmt.Printf("  %6d %6d  %s\n", r.st.Flat, r.st.Cum, r.name)
	}
}

func ratio(samples int, secs float64) float64 {
	if secs <= 0 {
		return 0
	}
	return float64(samples) / secs
}

func demoSamplingRate() *profileSummary {
	fmt.Println("== 1) CPU 를 태운 구간: 표본 수는 'CPU 초'에 비례한다 ==")
	before := cpuSeconds()
	sum, elapsed, err := runProfile(func() { burnCPU(300 * time.Millisecond) })
	if err != nil {
		fmt.Println("  프로파일 실패:", err)
		return nil
	}
	used := cpuSeconds() - before
	fmt.Printf("  벽시계 %.0fms, 표본 %d개, 벽시계 기준 기대값 %.0f개\n",
		elapsed.Seconds()*1000, sum.Total, elapsed.Seconds()*100)
	fmt.Printf("  프로세스 user CPU %.3fs -> 표본/CPU초 = %.1f\n", used, ratio(sum.Total, used))
	printTable(sum, 6)
	fmt.Println()
	return sum
}

func demoTinyCalls() {
	fmt.Println("== 2) 아주 짧은 함수 200000 회 호출 ==")
	const n = 200000
	start := time.Now()
	v := callTinyMany(n)
	wall := time.Since(start)

	sum, _, err := runProfile(func() { callTinyMany(n) })
	if err != nil {
		fmt.Println("  프로파일 실패:", err)
		return
	}
	flat, cum := flatCum(sum, "tiny")
	fmt.Printf("  %d 회 호출, 반환값 %d, 벽시계 %v, 잡힌 표본 %d개\n",
		n, v, wall.Round(time.Microsecond), sum.Total)
	fmt.Printf("  tiny flat=%d cum=%d  <- 호출 횟수는 표본 수를 만들어 주지 않는다\n", flat, cum)
	fmt.Println()
}

func demoBlockedSyscall() {
	fmt.Println("== 3) syscall 안에서 기다린 구간 ==")
	before := cpuSeconds()
	sum, elapsed, err := runProfile(func() { _ = blockOnSyscall(250 * time.Millisecond) })
	if err != nil {
		fmt.Println("  프로파일 실패:", err)
		return
	}
	used := cpuSeconds() - before
	fmt.Printf("  벽시계 %.0fms 대기, 표본 %d개, user CPU %.4fs\n",
		elapsed.Seconds()*1000, sum.Total, used)
	fmt.Println("  커널에서 잠든 시간은 Go 스택 위에 없다. SIGPROF 는 CPU 를 쓴 프로세스에만 온다.")
	fmt.Println()
}

func demoFlatVsCum(sum *profileSummary) {
	fmt.Println("== 4) flat 과 cum 을 헷갈리면 안 되는 이유 ==")
	if sum == nil {
		fmt.Println("  (1번 구간이 실패해 생략)")
		return
	}
	tf, tc := flatCum(sum, "hotTop")
	mf, mc := flatCum(sum, "hotMiddle")
	lf, lc := flatCum(sum, "hotLeaf")
	fmt.Printf("  hotTop    flat=%d cum=%d\n", tf, tc)
	fmt.Printf("  hotMiddle flat=%d cum=%d\n", mf, mc)
	fmt.Printf("  hotLeaf   flat=%d cum=%d\n", lf, lc)
	fmt.Println("  세 함수의 cum 이 같은 이유: 스택이 늘 hotTop->hotMiddle->hotLeaf 로 같기 때문.")
	fmt.Println("  flat 이 hotLeaf 에 몰리는 이유: 곱셈-덧셈이 그 몸통에서만 일어나기 때문.")
	fmt.Println()
}

func demoParallelCPU() {
	fmt.Println("== 5) 병렬 구간: 표본은 벽시계가 아니라 CPU 초를 센다 ==")
	const workers = 4
	sum, elapsed, err := runProfile(func() { burnCPUParallel(workers, 200*time.Millisecond) })
	if err != nil {
		fmt.Println("  프로파일 실패:", err)
		return
	}
	fmt.Printf("  GOMAXPROCS=%d, goroutine %d개, 벽시계 %.0fms, 표본 %d개\n",
		runtime.GOMAXPROCS(0), workers, elapsed.Seconds()*1000, sum.Total)
	fmt.Printf("  벽시계 기준 기대값은 %.0f개. 코어가 놀지 않았다면 그보다 훨씬 크다.\n",
		elapsed.Seconds()*100)
}

func main() {
	fmt.Println("== CPU profile 은 '측정'이 아니라 100Hz 표본이다 ==")
	fmt.Printf("GOMAXPROCS=%d, 표본 주기 10ms, 한 번에 스택 하나만 기록한다\n\n",
		runtime.GOMAXPROCS(0))

	sum := demoSamplingRate()
	demoTinyCalls()
	demoBlockedSyscall()
	demoFlatVsCum(sum)
	demoParallelCPU()
}