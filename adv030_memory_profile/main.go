// 메모리 프로파일에서 inuse_space 와 alloc_space 가 각각 무엇을 재는지,
// 표본 추출률(MemProfileRate)이 그 수치를 어떻게 흔드는지,
// "누수"와 "과다 할당"을 수치로 구분하는지 보여준다.
//
// 이 파일은 숫자를 두 종류로 나란히 찍는다.
//   - 정확값: runtime.MemStats 의 TotalAlloc / HeapAlloc. 표본과 무관하다.
//   - 표본값: 힙 프로파일 레코드(runtime.MemProfile)의 합. 이건 "할당한
//     바이트"가 아니라 "표본으로 잡힌 바이트"라서, 그대로 읽으면 실제보다
//     훨씬 작게 나온다. pprof 는 여기에 역산 계수를 곱해서 보여준다.
// 두 값을 같은 창(window)에서 비교해야 오차의 크기가 보인다.
package main

import (
	"bufio"
	"bytes"
	"fmt"
	"math"
	"os"
	"regexp"
	"runtime"
	"runtime/pprof"
	"strconv"
)

// heapHeaderRE 는 text 형식 힙 프로파일의 첫 줄을 잡는다.
//
//	heap profile: 3: 1234 [15: 5678] @ heap/524288
//	              inuse    [ alloc ]   추출 간격
//	              객체수/바이트
//
// 대괄호 밖이 "지금 살아 있는 것"(inuse_space), 안이 "지금까지 만든 것
// 전부"(alloc_space)다. 두 값이 같은 레코드에서 동시에 나오므로, 이 한
// 줄만 읽어도 두 지표를 같은 시점에서 비교할 수 있다.
var heapHeaderRE = regexp.MustCompile(`^heap profile:\s*(\d+):\s*(\d+)\s*\[(\d+):\s*(\d+)\]\s*@\s*heap/(\d+)`)

// HeapSnapshot 은 힙 프로파일 머리말 한 장을 요약한 값이다.
type HeapSnapshot struct {
	InuseObjects int64
	InuseBytes   int64
	AllocObjects int64
	AllocBytes   int64
	Rate         int64
}

// parseHeapHeader 는 머리말 한 줄을 파싱한다. 순수 함수로 떼어 둔 이유는
// 이 텍스트 포맷을 읽는 규칙 자체가 학습 대상이기 때문이다. 여기서 inuse
// 와 alloc 을 뒤집어 읽으면 "누수 없음"과 "누수 있음"이 정반대로 뒤집힌다.
func parseHeapHeader(line string) (HeapSnapshot, bool) {
	m := heapHeaderRE.FindStringSubmatch(line)
	if m == nil {
		return HeapSnapshot{}, false
	}
	var v [5]int64
	for i := range v {
		n, err := strconv.ParseInt(m[i+1], 10, 64)
		if err != nil {
			return HeapSnapshot{}, false
		}
		v[i] = n
	}
	return HeapSnapshot{
		InuseObjects: v[0],
		InuseBytes:   v[1],
		AllocObjects: v[2],
		AllocBytes:   v[3],
		Rate:         v[4],
	}, true
}

// readHeapText 는 debug=1 형식(text)으로 힙 프로파일을 한 장 뜬다.
// 이 호출은 내부에서 GC 를 강제한다. inuse 를 세려면 이미 죽은 객체를
// 먼저 회수해야 하기 때문이다. 즉 프로파일 한 장의 가격은 STW 한 번이다.
// 상시 모니터링에 그대로 넣으면 그 자체가 지연 스파이크가 된다.
func readHeapText() (HeapSnapshot, error) {
	var buf bytes.Buffer
	if err := pprof.Lookup("heap").WriteTo(&buf, 1); err != nil {
		return HeapSnapshot{}, err
	}
	sc := bufio.NewScanner(&buf)
	for sc.Scan() {
		if snap, ok := parseHeapHeader(sc.Text()); ok {
			return snap, nil
		}
	}
	if err := sc.Err(); err != nil {
		return HeapSnapshot{}, err
	}
	return HeapSnapshot{}, fmt.Errorf("힙 프로파일 머리말을 찾지 못했습니다")
}

// ExactTotals 는 MemStats 에서 읽은 정확값이다. 표본 추출률과 무관하다.
// TotalAlloc 은 태어난 뒤 지금까지 할당한 누적 바이트, HeapAlloc 은 지금
// 살아 있는 힙 바이트다. 프로파일 수치를 검증할 유일한 기준이다.
type ExactTotals struct {
	TotalAlloc uint64
	HeapAlloc  uint64
	Mallocs    uint64
	Frees      uint64
}

func exactTotals() ExactTotals {
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms) // 이것도 STW 를 유발한다
	return ExactTotals{
		TotalAlloc: ms.TotalAlloc,
		HeapAlloc:  ms.HeapAlloc,
		Mallocs:    ms.Mallocs,
		Frees:      ms.Frees,
	}
}

// ProfileTotals 는 힙 프로파일 레코드를 전부 더한 값이다. 레코드의
// AllocBytes/FreeBytes 는 표본으로 잡힌 객체 크기의 합이지 추정치가 아니다.
// inuse 는 사이트별로 AllocBytes - FreeBytes 를 더해서 만든다.
type ProfileTotals struct {
	AllocBytes   int64
	AllocObjects int64
	InuseBytes   int64
	InuseObjects int64
}

func profileTotals() ProfileTotals {
	var recs []runtime.MemProfileRecord
	for {
		n, ok := runtime.MemProfile(recs, false)
		if ok {
			recs = recs[:n]
			break
		}
		// 레코드 수는 실행 중에 늘어난다. 넉넉히 잡고 다시 읽는다.
		recs = make([]runtime.MemProfileRecord, n+n/4+16)
	}
	var t ProfileTotals
	for i := range recs {
		r := &recs[i]
		t.AllocBytes += r.AllocBytes
		t.AllocObjects += r.AllocObjects
		t.InuseBytes += r.AllocBytes - r.FreeBytes
		t.InuseObjects += r.AllocObjects - r.FreeObjects
	}
	return t
}

// scaleRawSample 은 pprof 가 힙 프로파일 원자료에 적용하는 역산이다
// (pprof 내부의 scaleHeapSample 과 같은 식). 객체가 표본으로 잡힐 확률이
// 크기에 비례한다는 사실을 이용해, 평균 크기에서의 포함 확률
// p = 1 - exp(-avgSize/rate) 를 구하고 1/p 를 곱한다. 이걸 안 하면
// 프로파일 숫자를 실제 할당량으로 오독하게 된다.
func scaleRawSample(objects, bytes int64, rate int) (int64, int64) {
	if objects <= 0 || bytes <= 0 || rate <= 1 {
		return objects, bytes
	}
	avg := float64(bytes) / float64(objects)
	p := 1 - math.Exp(-avg/float64(rate))
	if p <= 0 {
		return objects, bytes
	}
	scale := 1 / p
	return int64(float64(objects) * scale), int64(float64(bytes) * scale)
}

var (
	// churnKeep 는 방금 만든 객체를 잠깐 매달아 두는 전역이다. 다음 반복이
	// 덮어쓰므로 회수 대상이 되고, 컴파일러 입장에서는 "이 객체는 힙으로
	// escape 한다"는 사실만 남는다. escape 하지 않으면 make 가 스택으로
	// 올라가서 힙 프로파일에 아예 안 잡힌다. 이게 가장 흔한 함정이다.
	churnKeep any

	// leakStore 는 회수되지 않는 참조를 흉내 낸다. 프로세스가 살아 있는 한
	// GC 는 이 슬라이스를 통해 모든 원소를 살아 있는 것으로 본다.
	leakStore [][]byte
)

//go:noinline
func allocOne(size int) []byte {
	b := make([]byte, size)
	if size > 0 {
		b[0] = 1 // 실제로 메모리를 건드려야 페이지가 붙는다
	}
	return b
}

// allocChurn 은 size 바이트 객체를 n 번 만들고 즉시 버린다.
// allocOne 을 noinline 으로 둔 이유는 인라인되면 size 가 상수로 전파되어
// 컴파일러가 할당을 통째로 스택으로 옮겨 버리기 때문이다.
func allocChurn(n, size int) {
	for i := 0; i < n; i++ {
		churnKeep = allocOne(size)
	}
}

// leakGrow 는 만든 슬라이스를 전역에 계속 매달아 둔다. GC 가 돌아도
// 회수되지 않으므로 inuse 가 그대로 남는다. alloc 만 보면 allocChurn 과
// 구분되지 않는다는 점이 핵심이다.
func leakGrow(n, size int) {
	for i := 0; i < n; i++ {
		b := allocOne(size)
		leakStore = append(leakStore, b)
	}
}

// releaseLeak 은 붙잡고 있던 참조를 놓아 다음 GC 에서 회수되게 한다.
func releaseLeak() { leakStore = nil }

// resetSampleState 는 mcache 에 남아 있는 "다음 표본까지 남은 바이트"
// 카운터를 되돌린다. runtime.MemProfileRate 를 바꿔도 이 카운터는 즉시
// 다시 계산되지 않는다. 그래서 한 번 큰 추출 간격을 쓰고 나면 그 뒤로
// 수십 MB 를 할당해도 표본이 하나도 안 잡히는 일이 생긴다.
// 추출률 1 은 "모든 할당을 기록"이라 반드시 표본이 잡히고, 그 표본을
// 만드는 profilealloc 이 카운터를 새로 써 준다.
func resetSampleState() {
	prev := runtime.MemProfileRate
	runtime.MemProfileRate = 1
	churnKeep = allocOne(64)
	runtime.MemProfileRate = prev
	churnKeep = allocOne(64) // 복원된 추출률로 카운터를 다시 계산시킨다
}

// AllocInuse 는 한 번의 할당 폭주를 두 관점에서 기록한 값이다.
type AllocInuse struct {
	ExactAllocBytes   uint64
	HeapAllocBefore   uint64
	HeapAllocAfter    uint64
	ProfileAllocBytes int64
	ProfileInuseBytes int64
	Rate              int
}

// measureChurn 은 size 바이트 객체 n 개를 만들고 전부 버린 뒤,
// alloc 증가분과 inuse 를 정확값/표본값 양쪽에서 돌려준다. 증가분(delta)으로
// 비교하는 이유는 프로세스에 이미 들어와 있는 다른 메모리를 빼기 위해서다.
func measureChurn(n, size int) AllocInuse {
	resetSampleState()
	runtime.GC()
	p0 := profileTotals()
	e0 := exactTotals()

	allocChurn(n, size)

	runtime.GC()
	p1 := profileTotals()
	e1 := exactTotals()

	return AllocInuse{
		ExactAllocBytes:   e1.TotalAlloc - e0.TotalAlloc,
		HeapAllocBefore:   e0.HeapAlloc,
		HeapAllocAfter:    e1.HeapAlloc,
		ProfileAllocBytes: p1.AllocBytes - p0.AllocBytes,
		ProfileInuseBytes: p1.InuseBytes - p0.InuseBytes,
		Rate:              runtime.MemProfileRate,
	}
}

// LeakChurn 은 과다 할당과 누수를 같은 조건에서 재 본 결과다.
type LeakChurn struct {
	ChurnHeapAlloc    uint64
	LeakHeapAlloc     uint64
	AfterHeapAlloc    uint64
	ChurnExactAlloc   uint64
	LeakExactAlloc    uint64
	ChurnProfileInuse int64
	LeakProfileInuse  int64
}

// leakChurnExperiment 는 "만들고 버리기"와 "만들고 붙잡기"를 연달아
// 수행하며 각 단계의 inuse 를 기록한다. 두 경우 모두 alloc 은 비슷하게
// 늘지만 inuse 는 전혀 다르게 움직인다.
func leakChurnExperiment(n, size int) LeakChurn {
	releaseLeak()
	resetSampleState()
	runtime.GC()
	p0 := profileTotals()
	e0 := exactTotals()

	allocChurn(n, size)
	runtime.GC()
	p1 := profileTotals()
	e1 := exactTotals()

	leakGrow(n, size)
	runtime.GC()
	p2 := profileTotals()
	e2 := exactTotals()

	releaseLeak()
	runtime.GC()
	e3 := exactTotals()

	return LeakChurn{
		ChurnHeapAlloc:    e1.HeapAlloc,
		LeakHeapAlloc:     e2.HeapAlloc,
		AfterHeapAlloc:    e3.HeapAlloc,
		ChurnExactAlloc:   e1.TotalAlloc - e0.TotalAlloc,
		LeakExactAlloc:    e2.TotalAlloc - e1.TotalAlloc,
		ChurnProfileInuse: p1.InuseBytes - p0.InuseBytes,
		LeakProfileInuse:  p2.InuseBytes - p0.InuseBytes,
	}
}

// SamplingResult 는 한 추출률에서 잰 결과다.
type SamplingResult struct {
	Rate           int
	ProfileAllocB  int64  // 프로파일 원자료(표본 합) 증가분
	ProfileSamples int64  // 표본으로 잡힌 객체 수 증가분
	ExactAllocB    uint64 // MemStats.TotalAlloc 증가분(정확값)
	ScaledAllocB   int64  // pprof 가 보여주는 값(원자료에 역산 계수를 곱한 것)
}

// measureAlloc 은 추출률을 rate 로 맞춘 뒤 fn 이 유발한 할당을 프로파일
// 원자료와 MemStats 정확값으로 동시에 잰다.
func measureAlloc(rate int, fn func()) SamplingResult {
	prev := runtime.MemProfileRate
	runtime.MemProfileRate = rate
	defer func() { runtime.MemProfileRate = prev }()
	resetSampleState()

	runtime.GC()
	p0 := profileTotals()
	e0 := exactTotals()

	fn()

	runtime.GC()
	p1 := profileTotals()
	e1 := exactTotals()

	rawB := p1.AllocBytes - p0.AllocBytes
	rawN := p1.AllocObjects - p0.AllocObjects
	_, scaled := scaleRawSample(rawN, rawB, rate)
	return SamplingResult{
		Rate:           rate,
		ProfileAllocB:  rawB,
		ProfileSamples: rawN,
		ExactAllocB:    e1.TotalAlloc - e0.TotalAlloc,
		ScaledAllocB:   scaled,
	}
}

// writeHeapProfile 은 go tool pprof 로 여는 파일(protobuf)을 만든다.
// debug=0 이 배포용 형식이고, debug=1 은 사람이 읽는 텍스트다.
func writeHeapProfile(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := pprof.Lookup("heap").WriteTo(f, 0); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

func ratioOf(prof int64, exact uint64) float64 {
	if exact == 0 {
		return 0
	}
	return float64(prof) / float64(exact)
}

func demoChurn() {
	fmt.Println("[1] 만들고 즉시 버리는 루프 8 MiB")
	res := measureChurn(2000, 4096)
	fmt.Printf("  정확값   TotalAlloc +%d 바이트, HeapAlloc %d -> %d\n",
		res.ExactAllocBytes, res.HeapAllocBefore, res.HeapAllocAfter)
	fmt.Printf("  원자료   alloc %+d 바이트, inuse %+d 바이트 (추출률 %d)\n",
		res.ProfileAllocBytes, res.ProfileInuseBytes, res.Rate)
	if snap, err := readHeapText(); err == nil {
		fmt.Printf("  머리말   inuse %d객체/%d바이트 [alloc %d객체/%d바이트] @ heap/%d\n",
			snap.InuseObjects, snap.InuseBytes, snap.AllocObjects, snap.AllocBytes, snap.Rate)
	}
	fmt.Println("  => alloc 은 '지금까지 만든 양', inuse 는 '지금 살아 있는 양'.")
	fmt.Println("     버린 메모리는 alloc 에만 흔적을 남기고 inuse 에는 남지 않는다.")
}

func demoLeakVsChurn() {
	fmt.Println("\n[2] 같은 8 MiB 를 (a) 버리기 vs (b) 붙잡기")
	res := leakChurnExperiment(2000, 4096)
	fmt.Printf("  (a) 버리기 : 정확 alloc +%d, HeapAlloc %d, 프로파일 inuse %+d\n",
		res.ChurnExactAlloc, res.ChurnHeapAlloc, res.ChurnProfileInuse)
	fmt.Printf("  (b) 붙잡기 : 정확 alloc +%d, HeapAlloc %d, 프로파일 inuse %+d\n",
		res.LeakExactAlloc, res.LeakHeapAlloc, res.LeakProfileInuse)
	fmt.Printf("  (b) 참조 해제 뒤 HeapAlloc %d\n", res.AfterHeapAlloc)
	fmt.Println("  => alloc 만 보면 (a)와 (b)가 구별되지 않는다. 갈라내는 지표는 inuse.")
}

func demoRates() {
	fmt.Println("\n[3] 같은 8 MiB 작업을 추출률만 바꿔 재기")
	for _, rate := range []int{1, 64 << 10, 512 << 10, 8 << 20} {
		res := measureAlloc(rate, func() { allocChurn(2000, 4096) })
		fmt.Printf("  rate=%-8d 표본객체=%-6d 원자료=%-9d pprof보정=%-9d 정확값=%-9d 원자료/정확=%.3f\n",
			res.Rate, res.ProfileSamples, res.ProfileAllocB, res.ScaledAllocB,
			res.ExactAllocB, ratioOf(res.ProfileAllocB, res.ExactAllocB))
	}
	fmt.Println("  => 원자료는 표본 합이라 항상 실제보다 작다. pprof 는 여기에")
	fmt.Println("     1/(1-exp(-avgSize/rate)) 를 곱해 보정한다. 표본이 0 개면 보정할")
	fmt.Println("     대상이 없어 0 이 된다. rate 가 측정량보다 크면 아무것도 못 본다.")
}

func demoFiles() {
	fmt.Println("\n[4] 파일로 떠서 go tool pprof 로 여는 경로")
	path := "mem.prof"
	if err := writeHeapProfile(path); err != nil {
		fmt.Println("  프로파일 저장 실패:", err)
		return
	}
	st, err := os.Stat(path)
	if err != nil {
		fmt.Println("  저장한 파일을 찾을 수 없음:", err)
		return
	}
	fmt.Printf("  %s (%d 바이트) 생성\n", path, st.Size())
	fmt.Println("  go tool pprof -inuse_space", path, " / -alloc_space 도 같은 파일")
}

func main() {
	fmt.Println("메모리 프로파일: inuse 와 alloc, 그리고 표본 오차")
	fmt.Println("==============================================")
	demoChurn()
	demoLeakVsChurn()
	demoRates()
	demoFiles()
}