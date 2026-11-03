// GOGC 와 GOMEMLIMIT 을 한 프로그램 안에서 바꿔 가며 관찰한다.
//
//	1) GOGC 는 "지금 살아 있는 힙의 몇 배까지 쓰레기를 쌓아 둘지"를 정하는 비율이다.
//	   비율이므로 절대 상한이 없다. 같은 live 힙에서 GOGC 만 바꿔 peak 힙이 어떻게
//	   달라지는지 본다.
//	2) GOMEMLIMIT 은 절대 상한이지만 soft 다. 살아 있는 힙이 한도를 넘으면 한도는
//	   지켜지지 않고, 대신 runtime 은 GC 가 CPU 를 독식하지 않도록 스스로 물러난다.
//
// 실행: go run .
// GC 사이클을 직접 보려면: GODEBUG=gctrace=1 go run .
package main

import (
	"fmt"
	"io"
	"math"
	"os"
	"runtime"
	"runtime/debug"
	"runtime/metrics"
	"text/tabwriter"
	"time"
)

const (
	mib = 1 << 20
	// debug.SetMemoryLimit 은 math.MaxInt64 를 "한도 없음"으로 해석한다.
	// 0 을 넣으면 "한도 0"이라는 뜻이 되어 버리므로 무제한은 이 값으로 표현한다.
	noLimit = int64(math.MaxInt64)
)

// sink 는 garbage 가 정말 garbage 가 되도록 마지막 덩어리만 붙잡아 두는 손잡이다.
// 컴파일러가 할당 자체를 지워 버리지 못하게 하는 역할도 겸한다.
var sink [][]byte

// allocLiveBlocks 은 "살아 있는 힙"을 만든다. 이 블록들은 측정이 끝날 때까지
// 회수되지 않으므로, GC 입장에서는 목표 힙을 계산하는 기준점이 된다.
func allocLiveBlocks(mb int) [][]byte {
	blocks := make([][]byte, 0, mb)
	for i := 0; i < mb; i++ {
		b := make([]byte, mib)
		// make 만 하면 runtime 이 이미 0 으로 채워진 페이지를 주므로 물리 메모리가
		// 붙지 않는다. 페이지마다 한 바이트씩 실제로 써야 RSS 가 늘어난다.
		for j := 0; j < len(b); j += 4096 {
			b[j] = byte(i + 1)
		}
		blocks = append(blocks, b)
	}
	return blocks
}

// churn 은 살아 있는 힙은 그대로 두고 할당량만 늘린다. 4MiB 덩어리를 만들어
// 직전 것만 붙잡고 나머지는 즉시 버리므로, heapLive 는 일정하고 쓰레기만 쌓인다.
func churn(totalMB int, sample func()) {
	const chunkMB = 4
	for i := 0; i < totalMB/chunkMB; i++ {
		b := make([]byte, chunkMB*mib)
		for j := 0; j < len(b); j += 4096 {
			b[j] = byte(i + 1)
		}
		sink = append(sink[:0], b)
		sample()
	}
	sink = nil
}

func metricUint64(name string) uint64 {
	s := []metrics.Sample{{Name: name}}
	metrics.Read(s)
	return s[0].Value.Uint64()
}

func metricFloat64(name string) float64 {
	s := []metrics.Sample{{Name: name}}
	metrics.Read(s)
	return s[0].Value.Float64()
}

// phaseResult 는 한 설정에서 측정한 결과다. peak 값은 "할당 도중 관측된 최대"이지
// 순간 최대가 아니다(4MiB 마다 표본을 뜬다).
type phaseResult struct {
	Name          string
	GCPercent     int
	MemLimit      int64
	LiveBytes     uint64
	PeakHeapAlloc uint64
	PeakHeapSys   uint64
	PeakGoal      uint64
	NumGC         uint32
	GCCPUShare    float64 // 전체 CPU 용량 대비 GC 가 쓴 비율(%)
	WallMillis    float64
}

// runPhase 는 GC 설정을 바꿔 가며 같은 workload 를 돌리고 지표를 모은다.
func runPhase(name string, gcPercent int, memLimit int64, liveMB, garbageMB int) phaseResult {
	prevPct := debug.SetGCPercent(gcPercent)
	prevLimit := debug.SetMemoryLimit(memLimit)
	defer func() {
		// 설정은 process 전역이다. 되돌리지 않으면 다음 단계(과 test)가 오염된다.
		debug.SetGCPercent(prevPct)
		debug.SetMemoryLimit(prevLimit)
		runtime.GC()
	}()

	// 이전 단계가 남긴 쓰레기를 먼저 정리한다. 이걸 빼먹으면 단계마다 baseline 이
	// 달라져 peak 수치를 서로 비교할 수 없다.
	runtime.GC()

	live := allocLiveBlocks(liveMB)

	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	res := phaseResult{
		Name:      name,
		GCPercent: gcPercent,
		MemLimit:  memLimit,
		LiveBytes: uint64(liveMB) * mib,
	}
	ngc0 := ms.NumGC
	cpu0 := metricFloat64("/cpu/classes/gc/total:cpu-seconds")
	wall0 := time.Now()

	churn(garbageMB, func() {
		runtime.ReadMemStats(&ms)
		if ms.HeapAlloc > res.PeakHeapAlloc {
			res.PeakHeapAlloc = ms.HeapAlloc
			res.PeakGoal = metricUint64("/gc/heap/goal:bytes")
		}
		if ms.HeapSys > res.PeakHeapSys {
			res.PeakHeapSys = ms.HeapSys
		}
	})
	// 여기까지 live 블록이 살아 있어야 "살아 있는 힙이 일정하다"는 전제가 유지된다.
	runtime.KeepAlive(live)

	wall := time.Since(wall0).Seconds()
	share := metricFloat64("/cpu/classes/gc/total:cpu-seconds") - cpu0
	runtime.ReadMemStats(&ms)
	res.NumGC = ms.NumGC - ngc0
	res.WallMillis = wall * 1000
	if wall > 0 {
		// GC 는 여러 P 에서 동시에 돈다. 벽시계로만 나누면 100% 를 훌쩍 넘으므로
		// 전체 CPU 용량(GOMAXPROCS)으로 정규화해야 "용량 대비 비율"이 된다.
		res.GCCPUShare = 100 * share / (wall * float64(runtime.GOMAXPROCS(0)))
	}
	return res
}

func runAllPhases(liveMB, garbageMB int) []phaseResult {
	return []phaseResult{
		runPhase("기본값 GOGC=100", 100, noLimit, liveMB, garbageMB),
		runPhase("GOGC=400 (게으른 GC)", 400, noLimit, liveMB, garbageMB),
		runPhase("GOGC=off + GOMEMLIMIT=48MiB", -1, 48*mib, liveMB, garbageMB),
		// 한도를 live 힙보다 낮춘 경우. 쓰레기 양을 줄여 폭주를 관찰 가능한
		// 범위 안에 둔다.
		runPhase("GOMEMLIMIT=8MiB (< live 32MiB)", 100, 8*mib, liveMB, 64),
	}
}

// gcPercentHeapGoal 은 GOGC 비례 제어가 정하는 힙 목표를 계산한다.
// runtime 의 gcControllerState.gcPercentHeapGoal 과 같은 형태의 식이다.
// GOGC 가 음수면 GC 를 끈 것이므로 상한이 없다.
func gcPercentHeapGoal(live uint64, gcPercent int) uint64 {
	if gcPercent < 0 {
		return math.MaxUint64
	}
	goal := live + live*uint64(gcPercent)/100
	// runtime 은 아주 작은 힙에서 GC 가 초당 수백 번 도는 것을 막으려고
	// 최소 목표(4MiB)를 둔다.
	const minHeapGoal = 4 * mib
	if goal < minHeapGoal {
		return minHeapGoal
	}
	return goal
}

// memoryLimitHeapGoal 은 GOMEMLIMIT 이 정하는 힙 목표를 근사한다.
// 실제 runtime 은 한도에서 "힙이 아닌 몫"(goroutine 스택, 전역, runtime 및 GC
// 메타데이터, 단편화 여유분)을 먼저 떼어 낸 뒤 남은 만큼을 목표로 삼는다.
// 여기서는 그 몫을 한도의 10% 로 근사한다.
func memoryLimitHeapGoal(memLimit int64, live uint64) uint64 {
	if memLimit <= 0 || memLimit == noLimit {
		return math.MaxUint64
	}
	nonHeap := uint64(memLimit) / 10
	if uint64(memLimit) <= nonHeap {
		return live
	}
	goal := uint64(memLimit) - nonHeap
	if goal < live {
		// 살아 있는 힙보다 목표가 작으면 GC 로는 만족할 수 없다. 이때 목표를
		// 억지로 낮추면 GC 가 무한히 돌 뿐이므로 runtime 은 물러선다(soft limit).
		return live
	}
	return goal
}

// effectiveHeapGoal 은 두 목표 중 작은 쪽을 고른다. 실제 runtime 도 같은 규칙을 쓴다.
func effectiveHeapGoal(live uint64, gcPercent int, memLimit int64) uint64 {
	gcGoal := gcPercentHeapGoal(live, gcPercent)
	limGoal := memoryLimitHeapGoal(memLimit, live)
	if limGoal < gcGoal {
		return limGoal
	}
	return gcGoal
}

// recommendMemoryLimit 은 컨테이너 한도에서 Go runtime 에 줄 몫을 계산한다.
// GOMEMLIMIT 은 Go 가 추적하는 메모리만 세므로 cgo·mmap·sidecar 가 쓰는 몫을
// 먼저 떼어 내야 OOM 을 피할 수 있다.
func recommendMemoryLimit(containerBytes, reserveBytes int64) int64 {
	if containerBytes <= 0 {
		return noLimit
	}
	if reserveBytes < 0 {
		reserveBytes = 0
	}
	if reserveBytes >= containerBytes {
		// 여유분이 한도보다 크다. 이건 설정 자체가 잘못된 것이므로 "무제한"처럼
		// 보이는 값을 돌려주지 않고 최소한의 양보만 한다.
		return mib
	}
	return containerBytes - reserveBytes
}

func humanBytes(v uint64) string {
	switch {
	case v >= 1024*mib:
		return fmt.Sprintf("%.1f GiB", float64(v)/(1024*mib))
	case v >= mib:
		return fmt.Sprintf("%.1f MiB", float64(v)/mib)
	}
	return fmt.Sprintf("%d KiB", v/1024)
}

func printResults(w io.Writer, results []phaseResult) {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "단계\tGOGC\tGOMEMLIMIT\tpeak HeapAlloc\tpeak HeapSys\tpeak 목표힙\tGC 횟수\tGC CPU%")
	for _, r := range results {
		limit := "없음"
		if r.MemLimit != noLimit {
			limit = humanBytes(uint64(r.MemLimit))
		}
		gogc := fmt.Sprint(r.GCPercent)
		if r.GCPercent < 0 {
			gogc = "off"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%d\t%.1f%%\n",
			r.Name, gogc, limit,
			humanBytes(r.PeakHeapAlloc), humanBytes(r.PeakHeapSys),
			humanBytes(r.PeakGoal), r.NumGC, r.GCCPUShare)
	}
	tw.Flush()
}

func summarize(w io.Writer, results []phaseResult) {
	if len(results) < 4 {
		return
	}
	base, lazy, capped, soft := results[0], results[1], results[2], results[3]
	fmt.Fprintln(w, "\n해석:")
	fmt.Fprintf(w, "  * GOGC=100: live 힙 %s 의 2배인 %s 근처에서 GC 가 시작된다. 관측 peak %s\n",
		humanBytes(base.LiveBytes), humanBytes(2*base.LiveBytes), humanBytes(base.PeakHeapAlloc))
	fmt.Fprintf(w, "  * GOGC=400: live 힙은 같은데 목표가 5배로 커져 peak 가 %s (기본값의 %.1f배)\n",
		humanBytes(lazy.PeakHeapAlloc), float64(lazy.PeakHeapAlloc)/float64(base.PeakHeapAlloc))
	fmt.Fprintf(w, "  * GOGC=off 라도 GOMEMLIMIT=%s 이면 GC 가 돌아 peak 가 %s 로 눌린다 (GC %d회)\n",
		humanBytes(uint64(capped.MemLimit)), humanBytes(capped.PeakHeapAlloc), capped.NumGC)
	fmt.Fprintf(w, "  * 한도를 live 힙보다 낮추면(8MiB < 32MiB) 한도는 지켜지지 않는다: peak %s, GC %d회 (soft limit)\n",
		humanBytes(soft.PeakHeapAlloc), soft.NumGC)
}

func printFormulaTable(w io.Writer) {
	fmt.Fprintln(w, "\n[비례 제어 vs 절대 한도: 살아 있는 힙별 목표 힙]")
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "live 힙\t설정\t목표 힙")
	for _, liveMB := range []uint64{64, 512, 2048} {
		live := liveMB * mib
		fmt.Fprintf(tw, "%s\tGOGC=100\t%s\n", humanBytes(live), humanBytes(gcPercentHeapGoal(live, 100)))
		fmt.Fprintf(tw, "%s\tGOGC=400\t%s\n", humanBytes(live), humanBytes(gcPercentHeapGoal(live, 400)))
		fmt.Fprintf(tw, "%s\tGOMEMLIMIT=1GiB\t%s\n", humanBytes(live), humanBytes(effectiveHeapGoal(live, 100, 1<<30)))
	}
	tw.Flush()
	fmt.Fprintln(w, "  (마지막 줄이 2.0 GiB 로 나오면 한도가 살아 있는 힙보다 작아 아무 힘도 못 쓴 상태다)")
}

func printContainerRecipe(w io.Writer) {
	const containerBytes = 2 << 30 // 2GiB
	const reserve = 256 * mib
	fmt.Fprintf(w, "\n[컨테이너 2GiB 한도에서의 설정 절차]\n")
	fmt.Fprintf(w, "  1) steady state 의 live 힙을 잰다: runtime/metrics 의 /gc/heap/live:bytes, 또는 go tool pprof -inuse_space\n")
	fmt.Fprintf(w, "  2) live 힙이 한도의 1/3 을 넘으면 GOMEMLIMIT 을 건다: %s (= 2GiB - 여유 256MiB)\n",
		humanBytes(uint64(recommendMemoryLimit(containerBytes, reserve))))
	fmt.Fprintf(w, "  3) GOGC 는 100 으로 두거나, CPU 가 남으면 200~400 으로 올린다. 상한은 이제 GOMEMLIMIT 이 맡는다\n")
	fmt.Fprintf(w, "  4) 배포 후 GODEBUG=gctrace=1 로 'goal' 이 한도 근처에서 안정적인지, GC 사이클이 촘촘해지지 않는지 확인한다\n")
}

func main() {
	fmt.Printf("Go %s / GOMAXPROCS=%d\n", runtime.Version(), runtime.GOMAXPROCS(0))
	fmt.Println("GODEBUG=gctrace=1 로 실행하면 사이클마다 '시작 힙 -> 종료 힙 -> live 힙, goal' 이 찍힌다.")
	fmt.Println()

	results := runAllPhases(32, 256)
	printResults(os.Stdout, results)
	summarize(os.Stdout, results)
	printFormulaTable(os.Stdout)
	printContainerRecipe(os.Stdout)
}