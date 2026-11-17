package main

// execution tracer 로 지연 원인 분해하기.
//
// 벽시계 시간(wall)은 숫자 하나지만, 그 안에는 성격이 다른 시간이 섞여 있다.
//   - CPU 를 실제로 쓴 시간
//   - P 를 못 잡아 runnable 큐에서 기다린 시간 (스케줄 지연)
//   - mutex/채널/syscall 에서 블록된 시간
//   - GC 가 가져간 시간
// CPU 프로파일은 첫 번째만 보여준다. 그래서 나머지가 커지면 "CPU 는 한가한데
// 서비스는 느리다"는 상황이 되고, 프로파일에는 아무것도 안 뜬다.
// 여기서는 runtime/metrics 로 조각들을 수치로 분해하고, runtime/trace 로
// 실제 트레이스를 남겨 go tool trace 의 goroutine 타임라인에서 확인할 수 있게 한다.

import (
	"context"
	"fmt"
	"math"
	"os"
	"runtime"
	"runtime/metrics"
	"runtime/trace"
	"sync"
	"text/tabwriter"
	"time"
)

// 작업량. go run . 이 몇 초를 넘지 않도록 작게 잡았다.
// 대신 한 goroutine 이 P 를 붙잡는 시간을 충분히 길게 해서
// 뒤에 줄 선 goroutine 의 runnable 대기가 눈에 보이게 했다.
const (
	cpuWorkers   = 32
	cpuIters     = 1_500_000
	lockWorkers  = 32
	lockRounds   = 800
	allocWorkers = 24
	allocIters   = 80
	allocSize    = 32 << 10
)

// 계산 결과를 밖으로 흘려야 컴파일러가 루프를 통째로 지우지 않는다.
var sinkU64 []uint64
var sinkI64 int64

// cpuBurst 는 순수 CPU 작업 덩어리다. P 를 오래 붙잡을수록 뒤에 줄 선
// goroutine 의 runnable 대기가 길어지므로, 크기를 조절해 스케줄 지연을
// 인위적으로 만들 수 있다.
func cpuBurst(iters int) uint64 {
	x := uint64(0x9e3779b97f4a7c15)
	for i := 0; i < iters; i++ {
		x ^= x << 13
		x ^= x >> 7
		x ^= x << 17
	}
	return x
}

// runFanout 은 n 개 goroutine 을 동시에 띄우고 결과를 모은다.
// 결과를 인덱스별로 따로 쓰는 이유: 하나의 슬라이스에 append 하면 그 lock 이
// 측정하려는 현상(스케줄 지연)을 덮어버린다. 측정 도구가 측정 대상을
// 오염시키지 않게 하는 것이 먼저다.
func runFanout(n int, work func(i int) uint64) []uint64 {
	out := make([]uint64, n)
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			out[i] = work(i)
		}(i)
	}
	wg.Wait()
	return out
}

// phaseCPU 는 P 가 충분한 상태의 기준선이다. 이것과 아래 단계들을 비교한다.
func phaseCPU(workers, iters int) func() {
	return func() {
		sinkU64 = runFanout(workers, func(i int) uint64 { return cpuBurst(iters) })
	}
}

// phaseCPUOnProcs 는 "같은 일"을 P 개수만 줄여서 돌린다.
// 일의 양은 그대로이므로 user CPU 는 거의 같아야 하고, 늘어난 wall 은
// 전부 대기여야 한다. 그 대기의 정체를 sched 히스토그램이 알려준다.
func phaseCPUOnProcs(procs, workers, iters int) func() {
	return func() {
		old := runtime.GOMAXPROCS(procs)
		defer runtime.GOMAXPROCS(old)
		sinkU64 = runFanout(workers, func(i int) uint64 { return cpuBurst(iters) })
	}
}

// phaseLock 은 하나의 mutex 를 여러 goroutine 이 두드리는 상황이다.
// 임계 구역은 아주 짧지만 한 번에 하나만 들어가므로 처리량은 코어 수와
// 무관하게 1 에 가까워진다. 중요한 건 여기서 기다리는 goroutine 이
// runnable 이 아니라 blocked(semacquire) 상태라는 점이다. 그래서
// 스케줄 지연 지표는 조용한데 wall 만 커진다.
func phaseLock(workers, rounds int) func() {
	return func() {
		var mu sync.Mutex
		var shared uint64
		var wg sync.WaitGroup
		wg.Add(workers)
		for w := 0; w < workers; w++ {
			go func(w int) {
				defer wg.Done()
				for r := 0; r < rounds; r++ {
					mu.Lock()
					shared = shared*6364136223846793005 + uint64(w) + 1
					mu.Unlock()
				}
			}(w)
		}
		wg.Wait()
		sinkU64 = append(sinkU64[:0], shared)
	}
}

// phaseAlloc 은 할당 폭풍이다. 4개 중 1개만 붙잡아 live heap 을 만들었는데,
// live 가 있어야 GC 의 mark 단계가 실제로 할 일을 갖기 때문이다.
// live 가 0 이면 GC 는 시작하자마자 끝나서 GC CPU 가 거의 안 잡힌다.
func phaseAlloc(workers, iters, size int) func() {
	return func() {
		var wg sync.WaitGroup
		var mu sync.Mutex
		var kept [][]byte
		wg.Add(workers)
		for w := 0; w < workers; w++ {
			go func(w int) {
				defer wg.Done()
				local := make([][]byte, 0, iters/4+1)
				for i := 0; i < iters; i++ {
					b := make([]byte, size)
					b[0] = byte(i)
					b[size-1] = byte(w)
					if i%4 == 0 {
						local = append(local, b)
					}
				}
				mu.Lock()
				kept = append(kept, local...)
				mu.Unlock()
			}(w)
		}
		wg.Wait()
		sinkI64 = int64(len(kept))
		runtime.KeepAlive(kept)
	}
}

// schedLatencyHist 는 "runnable 이었지만 P 를 배정받지 못한 시간"의 누적
// 히스토그램을 복사해서 돌려준다. metrics.Read 가 돌려주는 슬라이스는
// 런타임 소유라 다음 Read 에서 덮어써진다. 복사하지 않으면 before/after 를
// 비교할 때 두 값이 같아져서 delta 가 0 이 되는 조용한 버그가 난다.
func schedLatencyHist() *metrics.Float64Histogram {
	var s [1]metrics.Sample
	s[0].Name = "/sched/latencies:seconds"
	metrics.Read(s[:])
	if s[0].Value.Kind() != metrics.KindFloat64Histogram {
		return nil
	}
	h := s[0].Value.Float64Histogram()
	counts := make([]uint64, len(h.Counts))
	copy(counts, h.Counts)
	buckets := make([]float64, len(h.Buckets))
	copy(buckets, h.Buckets)
	return &metrics.Float64Histogram{Counts: counts, Buckets: buckets}
}

type cpuSnapshot struct {
	user float64 // /cpu/classes/user:cpu-seconds
	gc   float64 // /cpu/classes/gc/total:cpu-seconds
}

func readCPU() cpuSnapshot {
	samples := []metrics.Sample{
		{Name: "/cpu/classes/user:cpu-seconds"},
		{Name: "/cpu/classes/gc/total:cpu-seconds"},
	}
	metrics.Read(samples)
	var s cpuSnapshot
	if samples[0].Value.Kind() == metrics.KindFloat64 {
		s.user = samples[0].Value.Float64()
	}
	if samples[1].Value.Kind() == metrics.KindFloat64 {
		s.gc = samples[1].Value.Float64()
	}
	return s
}

// histDelta 는 누적 히스토그램 두 개의 차이를 낸다. metrics 의 히스토그램은
// 구간별 "현재까지의 개수"이므로 빼기만 하면 그 사이 구간의 분포가 된다.
func histDelta(before, after *metrics.Float64Histogram) ([]uint64, []float64) {
	if before == nil || after == nil || len(before.Counts) != len(after.Counts) {
		return nil, nil
	}
	counts := make([]uint64, len(after.Counts))
	for i := range counts {
		if after.Counts[i] > before.Counts[i] {
			counts[i] = after.Counts[i] - before.Counts[i]
		}
	}
	buckets := make([]float64, len(after.Buckets))
	copy(buckets, after.Buckets)
	return counts, buckets
}

func sumCounts(counts []uint64) uint64 {
	var s uint64
	for _, c := range counts {
		s += c
	}
	return s
}

// quantileFromHist 는 버킷 상한을 돌려준다. 보수적 추정이라는 점이 중요하다.
// 실제 값은 버킷 안 어딘가에 있으므로 p99 는 "적어도 이만큼"으로 읽어야 한다.
func quantileFromHist(counts []uint64, buckets []float64, q float64) float64 {
	if len(counts) == 0 || len(buckets) != len(counts)+1 {
		return 0
	}
	total := sumCounts(counts)
	if total == 0 {
		return 0
	}
	if q < 0 {
		q = 0
	}
	if q > 1 {
		q = 1
	}
	target := uint64(float64(total) * q)
	if target >= total {
		target = total - 1
	}
	var acc uint64
	for i, c := range counts {
		acc += c
		if acc > target {
			if ub := buckets[i+1]; !math.IsInf(ub, 1) {
				return ub
			}
			return buckets[i]
		}
	}
	return buckets[len(buckets)-1]
}

// maxBucket 은 표본이 있는 마지막 구간의 상한이다. 꼬리 지연을 볼 때 쓴다.
func maxBucket(counts []uint64, buckets []float64) float64 {
	for i := len(counts) - 1; i >= 0; i-- {
		if counts[i] > 0 {
			if ub := buckets[i+1]; !math.IsInf(ub, 1) {
				return ub
			}
			return buckets[i]
		}
	}
	return 0
}

type phaseReport struct {
	name     string
	wall     time.Duration
	userCPU  float64
	gcCPU    float64
	schedP50 float64
	schedP99 float64
	schedMax float64
	samples  uint64
}

// parallelism 은 측정 구간 동안 평균적으로 몇 개의 P 가 실제로 바빴는지다.
// 값이 GOMAXPROCS 보다 한참 작으면 "코어는 남는데 goroutine 이 못 돈다"는 뜻이고,
// 1 근처면 "직렬화됐다"는 뜻이다. 둘은 처방이 정반대다.
func (r phaseReport) parallelism() float64 {
	if r.wall <= 0 {
		return 0
	}
	return (r.userCPU + r.gcCPU) / r.wall.Seconds()
}

func nonNeg(v float64) float64 {
	if v < 0 {
		return 0
	}
	return v
}

func msOf(d time.Duration) float64 { return float64(d) / float64(time.Millisecond) }

// measure 는 한 단계를 감싸서 wall, CPU, 스케줄 대기를 한꺼번에 뽑는다.
// trace.WithRegion 은 트레이스에 이 단계의 이름을 표시로 남긴다. 나중에
// go tool trace 에서 구간을 눈으로 찾을 때 이 표시가 없으면 고생한다.
func measure(ctx context.Context, name string, fn func()) phaseReport {
	lat0 := schedLatencyHist()
	cpu0 := readCPU()
	t0 := time.Now()
	trace.WithRegion(ctx, name, fn)
	wall := time.Since(t0)
	cpu1 := readCPU()
	lat1 := schedLatencyHist()

	rep := phaseReport{
		name:    name,
		wall:    wall,
		userCPU: nonNeg(cpu1.user - cpu0.user),
		gcCPU:   nonNeg(cpu1.gc - cpu0.gc),
	}
	counts, buckets := histDelta(lat0, lat1)
	rep.samples = sumCounts(counts)
	rep.schedP50 = quantileFromHist(counts, buckets, 0.50)
	rep.schedP99 = quantileFromHist(counts, buckets, 0.99)
	rep.schedMax = maxBucket(counts, buckets)
	return rep
}

func printReports(rows []phaseReport) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', tabwriter.AlignRight)
	fmt.Fprintln(w, "단계\twall ms\tuserCPU ms\tGCCPU ms\t점유P\t대기p50 ms\t대기p99 ms\t대기max ms\t샘플")
	for _, r := range rows {
		fmt.Fprintf(w, "%s\t%.1f\t%.1f\t%.1f\t%.2f\t%.3f\t%.3f\t%.3f\t%d\n",
			r.name, msOf(r.wall), r.userCPU*1000, r.gcCPU*1000,
			r.parallelism(), r.schedP50*1000, r.schedP99*1000, r.schedMax*1000, r.samples)
	}
	w.Flush()
}

func printInterpretation(rows []phaseReport, limited int) {
	if len(rows) < 4 {
		return
	}
	fmt.Println()
	fmt.Println("읽는 법")
	fmt.Printf("  1 vs 2: 같은 CPU 작업을 P=%d 로 줄여서 돌린 것이다.\n", limited)
	fmt.Println("    userCPU 가 비슷한데 wall 이 늘었다면, 늘어난 시간은 일이 아니라 대기다.")
	fmt.Println("    그 대기의 정체가 대기p99 열이다. goroutine 이 runnable 큐에서 P 를 기다린 시간.")
	fmt.Println("  3: wall 은 더 큰데 대기p99 는 작다. 원인이 runnable 이 아니라 blocked,")
	fmt.Println("    즉 sync.Mutex 의 semacquire 라는 뜻이다. 처방이 정반대다.")
	fmt.Println("  4: GCCPU 열이 wall 중 GC 가 가져간 몫이다. 이 값이 크면 코드 튜닝보다")
	fmt.Println("    할당 줄이기가 먼저다. CPU 프로파일 상위에 runtime.gcBgMarkWorker 가")
	fmt.Println("    뜨는 이유이기도 하다.")
}

// startTrace 는 트레이스를 켜고 정지 함수를 돌려준다. 정지는 한 번만
// 호출되도록 once 로 감쌌다. trace.Stop 을 두 번 부르면 panic 이 난다.
func startTrace(path string) (func(), error) {
	f, err := os.Create(path)
	if err != nil {
		return nil, err
	}
	if err := trace.Start(f); err != nil {
		f.Close()
		return nil, err
	}
	var once sync.Once
	return func() {
		once.Do(func() {
			trace.Stop()
			f.Close()
		})
	}, nil
}

func buildPhases() []struct {
	name string
	fn   func()
} {
	procs := runtime.GOMAXPROCS(0)
	limited := 2
	if procs < limited {
		limited = procs
	}
	return []struct {
		name string
		fn   func()
	}{
		{"1 cpu-bound P충분", phaseCPU(cpuWorkers, cpuIters)},
		{fmt.Sprintf("2 같은일 P=%d", limited), phaseCPUOnProcs(limited, cpuWorkers, cpuIters)},
		{"3 mutex 경합", phaseLock(lockWorkers, lockRounds)},
		{"4 할당/GC", phaseAlloc(allocWorkers, allocIters, allocSize)},
	}
}

func runDemo(ctx context.Context) []phaseReport {
	fmt.Printf("Go %s  GOMAXPROCS=%d  NumCPU=%d  GOGC=%s\n\n",
		runtime.Version(), runtime.GOMAXPROCS(0), runtime.NumCPU(), os.Getenv("GOGC"))
	phases := buildPhases()
	reports := make([]phaseReport, 0, len(phases))
	for _, p := range phases {
		reports = append(reports, measure(ctx, p.name, p.fn))
	}
	printReports(reports)
	printInterpretation(reports, runtime.GOMAXPROCS(0))
	return reports
}

func main() {
	stop, err := startTrace("trace.out")
	if err != nil {
		fmt.Fprintln(os.Stderr, "trace 시작 실패:", err)
	}
	runDemo(context.Background())
	if stop != nil {
		stop()
		fmt.Println()
		fmt.Println("트레이스 저장: trace.out")
		fmt.Println("  go tool trace trace.out            브라우저 UI 에서 Goroutine analysis 탭")
		fmt.Println("  GODEBUG=schedtrace=1000 go run .   스케줄러 상태를 1초마다 텍스트로")
	}
}