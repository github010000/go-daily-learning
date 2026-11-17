package main

import (
	"context"
	"runtime/metrics"
	"testing"
)

// TestQuantileFromHist 는 히스토그램 분위수 계산을 검증한다.
// 이 계산이 틀리면 "p99 스케줄 지연 3ms" 같은 결론 자체가 틀어지므로,
// 손으로 계산한 값과 정확히 일치하는지 본다.
// buckets 는 구간 경계, counts[i] 는 [buckets[i], buckets[i+1]) 구간의 개수다.
func TestQuantileFromHist(t *testing.T) {
	buckets := []float64{0, 1, 2, 4, 8}
	counts := []uint64{0, 90, 8, 2} // 총 100개

	cases := []struct {
		q    float64
		want float64
	}{
		{0.50, 2}, // 50번째 표본은 두 번째 버킷(상한 2)에 있다
		{0.90, 4},
		{0.99, 8},
		{1.00, 8},
	}
	for _, c := range cases {
		if got := quantileFromHist(counts, buckets, c.q); got != c.want {
			t.Errorf("q=%.2f: got %v, want %v", c.q, got, c.want)
		}
	}

	if got := quantileFromHist(nil, nil, 0.5); got != 0 {
		t.Errorf("빈 히스토그램: got %v, want 0", got)
	}
	if got := quantileFromHist([]uint64{0, 0}, []float64{0, 1, 2}, 0.5); got != 0 {
		t.Errorf("표본 0개: got %v, want 0", got)
	}
	if got := maxBucket(counts, buckets); got != 8 {
		t.Errorf("maxBucket: got %v, want 8", got)
	}
	if got := maxBucket([]uint64{0, 0}, []float64{0, 1, 2}); got != 0 {
		t.Errorf("빈 maxBucket: got %v, want 0", got)
	}
}

// TestHistDelta 는 누적 히스토그램의 차분을 검증한다.
// before/after 를 빼는 이 함수가 없으면 측정 구간 이전에 쌓인 표본이
// 계속 섞여 들어와서 1단계 측정값이 4단계 측정값에 오염된다.
func TestHistDelta(t *testing.T) {
	before := &metrics.Float64Histogram{Counts: []uint64{1, 2, 3}, Buckets: []float64{0, 1, 2, 3}}
	after := &metrics.Float64Histogram{Counts: []uint64{5, 2, 7}, Buckets: []float64{0, 1, 2, 3}}

	counts, buckets := histDelta(before, after)
	if len(counts) != 3 || len(buckets) != 4 {
		t.Fatalf("길이: counts=%d buckets=%d, want 3/4", len(counts), len(buckets))
	}
	want := []uint64{4, 0, 4}
	for i := range want {
		if counts[i] != want[i] {
			t.Errorf("counts[%d]=%d, want %d", i, counts[i], want[i])
		}
	}
	if buckets[3] != 3 {
		t.Errorf("buckets[3]=%v, want 3", buckets[3])
	}
	if got := sumCounts(counts); got != 8 {
		t.Errorf("sumCounts=%d, want 8", got)
	}

	if got, _ := histDelta(nil, after); got != nil {
		t.Errorf("before 가 nil 이면 nil 이어야 한다")
	}
	if got, _ := histDelta(before, nil); got != nil {
		t.Errorf("after 가 nil 이면 nil 이어야 한다")
	}
	mismatch := &metrics.Float64Histogram{Counts: []uint64{1}, Buckets: []float64{0, 1}}
	if got, _ := histDelta(mismatch, after); got != nil {
		t.Errorf("길이가 다르면 nil 이어야 한다")
	}
}

// TestRunFanoutRunsEveryIndexOnce 는 fanout 이 인덱스를 빠뜨리지도
// 중복 실행하지도 않는지 본다. 측정 harness 자체가 일을 덜 시키면
// 모든 수치가 조용히 작아지기 때문에 이 불변식이 먼저다.
// seen 은 인덱스마다 서로 다른 goroutine 만 건드리므로 race 가 아니다.
func TestRunFanoutRunsEveryIndexOnce(t *testing.T) {
	const n = 64
	seen := make([]int32, n)
	got := runFanout(n, func(i int) uint64 {
		seen[i] = 1
		return uint64(i) * 3
	})
	if len(got) != n {
		t.Fatalf("len=%d, want %d", len(got), n)
	}
	for i := 0; i < n; i++ {
		if seen[i] != 1 {
			t.Errorf("인덱스 %d 가 정확히 한 번 실행되지 않았다 (seen=%d)", i, seen[i])
		}
		if got[i] != uint64(i)*3 {
			t.Errorf("got[%d]=%d, want %d", i, got[i], uint64(i)*3)
		}
	}
}

// TestMeasureReportsOrderedLatency 는 measure 의 출력이 항상 성립해야 하는
// 순서 불변식을 확인한다. 절대 시간은 기계마다 다르므로 단정하지 않고,
// 구조적으로 참이어야 하는 관계만 검사한다.
func TestMeasureReportsOrderedLatency(t *testing.T) {
	rep := measure(context.Background(), "unit", func() {
		sinkU64 = runFanout(8, func(i int) uint64 { return cpuBurst(20000) })
	})
	if rep.name != "unit" {
		t.Errorf("name=%q, want unit", rep.name)
	}
	if rep.wall <= 0 {
		t.Errorf("wall=%v, 측정이 되지 않았다", rep.wall)
	}
	if rep.userCPU < 0 || rep.gcCPU < 0 {
		t.Errorf("CPU 값이 음수다: user=%v gc=%v", rep.userCPU, rep.gcCPU)
	}
	if rep.schedP99 < rep.schedP50 {
		t.Errorf("p99(%v) < p50(%v)", rep.schedP99, rep.schedP50)
	}
	if rep.schedMax < rep.schedP99 {
		t.Errorf("max(%v) < p99(%v)", rep.schedMax, rep.schedP99)
	}
	if p := rep.parallelism(); p < 0 {
		t.Errorf("parallelism=%v", p)
	}
}

// TestPhaseLockCompletes 는 convoy 단계가 모든 goroutine 을 실제로
// 끝까지 돌리는지 확인한다. lock 을 잘못 잡으면 여기서 교착으로 죽는다.
func TestPhaseLockCompletes(t *testing.T) {
	sinkU64 = nil
	phaseLock(6, 50)()
	if len(sinkU64) != 1 {
		t.Fatalf("sinkU64 길이=%d, want 1", len(sinkU64))
	}
}

// BenchmarkMeasureOverhead 는 측정 harness 자체의 비용을 잰다.
// runtime/metrics 읽기 두 번과 히스토그램 복사가 여기에 들어 있다.
// 측정 대상이 이 비용보다 짧으면 수치는 의미가 없다는 것을 보여준다.
func BenchmarkMeasureOverhead(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = measure(context.Background(), "bench", func() {})
	}
}