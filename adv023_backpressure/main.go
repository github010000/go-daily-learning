package main

import (
	"fmt"
	"math"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

// littleQueueDepth 는 Little's Law L = λW 로 필요한 최소 큐 깊이를 구한다.
// arrivalPerSec 이 초당 요청 수, latency 가 서비스 시간 + 큐 대기 시간이라면
// 이 값보다 버퍼가 작으면 목표 latency 를 지킬 수 없다.
func littleQueueDepth(arrivalPerSec float64, latency time.Duration) int {
	return int(math.Ceil(arrivalPerSec * latency.Seconds()))
}

// UnboundedQueue 는 slice append 로 동작하는 무한 버퍼다.
// producer 에게 어떤 배압도 전달하지 않는다.
type UnboundedQueue struct {
	mu     sync.Mutex
	cond   *sync.Cond
	buf    []int
	closed bool
}

func NewUnboundedQueue() *UnboundedQueue {
	q := &UnboundedQueue{}
	q.cond = sync.NewCond(&q.mu)
	return q
}

func (q *UnboundedQueue) Push(v int) {
	q.mu.Lock()
	q.buf = append(q.buf, v)
	q.cond.Signal()
	q.mu.Unlock()
}

func (q *UnboundedQueue) Pop() (int, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	for len(q.buf) == 0 && !q.closed {
		q.cond.Wait()
	}
	if len(q.buf) == 0 {
		return 0, false
	}
	v := q.buf[0]
	q.buf[0] = 0 // 포인터 원소라면 오래된 참조를 남기지 않기 위한 보호 장치
	q.buf = q.buf[1:]
	return v, true
}

func (q *UnboundedQueue) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.buf)
}

func (q *UnboundedQueue) Close() {
	q.mu.Lock()
	q.closed = true
	q.cond.Broadcast()
	q.mu.Unlock()
}

// DropQueue 는 버퍼가 가득 차면 기다리지 않고 요청을 버린다.
// select default 가 곧 부하 차단 신호다.
type DropQueue struct {
	ch      chan int
	dropped uint64
}

func NewDropQueue(capacity int) *DropQueue {
	return &DropQueue{ch: make(chan int, capacity)}
}

func (q *DropQueue) TryPush(v int) bool {
	select {
	case q.ch <- v:
		return true
	default:
		atomic.AddUint64(&q.dropped, 1)
		return false
	}
}

func (q *DropQueue) Pop() (int, bool) {
	v, ok := <-q.ch
	return v, ok
}

func (q *DropQueue) Drain(work time.Duration) {
	for range q.ch {
		time.Sleep(work)
	}
}

func (q *DropQueue) Close() {
	close(q.ch)
}

func (q *DropQueue) Len() int {
	return len(q.ch)
}

func (q *DropQueue) Dropped() uint64 {
	return atomic.LoadUint64(&q.dropped)
}

// BlockingQueue 는 버퍼가 가득 차면 producer goroutine 을 park 시킨다.
// 여기서 park 되는 것이 producer 에게 전달되는 배압이다.
type BlockingQueue struct {
	ch chan int
}

func NewBlockingQueue(capacity int) *BlockingQueue {
	return &BlockingQueue{ch: make(chan int, capacity)}
}

func (q *BlockingQueue) Push(v int) {
	q.ch <- v
}

func (q *BlockingQueue) Pop() int {
	return <-q.ch
}

func (q *BlockingQueue) Drain(work time.Duration) {
	for range q.ch {
		time.Sleep(work)
	}
}

func (q *BlockingQueue) Close() {
	close(q.ch)
}

func (q *BlockingQueue) Len() int {
	return len(q.ch)
}

func formatBytes(b uint64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	units := []string{"KiB", "MiB", "GiB", "TiB"}
	v := float64(b)
	i := -1
	for v >= unit && i < len(units)-1 {
		v /= unit
		i++
	}
	return fmt.Sprintf("%.1f %s", v, units[i])
}

// unboundedMemoryDemo 는 consumer 가 전혀 pop 하지 않는 동안
// 무한 버퍼가 얼마나 많은 힙을 삼키는지 보여준다.
func unboundedMemoryDemo(produce int) {
	q := NewUnboundedQueue()

	// 이전 GC 잔여물을 정리해 큐 때문에 늘어난 힙만 보이게 한다.
	runtime.GC()
	var before runtime.MemStats
	runtime.ReadMemStats(&before)

	for i := 0; i < produce; i++ {
		q.Push(i) // 소비자를 기다리지 않는다. 배압이 없다.
	}

	runtime.GC()
	var after runtime.MemStats
	runtime.ReadMemStats(&after)
	var delta uint64
	if after.HeapAlloc > before.HeapAlloc {
		delta = after.HeapAlloc - before.HeapAlloc
	}

	fmt.Printf("produced=%d queue_depth=%d\n", produce, q.Len())
	fmt.Printf("heap_alloc before=%s after=%s delta=%s\n",
		formatBytes(before.HeapAlloc), formatBytes(after.HeapAlloc), formatBytes(delta))
	fmt.Printf("heap_objects before=%d after=%d\n", before.HeapObjects, after.HeapObjects)
	// 의도적으로 drain 하지 않는다. 무한 버퍼는 함수가 끝날 때까지 모든 항목을 붙잡는다.
}

// runDropScenario 는 cap 만큼만 받고 나머지는 즉시 버리는 시나리오를 돌린다.
func runDropScenario(tasks, capacity int, work time.Duration) (accepted, dropped int, elapsed time.Duration) {
	q := NewDropQueue(capacity)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		q.Drain(work)
	}()

	start := time.Now()
	for i := 0; i < tasks; i++ {
		if q.TryPush(i) {
			accepted++
		}
	}
	q.Close()
	wg.Wait()
	return accepted, tasks - accepted, time.Since(start)
}

// runBlockingScenario 는 같은 cap 을 가진 blocking queue 로 모든 일을 보존하되
// producer 가 큐가 찰 때마다 멈추는 시나리오를 돌린다.
func runBlockingScenario(tasks, capacity int, work time.Duration) (accepted int, elapsed time.Duration) {
	q := NewBlockingQueue(capacity)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		q.Drain(work)
	}()

	start := time.Now()
	for i := 0; i < tasks; i++ {
		q.Push(i) // 큐가 찼을 때 producer goroutine 이 park 된다.
	}
	q.Close()
	wg.Wait()
	return tasks, time.Since(start)
}

func main() {
	fmt.Println("=== Little's Law: queue depth ===")
	const arrival = 1200.0
	work := 25 * time.Millisecond
	depth := littleQueueDepth(arrival, work)
	fmt.Printf("arrival=%.0f/s work=%s => queue must hold at least %d\n", arrival, work, depth)

	fmt.Println("\n=== Unbounded queue hides latency and burns memory ===")
	unboundedMemoryDemo(1_000_000)

	fmt.Println("\n=== Drop vs Blocking ===")
	accepted, dropped, elapsed := runDropScenario(300, 30, 2*time.Millisecond)
	fmt.Printf("DROP   cap=30 tasks=300 work=2ms accepted=%d dropped=%d elapsed=%v\n", accepted, dropped, elapsed)
	accepted, elapsed = runBlockingScenario(300, 30, 2*time.Millisecond)
	fmt.Printf("BLOCK  cap=30 tasks=300 work=2ms accepted=%d elapsed=%v\n", accepted, elapsed)
}