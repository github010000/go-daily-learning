package main

import (
	"sync"
	"testing"
)

// TestPublishState는 atomic.Bool을 발행 플래그로 쓸 때 Go 메모리 모델이
// data 쓰기와 ready Store/Load 사이에 happens-before를 만드는지 검증한다.
// ready=true를 관찰한 reader는 반드시 writer가 쓴 data 값을 봐야 한다.
func TestPublishState(t *testing.T) {
	const trials = 1000

	for i := 0; i < trials; i++ {
		p := newPublishState()
		expected := int64(i + 1)
		var wg sync.WaitGroup
		wg.Add(2)

		go func(v int64) {
			defer wg.Done()
			p.publish(v)
		}(expected)

		go func(v int64) {
			defer wg.Done()
			got := p.read()
			if got != v {
				t.Errorf("memory ordering broken: got %d want %d", got, v)
			}
		}(expected)

		wg.Wait()
	}
}

// TestAtomicCounterSum은 여러 goroutine이 atomic.Int64를 동시에 증가시켜도
// 읽기-수정-쓰기가 원자적으로 수행되어 합계가 정확히 보존되는지 확인한다.
func TestAtomicCounterSum(t *testing.T) {
	const workers = 16
	const perWorker = 1000
	got := runAtomicCounterDemo(workers, perWorker)
	want := int64(workers * perWorker)

	if got != want {
		t.Fatalf("atomic counter sum = %d, want %d", got, want)
	}
}

// TestMutexCounterSum은 atomic과 비교하기 위해 sync.Mutex도 같은 합계를
// 보여주는지 확인한다. 둘 다 동시성 안전을 보장해야 한다.
func TestMutexCounterSum(t *testing.T) {
	const workers = 16
	const perWorker = 1000
	got := runMutexCounterDemo(workers, perWorker)
	want := int64(workers * perWorker)

	if got != want {
		t.Fatalf("mutex counter sum = %d, want %d", got, want)
	}
}

// BenchmarkAtomicCounter는 단일 goroutine에서 atomic.Int64.Add의 처리량을 잰다.
// b.ResetTimer()를 루프 앞에 두어 준비 비용을 측정에서 제외한다.
func BenchmarkAtomicCounter(b *testing.B) {
	c := &atomicCounter{}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.inc()
	}
}

// BenchmarkMutexCounter는 동일 조건에서 sync.Mutex.Lock/Unlock의 처리량을 잰다.
// 경합이 없는 fast path에서도 atomic보다는 느리다는 것이 이 벤치마크에서 관찰된다.
func BenchmarkMutexCounter(b *testing.B) {
	c := &mutexCounter{}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.inc()
	}
}