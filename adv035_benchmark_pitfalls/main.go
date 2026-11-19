package main

import (
	"flag"
	"fmt"
	"math"
	"runtime"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// sink: 측정한 값을 붙잡아 두는 전역 변수
// ---------------------------------------------------------------------------
//
// 왜 지역 변수로는 안 되는가. 지역 변수에 결과를 담고 아무도 그 값을 읽지 않으면
// SSA deadcode 단계에서 저장 자체가 사라지고, 저장이 사라지면 그 값을 만들어낸
// 계산도 함께 사라진다. 패키지 수준 변수에 대한 쓰기는 다른 goroutine 이 관측할
// 수 있는 효과라서 지울 수 없다. Go 는 volatile 도 asm 훼이크도 없는 대신
// 이 관용구 하나로 끝낸다.
var (
	sinkInt  int
	sinkStr  string
	sinkByte []byte
)

const dataSize = 1 << 10

// buildData 는 "준비 비용"을 만들어내는 함수다.
// noinline 을 붙인 이유: 인라인되면 컴파일러가 본문을 들여다보고 결과가
// 쓰이지 않는 호출을 통째로 지울 수 있다. opaque 한 호출로 남겨야
// 준비 비용이 실제로 발생한다.
//
//go:noinline
func buildData(n int) []byte {
	buf := make([]byte, n)
	for i := range buf {
		buf[i] = byte(i*31 + 7)
	}
	return buf
}

// dataBuf 는 루프 강도 감소를 막기 위한 전역 버퍼다. 상수 반복 횟수를 가진
// 단순 합 루프는 컴파일러가 닫힌 형태로 접어버릴 수 있다. 전역 슬라이스에서
// 읽게 하면 그럴 수 없다.
var dataBuf = buildData(1 << 12)

// work 는 "측정하고 싶은 진짜 일"이다. k 에 거의 선형으로 비례한다.
func work(k int) {
	s := 0
	for i := 0; i < k; i++ {
		s += int(dataBuf[i&4095])
	}
	sinkInt += s
}

// ---------------------------------------------------------------------------
// 미니 벤치마크 하네스: testing.B 가 b.N 을 정하는 방식을 그대로 흉내 낸다
// ---------------------------------------------------------------------------

// runN 은 f 를 정확히 n 번 호출한다. testing.B 의 runN 과 같은 계약이며,
// 시간이 아니라 호출 횟수를 테스트할 수 있도록 분리해 두었다.
func runN(n int, f func()) {
	for i := 0; i < n; i++ {
		f()
	}
}

// benchN 은 1회 실행부터 시작해 목표 시간(target)에 도달할 때까지 N 을 키우며
// 다시 잰다. testing.B 의 run1 과 같은 전략이다.
//
// 왜 고정 N 이 아니라 적응형인가. 벤치마크 대상의 비용은 수 ns 에서 수 ms 까지
// 여섯 자릿수나 벌어진다. 고정 N 으로는 어떤 대상은 타이머 해상도에 묻히고
// 어떤 대상은 테스트를 몇 분씩 잡아먹는다. 그래서 "목표 시간"에서 N 을 역산한다.
// 즉 N 은 측정 대상이 아니라 측정 도구다.
func benchN(f func(), target time.Duration) (int, float64, []int) {
	if target <= 0 {
		target = time.Millisecond
	}
	var trace []int
	n := 1
	for round := 0; round < 40; round++ {
		trace = append(trace, n)
		start := time.Now()
		runN(n, f)
		d := time.Since(start)
		if d < time.Nanosecond {
			d = time.Nanosecond
		}
		if d >= target || n >= 1<<26 {
			return n, float64(d.Nanoseconds()) / float64(n), trace
		}
		// 다음 N = 현재 N * (목표 시간 / 실제 걸린 시간) * 여유 계수.
		// 한 번에 너무 크게 뛰면 느린 함수에서 폭발하므로 1000배로 제한한다.
		next := int(float64(n) * float64(target) / float64(d) * 1.2)
		if next <= n {
			next = n + 1
		}
		if next > n*1000 {
			next = n * 1000
		}
		if next > 1<<26 {
			next = 1 << 26
		}
		n = next
	}
	return n, 0, trace
}

// collect 는 같은 함수를 여러 번 재서 ns/op 표본을 모은다.
// 단일 측정값으로 두 구현을 비교하는 것이 benchstat 이 존재하는 이유다.
func collect(runs int, target time.Duration, f func()) []float64 {
	out := make([]float64, 0, runs)
	for i := 0; i < runs; i++ {
		_, ns, _ := benchN(f, target)
		out = append(out, ns)
	}
	return out
}

// stats 는 benchstat 류 도구의 입력이 되는 요약 통계다.
type stats struct {
	N      int
	Mean   float64
	Stddev float64
	Min    float64
	Max    float64
}

func (s stats) String() string {
	rel := 0.0
	if s.Mean != 0 {
		rel = s.Stddev / s.Mean * 100
	}
	return fmt.Sprintf("N=%d mean=%.1f sd=%.1f (%.1f%%) min=%.1f max=%.1f",
		s.N, s.Mean, s.Stddev, rel, s.Min, s.Max)
}

// summarize 는 표본표준편차(n-1 로 나눔)를 쓴다. 모표준편차를 쓰면 표본이
// 작을 때 변동성을 과소평가해서, 잡음을 개선으로 착각하게 만든다.
func summarize(xs []float64) stats {
	if len(xs) == 0 {
		return stats{}
	}
	n := len(xs)
	lo, hi := xs[0], xs[0]
	sum := 0.0
	for _, x := range xs {
		sum += x
		if x < lo {
			lo = x
		}
		if x > hi {
			hi = x
		}
	}
	mean := sum / float64(n)
	sd := 0.0
	if n > 1 {
		ss := 0.0
		for _, x := range xs {
			d := x - mean
			ss += d * d
		}
		sd = math.Sqrt(ss / float64(n-1))
	}
	return stats{N: n, Mean: mean, Stddev: sd, Min: lo, Max: hi}
}

// significant 는 두 표본이 "다르다"고 말해도 되는지 판정한다.
// 두 조건을 모두 요구한다.
//  1) 평균 차이가 합동 표준편차의 2배를 넘을 것 (잡음 대비 충분한 신호)
//  2) 평균 차이가 평균값의 1% 를 넘을 것 (실무적으로 의미 있는 크기)
// benchstat 은 여기에 웰치 t-검정과 기하평균을 더 얹지만, 골격은 같다.
func significant(a, b stats) bool {
	if a.N < 2 || b.N < 2 {
		return false
	}
	if a.Mean <= 0 || b.Mean <= 0 {
		return false
	}
	pooled := math.Sqrt((a.Stddev*a.Stddev + b.Stddev*b.Stddev) / 2)
	diff := math.Abs(a.Mean - b.Mean)
	if diff < 2*pooled {
		return false
	}
	mean := (a.Mean + b.Mean) / 2
	if diff/mean < 0.01 {
		return false
	}
	return true
}

// ---------------------------------------------------------------------------
// [1] 결과를 버리면 측정 자체가 사라진다
// ---------------------------------------------------------------------------

// allocsDiscard 는 변환 결과를 blank identifier 로 버린다.
// 컴파일러 입장에서 관측 가능한 결과가 없으므로 통째로 사라질 수 있다.
func allocsDiscard(b []byte) float64 {
	return testing.AllocsPerRun(200, func() { _ = string(b) })
}

// allocsSink 는 결과를 전역 sink 에 밀어 넣는다. 이 쓰기는 지울 수 없다.
func allocsSink(b []byte) float64 {
	return testing.AllocsPerRun(200, func() { sinkStr = string(b) })
}

func demoDeadCode() {
	fmt.Println("[1] 결과를 버리면 측정 대상 자체가 사라진다 (1024 B []byte -> string)")
	buf := buildData(dataSize)

	discardAllocs := allocsDiscard(buf)
	sinkAllocs := allocsSink(buf)
	_, discardNs, _ := benchN(func() { _ = string(buf) }, 20*time.Millisecond)
	_, sinkNs, _ := benchN(func() { sinkStr = string(buf) }, 20*time.Millisecond)

	fmt.Printf("  결과를 버림    allocs/op=%4.1f  ns/op=%9.2f\n", discardAllocs, discardNs)
	fmt.Printf("  sink 에 저장   allocs/op=%4.1f  ns/op=%9.2f\n", sinkAllocs, sinkNs)
	fmt.Println("  해석: allocs/op 가 0 이면 그 벤치마크는 '아무것도 하지 않는 코드'를 잰 것이다.")
	fmt.Println("        ns/op 가 작은 것은 빠른 구현이 아니라 컴파일러가 일을 지운 흔적이다.")
}

// ---------------------------------------------------------------------------
// [2] 준비 비용이 측정 구간에 섞이는 문제
// ---------------------------------------------------------------------------

var nTrace []int

func noteN(b *testing.B) {
	nTrace = append(nTrace, b.N)
}

// setupInsideBody 는 준비와 측정 대상을 한 덩어리로 묶은 잘못된 예다.
// 이렇게 하면 매 반복마다 1 KiB 할당이 발생하고 그 비용이 ns/op 에 섞인다.
func setupInsideBody() {
	sinkByte = buildData(dataSize)
	sinkInt += len(sinkByte)
}

// setupOutsideBody 는 준비를 이미 마친 상태에서 측정 대상만 실행한다.
func setupOutsideBody(data []byte) {
	sinkInt += len(data)
}

func benchSetupInside(b *testing.B) {
	noteN(b)
	for i := 0; i < b.N; i++ {
		setupInsideBody()
	}
}

// benchSetupOutside 는 준비를 한 번만 하고 b.ResetTimer 로 측정선을 긋는다.
// ResetTimer 는 시간뿐 아니라 할당 카운터(b.netAllocs, b.netBytes)도 0 으로
// 되돌린다. 그래서 준비 단계의 할당이 allocs/op 에 남지 않는다.
func benchSetupOutside(b *testing.B) {
	noteN(b)
	data := buildData(dataSize)
	sinkByte = data
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		setupOutsideBody(data)
	}
}

// benchStopTimer 는 "매 반복 준비가 필요하지만 측정 대상은 아닌" 경우다.
// 할당은 측정에서 빠지지만, StopTimer/StartTimer 가 매번 runtime.ReadMemStats 를
// 부르기 때문에 그 비용이 ns/op 를 부풀린다.
func benchStopTimer(b *testing.B) {
	noteN(b)
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		key := buildData(64)
		b.StartTimer()
		sinkInt += len(key)
	}
}

// printBench 는 testing.Benchmark 를 돌려 결과를 사람이 읽을 형태로 찍는다.
// 주의: BenchmarkResult.NsPerOp / AllocsPerOp / AllocedBytesPerOp 는 모두 int64 다.
// %f 로 찍으면 vet 이 잡아낸다.
func printBench(label string, f func(*testing.B)) testing.BenchmarkResult {
	nTrace = nil
	r := testing.Benchmark(f)
	fmt.Printf("  %-20s N=%-9d %8d ns/op %4d allocs/op %6d B/op\n",
		label, r.N, r.NsPerOp(), r.AllocsPerOp(), r.AllocedBytesPerOp())
	fmt.Printf("  %-20s 시도한 b.N: %v\n", "", trimTrace(append([]int(nil), nTrace...)))
	return r
}

func demoResetTimer() {
	fmt.Println("[2] 준비 비용이 측정 구간에 섞이면 ns/op 가 무엇을 재는지 알 수 없다")
	printBench("준비를 루프 안에서", benchSetupInside)
	printBench("ResetTimer 로 분리", benchSetupOutside)
	printBench("StopTimer 반복", benchStopTimer)
	fmt.Println("  해석: 첫 줄은 allocs/op 가 1, B/op 가 1024 다. 반복마다 1 KiB 를 새로 만든다는")
	fmt.Println("        뜻이고 그 비용이 ns/op 에 들어 있다. 둘째 줄은 allocs/op 가 0 이다.")
	fmt.Println("        셋째 줄은 할당을 측정에서 뺐지만, StopTimer 가 매번 MemStats 를 읽는")
	fmt.Println("        비용 때문에 ns/op 가 계측 오버헤드로 부풀어 오른다.")
}

// ---------------------------------------------------------------------------
// [3] b.N 은 고정이 아니다
// ---------------------------------------------------------------------------

// trimTrace 는 N 궤적이 길 때 앞뒤만 남긴다. -1 은 생략 표시다.
func trimTrace(tr []int) []int {
	if len(tr) <= 10 {
		return tr
	}
	out := make([]int, 0, 11)
	out = append(out, tr[:5]...)
	out = append(out, -1)
	out = append(out, tr[len(tr)-4:]...)
	return out
}

func benchWork(b *testing.B) {
	noteN(b)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		work(1000)
	}
}

func demoBenchGrowth() {
	fmt.Println("[3] b.N 은 내가 정하는 값이 아니라 측정기가 정한다")
	nTrace = nil
	r1 := testing.Benchmark(benchWork)
	t1 := trimTrace(append([]int(nil), nTrace...))
	nTrace = nil
	r2 := testing.Benchmark(benchWork)
	t2 := trimTrace(append([]int(nil), nTrace...))

	fmt.Printf("  1회차 시도한 N: %v\n", t1)
	fmt.Printf("  1회차 최종 N=%-9d 총 소요 %-12v ns/op=%d\n", r1.N, r1.T, r1.NsPerOp())
	fmt.Printf("  2회차 시도한 N: %v\n", t2)
	fmt.Printf("  2회차 최종 N=%-9d 총 소요 %-12v ns/op=%d\n", r2.N, r2.T, r2.NsPerOp())
	fmt.Println("  해석: 같은 코드인데 두 회차의 N 과 ns/op 가 다르다. -1 은 중간 생략 표시다.")
	fmt.Println("        b.N 을 코드에 박으면 안 되고, 반드시 b.N 을 루프 상한으로 써야 한다.")
}

// ---------------------------------------------------------------------------
// [4] 표본과 유의성
// ---------------------------------------------------------------------------

func demoStats() {
	fmt.Println("[4] 표본을 모아야 유의성을 말할 수 있다 (benchstat 가 하는 일)")
	const target = 5 * time.Millisecond
	base := collect(5, target, func() { work(1000) })
	again := collect(5, target, func() { work(1000) })
	twice := collect(5, target, func() { work(2000) })

	sb, sa, st := summarize(base), summarize(again), summarize(twice)
	fmt.Printf("  A work(1000) 1회차: %v\n", sb)
	fmt.Printf("  A work(1000) 2회차: %v\n", sa)
	fmt.Printf("  B work(2000)      : %v\n", st)
	fmt.Printf("  A1 vs A2 (같은 코드)  유의한가? %v\n", significant(sb, sa))
	fmt.Printf("  A1 vs B   (일 2배)    유의한가? %v\n", significant(sb, st))
	fmt.Println("  해석: 같은 코드끼리는 대개 '유의하지 않음'이 나온다. 이 기준선을 넘지 못하는")
	fmt.Println("        차이는 벤치마크를 다시 돌리면 뒤집힌다. 그걸 걸러내는 도구가 benchstat 이다.")
}

// shortenBenchDemos 는 데모 시간을 줄인다.
// testing.Benchmark 는 "go test" 의 -test.benchtime 플래그 값을 그대로 읽는다.
// go run 으로 돌릴 때는 아무도 그 플래그를 파싱하지 않으므로 기본값 1초가 쓰여
// 데모가 수십 초로 늘어난다. testing.Init 으로 플래그를 등록한 뒤 직접 낮춘다.
// 실패해도 치명적이지 않다(느려질 뿐이다).
func shortenBenchDemos(d time.Duration) {
	testing.Init()
	if f := flag.Lookup("test.benchtime"); f != nil {
		_ = f.Value.Set(d.String())
	}
}

func main() {
	shortenBenchDemos(20 * time.Millisecond)
	fmt.Printf("Go %s  GOMAXPROCS=%d  NumCPU=%d\n\n",
		runtime.Version(), runtime.GOMAXPROCS(0), runtime.NumCPU())

	demoDeadCode()
	fmt.Println()
	demoResetTimer()
	fmt.Println()
	demoBenchGrowth()
	fmt.Println()
	demoStats()
	fmt.Println()
	fmt.Println("여러 번 재서 benchstat 으로 비교하려면:")
	fmt.Println("  go test -run=^$ -bench=. -benchmem -count=10 -benchtime=100ms > new.txt")
}