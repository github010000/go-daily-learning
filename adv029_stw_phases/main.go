// 이 프로그램은 Go GC 의 "정지 구간"이 실제로 어디에 있는지, 그리고 사람들이
// STW 로 오해하는 지연 스파이크가 어디서 오는지를 수치로 보여준다.
//
//	go run .
//	GODEBUG=gctrace=1 go run .   <- 사이클마다 STW 두 조각이 gcN(a+c ms) 로 찍힌다
//
// 왜 이렇게 세 부분으로 나눴나:
//  1) STW 자체의 크기   - debug.GCStats / runtime/metrics 의 pause 지표
//  2) GC 의 CPU 구성    - /cpu/classes/gc/* 로 assist, dedicated, pause 를 분해
//  3) 체감 지연         - 할당하는 goroutine 자신의 호출 구간을 직접 잰다
// STW 를 "작다"고 말하려면 1 이 필요하고, "그런데 왜 느린가"를 말하려면 2 와 3 이 필요하다.
package main

import (
	"fmt"
	"math"
	"runtime"
	"runtime/debug"
	"runtime/metrics"
	"sort"
	"time"
)

var (
	// allocSink 는 "살아있는 참조"를 유지하기 위한 전역이다.
	// 왜 필요한가: make([]byte, n) 결과를 아무도 안 쓰면 컴파일러가 할당을 지우거나
	// 스택으로 옮겨서(escape analysis) GC 가 볼 힙 쓰레기가 아예 생기지 않는다.
	allocSink [][]byte

	// liveSink 는 힙 목표(heap goal)를 만들어 두는 상주 집합이다.
	// 왜 필요한가: 상주 집합이 수 KB 뿐이면 GC 목표도 수 KB 라 매 할당마다 사이클이
	// 돌아 버려서, 측정하려는 assist 효과가 아니라 병리적 상황만 보게 된다.
	liveSink []byte
)

// knownMetrics 는 이 런타임이 실제로 제공하는 지표 이름 집합이다.
// 왜 미리 만들어 두나: 런타임 버전에 따라 /sched/pauses/* 같은 지표가 없을 수 있다.
// metrics.All() 로 먼저 확인하면 없는 이름을 읽으려다 겪는 문제를 원천 차단한다.
var knownMetrics = func() map[string]bool {
	m := make(map[string]bool, 128)
	for _, d := range metrics.All() {
		m[d.Name] = true
	}
	return m
}()

// ---------- STW 일시정지 집계 ----------

type pauseSummary struct {
	Count int
	Total time.Duration
	Max   time.Duration
	P50   time.Duration
	P99   time.Duration
}

// summarizePauses 는 입력을 건드리지 않고 정렬 사본으로 분위수를 낸다.
// 왜 사본인가: debug.GCStats.Pause 는 호출자가 넘긴 버퍼를 재사용하는데,
// 제자리 정렬을 하면 그 버퍼의 "최신이 앞" 이라는 의미가 뒤집혀 다음 호출이 망가진다.
func summarizePauses(ps []time.Duration) pauseSummary {
	var s pauseSummary
	s.Count = len(ps)
	if len(ps) == 0 {
		return s
	}
	cp := make([]time.Duration, len(ps))
	copy(cp, ps)
	sort.Slice(cp, func(i, j int) bool { return cp[i] < cp[j] })
	for _, d := range cp {
		s.Total += d
	}
	s.Max = cp[len(cp)-1]
	s.P50 = quantile(cp, 0.50)
	s.P99 = quantile(cp, 0.99)
	return s
}

// quantile 은 이미 오름차순인 슬라이스에서 nearest-rank 로 값을 고른다.
// 왜 보간하지 않나: STW 는 "실제로 존재했던 값"이 중요하고, 보간값은
// 스케줄러가 만든 실제 분포를 매끈하게 왜곡한다.
func quantile(sorted []time.Duration, q float64) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(q * float64(len(sorted)))
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

// readGCStats 는 최근 STW 일시정지 최대 256개를 받아온다.
// 256 은 런타임 내부 링 버퍼(mstats.pauseNs)의 크기다.
func readGCStats() debug.GCStats {
	st := debug.GCStats{
		Pause:          make([]time.Duration, 256),
		PauseQuantiles: make([]time.Duration, 5),
	}
	debug.ReadGCStats(&st)
	return st
}

// ---------- 쓰레기 생성 / 지연 측정 ----------

// holdLive 는 상주 집합을 n 바이트로 고정한다.
// 왜 페이지를 직접 만지나: make 만 하면 가상 주소만 잡히고 실제 페이지는 아직 안 붙어
// 첫 스캔에서 페이지 폴트가 섞여 측정이 흔들린다.
func holdLive(n int) {
	b := make([]byte, n)
	for i := 0; i < len(b); i += 4096 {
		b[i] = 1
	}
	liveSink = b
}

// churn 은 64번에 한 번만 참조를 살려 두고 나머지는 쓰레기로 버린다.
// 왜 전부 살리지 않나: 전부 살리면 GC 가 아니라 그냥 메모리 증가 실험이 된다.
func churn(rounds, size int) int64 {
	keep := size / 8
	if keep < 1 {
		keep = 1
	}
	var total int64
	for i := 0; i < rounds; i++ {
		b := make([]byte, size)
		b[0] = byte(i) // 관찰 가능한 쓰기: 할당이 실제로 일어나게 붙잡아 둔다
		total += int64(size)
		if i%64 == 0 {
			allocSink = append(allocSink[:0], b[:keep])
		}
	}
	return total
}

type latencyStats struct {
	Count int
	Min   time.Duration
	P50   time.Duration
	P99   time.Duration
	Max   time.Duration
}

// probe 는 "할당하는 goroutine 자신이 겪는 지연"을 잰다.
// 왜 남의 시계가 아니라 자기 호출 구간인가: mark assist 는 할당을 트리거한
// goroutine 의 몫에서 시간을 떼어 가므로, 그 비용은 정확히 이 구간에 나타난다.
func probe(iters, size int) latencyStats {
	if size < 1 {
		size = 1
	}
	ds := make([]time.Duration, 0, iters)
	for i := 0; i < iters; i++ {
		start := time.Now()
		b := make([]byte, size)
		b[0] = byte(i)
		allocSink = append(allocSink[:0], b) // 마지막 하나만 살린다
		ds = append(ds, time.Since(start))
	}
	st := latencyStats{Count: len(ds)}
	if st.Count == 0 {
		return st
	}
	sort.Slice(ds, func(i, j int) bool { return ds[i] < ds[j] })
	st.Min = ds[0]
	st.Max = ds[len(ds)-1]
	st.P50 = quantile(ds, 0.50)
	st.P99 = quantile(ds, 0.99)
	return st
}

// ---------- runtime/metrics 접근 ----------

func readFloat(name string) (float64, bool) {
	if !knownMetrics[name] {
		return 0, false
	}
	s := []metrics.Sample{{Name: name}}
	metrics.Read(s)
	if s[0].Value.Kind() != metrics.KindFloat64 {
		return 0, false
	}
	return s[0].Value.Float64(), true
}

// readHistogram 은 Value.Float64Histogram() 이 *Float64Histogram 을 돌려준다는 점에
// 주의해야 한다. 값 타입으로 받으려 하면 컴파일이 안 된다.
func readHistogram(name string) (*metrics.Float64Histogram, bool) {
	if !knownMetrics[name] {
		return nil, false
	}
	s := []metrics.Sample{{Name: name}}
	metrics.Read(s)
	if s[0].Value.Kind() != metrics.KindFloat64Histogram {
		return nil, false
	}
	h := s[0].Value.Float64Histogram()
	if h == nil || len(h.Counts) == 0 {
		return nil, false
	}
	return h, true
}

// edgeMicro 는 히스토그램 경계값(초)을 마이크로초 문자열로 바꾼다.
// 왜 분기가 필요한가: 첫 경계는 -Inf, 마지막 경계는 +Inf 라서 그냥 곱하면 NaN/Inf 가 찍힌다.
func edgeMicro(v float64) string {
	switch {
	case math.IsInf(v, -1):
		return "0"
	case math.IsInf(v, 1):
		return "inf"
	default:
		return fmt.Sprintf("%.2f", v*1e6)
	}
}

func printHistogram(h *metrics.Float64Histogram) {
	shown := 0
	for i, c := range h.Counts {
		if c == 0 {
			continue
		}
		fmt.Printf("  [%10s us, %10s us) : %d회\n",
			edgeMicro(h.Buckets[i]), edgeMicro(h.Buckets[i+1]), c)
		shown++
	}
	if shown == 0 {
		fmt.Println("  (기록 없음)")
	}
}

type cpuClass struct {
	assist    float64
	dedicated float64
	pause     float64
	total     float64
	ok        bool
}

func readGCCPU() cpuClass {
	var c cpuClass
	var okA, okD, okP, okT bool
	c.assist, okA = readFloat("/cpu/classes/gc/mark/assist:cpu-seconds")
	c.dedicated, okD = readFloat("/cpu/classes/gc/mark/dedicated:cpu-seconds")
	c.pause, okP = readFloat("/cpu/classes/gc/pause:cpu-seconds")
	c.total, okT = readFloat("/cpu/classes/gc/total:cpu-seconds")
	c.ok = okA && okD && okP && okT
	return c
}

func pct(a, b float64) float64 {
	if b <= 0 {
		return 0
	}
	return a / b * 100
}

// measureGCCPU 는 P 수를 바꿔 가며 같은 양의 쓰레기를 치우고, 그때 GC 가
// 어떤 종류의 CPU 시간을 얼마나 썼는지 돌려준다.
// 왜 P 수를 바꾸나: mark assist 는 "백그라운드 marker 가 못 따라갈 때" 발생하므로
// P 를 줄이면 assist 비중이 커지는 것이 관찰되어야 한다.
func measureGCCPU(nproc, rounds, size int) (cpuClass, time.Duration) {
	old := runtime.GOMAXPROCS(nproc)
	defer runtime.GOMAXPROCS(old)

	holdLive(4 << 20)
	allocSink = nil
	runtime.GC() // 이전 단계의 쓰레기가 이번 구간에 섞이지 않게 정리

	before := readGCCPU()
	start := time.Now()
	churn(rounds, size)
	d := time.Since(start)
	after := readGCCPU()

	return cpuClass{
		assist:    after.assist - before.assist,
		dedicated: after.dedicated - before.dedicated,
		pause:     after.pause - before.pause,
		total:     after.total - before.total,
		ok:        before.ok && after.ok,
	}, d
}

// ---------- 시연 1: STW 의 크기 ----------

func demonstrateSTW() {
	fmt.Println("\n=== 1. STW 는 어디에 있는가: 사이클당 두 번, 아주 짧게 ===")

	prev := debug.SetGCPercent(-1) // 자동 GC 를 꺼서 우리가 센 사이클만 보이게 한다
	before := readGCStats()
	const cycles = 5
	for i := 0; i < cycles; i++ {
		runtime.GC()
	}
	after := readGCStats()
	debug.SetGCPercent(prev)

	got := after.NumGC - before.NumGC
	fmt.Printf("runtime.GC() %d회 -> 실제 GC 사이클 %d개\n", cycles, got)
	fmt.Printf("누적 STW 시간 증가 %v\n", (after.PauseTotal - before.PauseTotal).Round(time.Microsecond))

	n := 2 * got
	if n > len(after.Pause) {
		n = len(after.Pause)
	}
	if n > 0 {
		fmt.Println("최근 일시정지 (최신이 앞):")
		for i := 0; i < n; i++ {
			fmt.Printf("  #%d %v\n", i, after.Pause[i].Round(time.Nanosecond))
		}
		sum := summarizePauses(after.Pause[:n])
		fmt.Printf("  요약: 합계 %v, 최대 %v, p50 %v, p99 %v\n",
			sum.Total.Round(time.Microsecond), sum.Max.Round(time.Microsecond),
			sum.P50.Round(time.Microsecond), sum.P99.Round(time.Microsecond))
	}

	if h, ok := readHistogram("/sched/pauses/stopping/gc:seconds"); ok {
		fmt.Println("runtime/metrics: /sched/pauses/stopping/gc:seconds (프로세스 시작 이후 누적)")
		printHistogram(h)
	}
	fmt.Println("=> 사이클당 정지 지점은 사이클 시작(sweep termination)과 종료(mark termination) 둘뿐이고,")
	fmt.Println("   그 사이의 마킹은 애플리케이션과 나란히 돈다. gctrace 의 a+c 가 그 두 조각이다.")
}

// ---------- 시연 2: GC CPU 의 구성 ----------

func demonstrateCPUBreakdown() {
	fmt.Println("\n=== 2. GC 의 CPU 를 쪼개 보면: STW 는 한 자릿수 이하, assist 는 징발 ===")

	for _, nproc := range []int{1, 4} {
		c, wall := measureGCCPU(nproc, 1500, 32*1024)
		if !c.ok {
			fmt.Printf("GOMAXPROCS=%d: /cpu/classes/gc/* 지표를 이 런타임이 제공하지 않는다\n", nproc)
			continue
		}
		fmt.Printf("GOMAXPROCS=%d  벽시계 %v  GC 총 CPU %.1f ms\n",
			nproc, wall.Round(time.Millisecond), c.total*1e3)
		fmt.Printf("  mark assist    %8.2f ms (%6.2f%%)  <- 애플리케이션 goroutine 이 징발된 시간\n",
			c.assist*1e3, pct(c.assist, c.total))
		fmt.Printf("  mark dedicated %8.2f ms (%6.2f%%)  <- 백그라운드 mark worker\n",
			c.dedicated*1e3, pct(c.dedicated, c.total))
		fmt.Printf("  STW pause      %8.3f ms (%6.3f%%)  <- 멈춘 구간\n",
			c.pause*1e3, pct(c.pause, c.total))
	}
	fmt.Println("=> P 를 1 로 줄이면 marker 가 쓸 여유가 없어 assist 몫이 커진다.")
	fmt.Println("   즉 STW 를 줄인 대신, 지연을 '할당하는 쪽'으로 분산시킨 것이 이 설계다.")
}

// ---------- 시연 3: 지연 스파이크의 출처 ----------

type phaseResult struct {
	name   string
	st     latencyStats
	stw    time.Duration
	assist time.Duration
}

func demonstrateLatencySource() {
	fmt.Println("\n=== 3. 지연 스파이크의 진짜 출처 ===")
	const iters, size = 2000, 32 * 1024

	phases := []struct {
		name      string
		gcPercent int
		nproc     int
	}{
		{"GC 꺼짐 (기준선)", -1, 4},
		{"GOGC=100 GOMAXPROCS=4", 100, 4},
		{"GOGC=100 GOMAXPROCS=1", 100, 1},
	}

	results := make([]phaseResult, 0, len(phases))
	fmt.Printf("%-24s %10s %10s %10s %12s %12s\n", "설정", "p50", "p99", "max", "STW 합계", "assist")

	for _, p := range phases {
		holdLive(4 << 20)
		oldPct := debug.SetGCPercent(p.gcPercent)
		oldProcs := runtime.GOMAXPROCS(p.nproc)
		allocSink = nil
		runtime.GC()

		bs := readGCStats()
		bc := readGCCPU()
		st := probe(iters, size)
		ac := readGCCPU()
		as := readGCStats()

		runtime.GOMAXPROCS(oldProcs)
		debug.SetGCPercent(oldPct)

		r := phaseResult{
			name:   p.name,
			st:     st,
			stw:    as.PauseTotal - bs.PauseTotal,
			assist: time.Duration((ac.assist - bc.assist) * 1e9),
		}
		results = append(results, r)
		fmt.Printf("%-24s %10v %10v %10v %12v %12v\n",
			r.name,
			r.st.P50.Round(time.Microsecond), r.st.P99.Round(time.Microsecond),
			r.st.Max.Round(time.Microsecond),
			r.stw.Round(time.Microsecond), r.assist.Round(time.Microsecond))
	}

	if len(results) < 2 {
		return
	}
	base := results[0]
	fmt.Println()
	for _, r := range results[1:] {
		// p99 가 p50 보다 커진 만큼을 "전체 체감 지연 증가"로 환산한다.
		tailP99 := (r.st.P99 - base.st.P99) * time.Duration(iters)
		tailP50 := (r.st.P50 - base.st.P50) * time.Duration(iters)
		fmt.Printf("%s: p50 증가분×%d회 = %v, p99 증가분×%d회 = %v, 그 구간 STW 합계 = %v\n",
			r.name, iters, tailP50.Round(time.Microsecond),
			iters, tailP99.Round(time.Microsecond), r.stw.Round(time.Microsecond))
	}
	fmt.Println("=> STW 합계는 수백 마이크로초인데 꼬리 지연은 수십 밀리초 규모다.")
	fmt.Println("   p99 스파이크의 출처는 멈춘 구간이 아니라, 할당 지점에서 징발된 mark assist 이다.")
}

func main() {
	fmt.Println("Go GC: STW 는 실제로 어디에 있고, 지연 스파이크는 어디서 오는가")
	fmt.Printf("Go %s, GOMAXPROCS=%d, NumCPU=%d\n", runtime.Version(), runtime.GOMAXPROCS(0), runtime.NumCPU())

	demonstrateSTW()
	demonstrateCPUBreakdown()
	demonstrateLatencySource()
}