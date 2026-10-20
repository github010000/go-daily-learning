package main

import (
	"testing"
	"time"
)

// TestLittleQueueDepthMatchesLittleLaw 는 L = λW 계산이 정확한지 확인한다.
// 시간 의존 단정이 아니라 수식 자체를 검증한다.
func TestLittleQueueDepthMatchesLittleLaw(t *testing.T) {
	// 1000/s * 20ms = 20
	if got := littleQueueDepth(1000, 20*time.Millisecond); got != 20 {
		t.Fatalf("littleQueueDepth(1000, 20ms) = %d, want 20", got)
	}
	// 1234/s * 5ms = 6.17 -> ceil 로 7
	if got := littleQueueDepth(1234, 5*time.Millisecond); got != 7 {
		t.Fatalf("littleQueueDepth(1234, 5ms) = %d, want 7", got)
	}
}

// TestUnboundedQueueGrowsWithoutBackpressure 는 무한 버퍼가 producer 를 막지 않고
// 모든 값이 FIFO 순서로 보존되는지 검증한다.
func TestUnboundedQueueGrowsWithoutBackpressure(t *testing.T) {
	q := NewUnboundedQueue()
	const n = 10000

	for i := 0; i < n; i++ {
		q.Push(i)
	}
	if q.Len() != n {
		t.Fatalf("Len() after %d pushes = %d, want %d", n, q.Len(), n)
	}

	for i := 0; i < n; i++ {
		v, ok := q.Pop()
		if !ok {
			t.Fatalf("Pop() at index %d: unexpectedly closed", i)
		}
		if v != i {
			t.Fatalf("Pop() at index %d = %d, want %d", i, v, i)
		}
	}
	if q.Len() != 0 {
		t.Fatalf("Len() after draining all = %d, want 0", q.Len())
	}
}

// TestDropQueueBoundsQueueDepth 는 용량을 초과하면 즉시 drop 되고
// 큐 깊이가 capacity 를 넘지 않는 불변식을 확인한다.
func TestDropQueueBoundsQueueDepth(t *testing.T) {
	const capacity = 16
	q := NewDropQueue(capacity)
	accepted := 0

	for i := 0; i < 100; i++ {
		if q.TryPush(i) {
			accepted++
		}
	}

	if accepted != capacity {
		t.Fatalf("accepted = %d, want capacity %d", accepted, capacity)
	}
	if got := q.Len(); got != capacity {
		t.Fatalf("Len() after overflow = %d, want %d", got, capacity)
	}
	if dropped := q.Dropped(); dropped != 100-capacity {
		t.Fatalf("Dropped() = %d, want %d", dropped, 100-capacity)
	}

	for i := 0; i < capacity; i++ {
		v, ok := q.Pop()
		if !ok {
			t.Fatalf("Pop() at index %d: channel closed unexpectedly", i)
		}
		if v != i {
			t.Fatalf("Pop() at index %d = %d, want %d", i, v, i)
		}
	}
	if q.Len() != 0 {
		t.Fatalf("Len() after drain = %d, want 0", q.Len())
	}
}

// TestBlockingQueuePreservesAllWork 는 blocking queue 가 데이터를 버리지 않으면서
// producer 를 큐 용량에서 잠시 멈추게 하는 동작을 검증한다.
// 순서와 개수만을 단정하므로 시간에 의존하지 않는다.
func TestBlockingQueuePreservesAllWork(t *testing.T) {
	q := NewBlockingQueue(2)

	q.Push(10)
	q.Push(20)
	if q.Len() != 2 {
		t.Fatalf("Len() after two pushes = %d, want 2", q.Len())
	}

	pushed := make(chan struct{})
	go func() {
		q.Push(30) // cap = 2 이므로 하나를 꺼낼 때까지 여기서 멈춘다.
		close(pushed)
	}()

	// 두 개를 꺼내야 Push(30) 이 완료된다. 큐의 FIFO 순서를 검증한다.
	if v := q.Pop(); v != 10 {
		t.Fatalf("first Pop() = %d, want 10", v)
	}
	if v := q.Pop(); v != 20 {
		t.Fatalf("second Pop() = %d, want 20", v)
	}

	// Push(30) goroutine 이 끝났을 때만 channel 이 닫힌다.
	<-pushed

	if v := q.Pop(); v != 30 {
		t.Fatalf("third Pop() = %d, want 30", v)
	}
	if q.Len() != 0 {
		t.Fatalf("Len() after drain = %d, want 0", q.Len())
	}
}

// BenchmarkUnboundedQueuePush 는 무한 버퍼가 할당을 얼마나 일으키는지 보여준다.
func BenchmarkUnboundedQueuePush(b *testing.B) {
	q := NewUnboundedQueue()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		q.Push(i)
	}
}

// BenchmarkDropQueueTryPush 는 drop 경로가 큐가 찬 뒤에도 할당 없이 도는지 보여준다.
func BenchmarkDropQueueTryPush(b *testing.B) {
	q := NewDropQueue(16)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		q.TryPush(i)
	}
}