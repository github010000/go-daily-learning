package main

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestBlockingSyscallWorker는 블로킹 syscall 고루틴이 정확한 횟수만큼
// syscall을 호출하는지 검증한다. duration을 짧게 주어 테스트를 빠르게 한다.
func TestBlockingSyscallWorker(t *testing.T) {
	var wg sync.WaitGroup
	var count int64
	iters := 20
	duration := 1 * time.Millisecond

	wg.Add(1)
	go blockingSyscallWorker(0, duration, iters, &wg, &count)
	wg.Wait()

	if atomic.LoadInt64(&count) != int64(iters) {
		t.Fatalf("expected %d syscalls, got %d", iters, count)
	}
}

// TestCPUWorker는 cpuBoundWorker가 계산을 정확히 누적하는지 검증한다.
// progress가 0보다 크고, result가 기대 합과 일치해야 한다.
func TestCPUWorker(t *testing.T) {
	var wg sync.WaitGroup
	var progress int64
	var result int64
	iters := 10000

	wg.Add(1)
	go cpuBoundWorker(0, iters, &wg, &progress, &result)
	wg.Wait()

	expected := int64(0)
	for i := 0; i < iters; i++ {
		expected += int64(i) * 2
	}

	if atomic.LoadInt64(&result) != expected {
		t.Fatalf("expected result %d, got %d", expected, result)
	}
	if atomic.LoadInt64(&progress) <= 0 {
		t.Fatalf("expected progress > 0, got %d", progress)
	}
}

// TestConcurrentWorkers는 블로킹 syscall 고루틴과 CPU 바운드 고루틴을
// 동시에 실행했을 때 race 없이 모든 작업이 완료되는지 확인한다.
// 이는 -race 플래그로 실행해도 통과해야 한다.
func TestConcurrentWorkers(t *testing.T) {
	var wg sync.WaitGroup
	var syscallCount int64
	var cpuProgress int64
	var cpuResult int64

	syscallIters := 5
	cpuIters := 1000
	duration := 1 * time.Millisecond

	wg.Add(3)
	go blockingSyscallWorker(1, duration, syscallIters, &wg, &syscallCount)
	go cpuBoundWorker(1, cpuIters, &wg, &cpuProgress, &cpuResult)
	go cpuBoundWorker(2, cpuIters, &wg, &cpuProgress, &cpuResult)
	wg.Wait()

	if atomic.LoadInt64(&syscallCount) != int64(syscallIters) {
		t.Fatalf("expected %d syscalls, got %d", syscallIters, syscallCount)
	}
	expectedCPU := int64(0)
	for i := 0; i < cpuIters; i++ {
		expectedCPU += int64(i) * 2
	}
	expectedTotal := expectedCPU * 2 // 두 CPU 고루틴
	if atomic.LoadInt64(&cpuResult) != expectedTotal {
		t.Fatalf("expected total CPU result %d, got %d", expectedTotal, cpuResult)
	}
	if atomic.LoadInt64(&cpuProgress) <= 0 {
		t.Fatalf("expected cpu progress > 0, got %d", cpuProgress)
	}
}

// BenchmarkBlockingSyscallWorker는 syscall 고루틴의 반복 비용을 측정한다.
func BenchmarkBlockingSyscallWorker(b *testing.B) {
	var count int64
	duration := 1 * time.Microsecond
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var wg sync.WaitGroup
		wg.Add(1)
		go blockingSyscallWorker(0, duration, 1, &wg, &count)
		wg.Wait()
	}
}

// BenchmarkCPUWorker는 CPU 바운드 작업의 성능을 측정한다.
func BenchmarkCPUWorker(b *testing.B) {
	var progress int64
	var result int64
	iters := 1000
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var wg sync.WaitGroup
		wg.Add(1)
		go cpuBoundWorker(0, iters, &wg, &progress, &result)
		wg.Wait()
	}
}