package main

import (
	"runtime"
	"testing"
)

// cpuBound 는 결정적이므로 같은 입력에는 항상 같은 결과를 내야 한다.
// 루프 합계 공식과 workerID 반영을 순차 계산으로 재현해 검증한다.
func TestCPUBoundDeterministic(t *testing.T) {
	iterations := 1000
	got := cpuBound(7, iterations)

	want := uint64(0)
	for i := 0; i < iterations; i++ {
		want += uint64(i)
	}
	want += uint64(7)

	if got != want {
		t.Fatalf("cpuBound(7, %d) = %d, want %d", iterations, got, want)
	}
}

// runParallel 이 workers 개수만큼 goroutine을 실제로 실행하고
// 각 결과를 정확히 기록하는지 검증한다.
// 시간에 의존하지 않고 개수와 값이라는 불변식만 확인한다.
func TestRunParallelCompletesAllWorkers(t *testing.T) {
	workers := 16
	iterations := 100000
	results := runParallel(workers, iterations)

	if len(results) != workers {
		t.Fatalf("len(results) = %d, want %d", len(results), workers)
	}

	for i := 0; i < workers; i++ {
		want := cpuBound(i, iterations)
		if results[i] != want {
			t.Errorf("results[%d] = %d, want %d", i, results[i], want)
		}
	}
}

// cooperativeYieldWorkers 는 모든 goroutine이 Gosched 후에도
// 카운터를 정확히 workers*1000 회 증가시키는지 검증한다.
// 실행 순서가 아니라 총 횟수라는 불변식을 확인한다.
func TestCooperativeYieldWorkersCount(t *testing.T) {
	workers := 8
	got := cooperativeYieldWorkers(workers)
	want := workers * 1000

	if got != want {
		t.Fatalf("cooperativeYieldWorkers(%d) = %d, want %d", workers, got, want)
	}
}

// BenchmarkRunParallel 은 현재 GOMAXPROCS에서 runParallel 의 처리량을 측정한다.
// CPU-bound goroutine을 여러 개 돌릴 때 P 개수에 따른 확장성을 실험하기 위한
// 기준 숫자를 제공한다.
func BenchmarkRunParallel(b *testing.B) {
	workers := runtime.GOMAXPROCS(0)
	iterations := 100000

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		runParallel(workers, iterations)
	}
}