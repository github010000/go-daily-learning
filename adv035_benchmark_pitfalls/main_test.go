package main

import (
	"math"
	"testing"
	"time"
)

// runN 의 계약은 "f 를 정확히 n 번 호출한다"이다. b.N 번 반복이라는 벤치마크의
// 핵심 불변식이라 여기서 고정해 둔다. 이 계약이 깨지면 ns/op 자체가 의미를 잃는다.
// 시간이 아니라 호출 횟수를 세므로 머신 성능이나 CI 부하에 흔들리지 않는다.
func TestRunNCallsExactlyN(t *testing.T) {
	for _, n := range []int{0, 1, 7, 1000} {
		calls := 0
		runN(n, func() { calls++ })
		if calls != n {
			t.Errorf("n=%d 일 때 호출 횟수가 %d 였다", n, calls)
		}
	}
}

// benchN 이 N 을 키우는 방식에 대한 불변식을 검증한다.
// N 궤적은 순증가해야 하고(같은 N 을 두 번 재면 이전 실행의 캐시 상태가 다음
// 측정에 남는다), 반환된 N 은 궤적의 마지막 값이어야 하며, ns/op 는 양수여야 한다.
// "몇 초 안에 끝난다" 같은 시간 단정은 쓰지 않는다.
func TestBenchNInvariants(t *testing.T) {
	n, ns, tr := benchN(func() { sinkInt++ }, time.Millisecond)

	if len(tr) == 0 {
		t.Fatal("N 궤적이 비어 있다")
	}
	if tr[len(tr)-1] != n {
		t.Errorf("반환된 N=%d, 궤적의 마지막 값=%d", n, tr[len(tr)-1])
	}
	for i := 1; i < len(tr); i++ {
		if tr[i] <= tr[i-1] {
			t.Fatalf("N 궤적이 순증가하지 않는다: %v", tr)
		}
	}
	if !(ns > 0) {
		t.Errorf("ns/op 가 양수가 아니다: %v", ns)
	}
}

// summarize 의 평균과 표본표준편차 계산을 손으로 검증 가능한 고정 입력으로 확인한다.
// 이 계산이 틀리면 유의성 판정 전체가 틀어지므로 여기서 못박아 둔다.
func TestSummarize(t *testing.T) {
	if s := summarize(nil); s.N != 0 {
		t.Errorf("빈 입력의 N=%d, 기대 0", s.N)
	}

	xs := []float64{2, 4, 4, 4, 5, 5, 7, 9}
	s := summarize(xs)
	// 평균 5, 모표준편차 2 이므로 표본표준편차는 2*sqrt(8/7) 이다.
	wantSd := 2 * math.Sqrt(8.0/7.0)
	if math.Abs(s.Mean-5) > 1e-9 {
		t.Errorf("평균=%v, 기대 5", s.Mean)
	}
	if math.Abs(s.Stddev-wantSd) > 1e-9 {
		t.Errorf("표본표준편차=%v, 기대 %v", s.Stddev, wantSd)
	}
	if s.Min != 2 || s.Max != 9 {
		t.Errorf("Min/Max=%v/%v, 기대 2/9", s.Min, s.Max)
	}
	if s.N != 8 {
		t.Errorf("N=%d, 기대 8", s.N)
	}
}

// 유의성 판정이 잡음은 걸러내고 큰 차이는 잡아내는지 확인한다.
// 관대해지면 벤치마크 비교가 무의미해지고, 엄격해지면 진짜 개선을 놓친다.
func TestSignificant(t *testing.T) {
	noisy := stats{N: 10, Mean: 100, Stddev: 5}
	same := stats{N: 10, Mean: 102, Stddev: 5}   // 2% 차이, 표준편차에 묻힘
	double := stats{N: 10, Mean: 200, Stddev: 5} // 2배 차이

	if significant(noisy, same) {
		t.Errorf("2%% 차이를 유의하다고 판정했다 (표준편차 5%%)")
	}
	if !significant(noisy, double) {
		t.Errorf("2배 차이를 유의하지 않다고 판정했다")
	}
	if significant(stats{}, stats{}) {
		t.Errorf("빈 표본을 유의하다고 판정했다")
	}
	if significant(stats{N: 10, Mean: 100, Stddev: 0}, stats{N: 10, Mean: 100.5, Stddev: 0}) {
		t.Errorf("0.5%% 차이를 유의하다고 판정했다 (최소 효과 크기 1%% 미만)")
	}
}

// sink 가 컴파일러의 제거를 막는지 확인한다. 지키려는 계약은 두 가지다.
//  1) sink 에 넣으면 최소 1 allocs/op 가 실제로 발생한다.
//  2) 결과를 버린 쪽이 sink 쪽보다 더 많은 할당을 보고하지는 않는다.
// 컴파일러가 변환을 지우지 않는 버전에서는 두 값이 같아질 수 있으므로
// 등호가 아니라 부등호로 검사한다. 그래야 컴파일러가 똑똑해져도 테스트가 살아남는다.
func TestSinkPreventsElimination(t *testing.T) {
	buf := buildData(1 << 10)

	discard := allocsDiscard(buf)
	sink := allocsSink(buf)

	if sink < 1 {
		t.Fatalf("sink 사용 시 allocs/op=%v, 기대값 1 이상", sink)
	}
	if discard > sink {
		t.Errorf("결과를 버린 쪽(%v allocs/op)이 sink 쪽(%v allocs/op)보다 많다: 측정이 잘못됐다",
			discard, sink)
	}
	if len(sinkStr) != len(buf) {
		t.Errorf("sink 에 저장된 길이 %d, 기대 %d", len(sinkStr), len(buf))
	}
}

// ResetTimer 가 준비 비용을 측정 구간에서 실제로 빼는지, 그 원리를 검사한다.
// 시간이 아니라 할당 카운터를 보는 이유: 할당 수는 결정적이라서 CI 부하나
// 머신 성능에 흔들리지 않는다. testing.Benchmark 를 직접 부르지 않는 이유도
// 같다. 그 함수는 -test.benchtime 동안 실제로 돌기 때문에 테스트 시간이 커진다.
func TestTimerAccountingExcludesSetup(t *testing.T) {
	data := buildData(dataSize)

	inside := testing.AllocsPerRun(50, setupInsideBody)
	outside := testing.AllocsPerRun(50, func() { setupOutsideBody(data) })

	if inside < 1 {
		t.Errorf("준비를 측정 구간 안에 두면 allocs/op 가 1 이상이어야 하는데 %v 였다", inside)
	}
	if outside >= 1 {
		t.Errorf("준비를 측정 구간 밖으로 빼면 allocs/op 가 0 이어야 하는데 %v 였다", outside)
	}
	if len(sinkByte) != dataSize {
		t.Errorf("sinkByte 길이 %d, 기대 %d", len(sinkByte), dataSize)
	}
}

// ---------------------------------------------------------------------------
// 실제 benchmark 함수들.
// go test -run=^$ -bench=. -benchmem -count=10 > new.txt 로 돌려 benchstat 에 넣는다.
// -count 를 주지 않으면 표본이 1개뿐이라 유의성 판정을 할 수 없다.
// ---------------------------------------------------------------------------

// BenchmarkStringConvDiscard 는 결과를 버리는 벤치마크다.
// benchmem 출력에서 0 allocs/op 가 나오면 측정된 것이 없다는 신호다.
func BenchmarkStringConvDiscard(b *testing.B) {
	buf := buildData(dataSize)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = string(buf)
	}
}

// BenchmarkStringConvSink 는 같은 일을 sink 로 붙잡아 둔 버전이다.
// 위 벤치마크와 나란히 놓아야 "무엇이 사라졌는지"가 숫자로 보인다.
func BenchmarkStringConvSink(b *testing.B) {
	buf := buildData(dataSize)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sinkStr = string(buf)
	}
}

// BenchmarkWork 는 반복 횟수에 거의 선형인 기준선이다.
// 다른 벤치마크의 ns/op 가 이 값의 배수로 읽히는지 확인할 때 쓴다.
func BenchmarkWork(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		work(1000)
	}
}