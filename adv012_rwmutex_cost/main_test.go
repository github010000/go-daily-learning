package main

import (
	"sync"
	"sync/atomic"
	"testing"
)

// sumFirstN은 1부터 n까지의 합을 돌려준다.
func sumFirstN(n int) int {
	return n * (n + 1) / 2
}

// TestReadSumCorrectness: 잠금 종류가 달라도 읽기 결과는 같아야 한다.
// Mutex와 RWMutex 모두 같은 dataSize에 대해 같은 합을 반환하는지 검사한다.
// 그래야 main.go의 처리량 차이가 계산 오류 때문이 아님을 보장할 수 있다.
func TestReadSumCorrectness(t *testing.T) {
	const size = 4
	want := sumFirstN(size)

	m := newMutexState(size)
	r := newRWMutexState(size)

	for i := 0; i < 100; i++ {
		if got := m.readSumMutex(); got != want {
			t.Fatalf("mutex readSum = %d, want %d", got, want)
		}
		if got := r.readSumRWMutex(); got != want {
			t.Fatalf("rwmutex readSum = %d, want %d", got, want)
		}
	}
}

// TestConcurrentReadSumsNoRace: 여러 goroutine이 같은 읽기 전용 상태를
// 동시에 읽더라도 결과가 모두 같아야 하고, -race 검사에서도 안전해야 한다.
// Mutex와 RWMutex 둘 다 동시 reader 상황에서 정확성을 유지하는지 검증한다.
func TestConcurrentReadSumsNoRace(t *testing.T) {
	const size = 16
	want := sumFirstN(size)

	m := newMutexState(size)
	r := newRWMutexState(size)

	const goroutines = 8
	const iterations = 200

	var wg sync.WaitGroup
	var bad atomic.Bool

	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				if got := m.readSumMutex(); got != want {
					bad.Store(true)
					return
				}
				if got := r.readSumRWMutex(); got != want {
					bad.Store(true)
					return
				}
			}
		}()
	}
	wg.Wait()

	if bad.Load() {
		t.Fatalf("some concurrent read returned wrong sum")
	}
}

// BenchmarkParallelMutexShortRead: 짧은 읽기 구간을 여러 goroutine으로 돌려
// Mutex의 직렬 처리량을 관찰한다.
func BenchmarkParallelMutexShortRead(b *testing.B) {
	m := newMutexState(4)
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			m.readSumMutex()
		}
	})
}

// BenchmarkParallelRWMutexShortRead: 같은 짧은 읽기를 RWMutex.RLock으로 돌려
// readerCount 원자 연산과 캐시 라인 경합이 오버헤드로 나타나는지 본다.
func BenchmarkParallelRWMutexShortRead(b *testing.B) {
	r := newRWMutexState(4)
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			r.readSumRWMutex()
		}
	})
}

// BenchmarkParallelMutexLongRead: 계산이 긴 읽기 구간에서는 Mutex가
// 모든 reader를 직렬화하므로 처리량이 낮아질 것인지 관찰한다.
func BenchmarkParallelMutexLongRead(b *testing.B) {
	m := newMutexState(4096)
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			m.readSumMutex()
		}
	})
}

// BenchmarkParallelRWMutexLongRead: 계산이 긴 읽기 구간에서는 RWMutex가
// 여러 reader를 동시에 진행시켜 Mutex보다 빠를 수 있음을 관찰한다.
func BenchmarkParallelRWMutexLongRead(b *testing.B) {
	r := newRWMutexState(4096)
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			r.readSumRWMutex()
		}
	})
}