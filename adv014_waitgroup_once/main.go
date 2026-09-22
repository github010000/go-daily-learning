package main

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// high32Delta converts a signed counter delta into the upper 32 bits of the
// WaitGroup state word. sync.WaitGroup does exactly this: Add(delta) does
// state.Add(uint64(delta) << 32) so one atomic add updates counter only.
func high32Delta(delta int64) uint64 {
	return uint64(delta) << 32
}

// simulatePackedState prints the packed 64-bit state that sync.WaitGroup
// maintains internally. This is not sync.WaitGroup itself, but the same
// arithmetic so the high/low split becomes visible.
func simulatePackedState() {
	var state atomic.Uint64

	state.Store(uint64(2) << 32) // counter=2, waiters=0
	printPacked("Add(2) 후", state.Load())

	state.Add(1) // Wait 등록: waiter 1 증가
	printPacked("Wait 등록 후", state.Load())

	state.Add(high32Delta(-1)) // Done: counter 1 감소
	printPacked("Done 후", state.Load())
}

func printPacked(label string, v uint64) {
	counter := int32(v >> 32)
	waiters := uint32(v)
	fmt.Printf("%-16s: counter=%d waiters=%d raw=0x%016x\n", label, counter, waiters, v)
}

const defaultWorkers = 8

// runCorrectWaitGroup starts n workers after Add(n) and waits with Wait.
// Add must happen before the goroutine is scheduled, otherwise Wait may return
// before the workers have registered themselves.
func runCorrectWaitGroup(n int) int64 {
	var wg sync.WaitGroup
	var completed atomic.Int64

	wg.Add(n) // 핵심: 새 goroutine이 아니라 만드는 쪽에서 Add 한다.
	for i := 0; i < n; i++ {
		go func(id int) {
			defer wg.Done()
			completed.Add(1)
			// 실제 작업 대신 counter 증가만 기록한다.
		}(i)
	}
	wg.Wait()
	return completed.Load()
}

// runWrongAddInGoroutine shows the classic broken pattern: each goroutine calls
// Add(1) just before its own Done. The workers are fenced behind start so
// Wait() is guaranteed to run while counter is still zero. This makes the
// premature return deterministic instead of schedule-dependent.
func runWrongAddInGoroutine(n int) (returnedBefore, finalCompleted int64) {
	var wg sync.WaitGroup
	var completed atomic.Int64

	start := make(chan struct{})
	done := make(chan struct{}, n)

	for i := 0; i < n; i++ {
		go func(id int) {
			<-start
			wg.Add(1) // 잘못된 위치: Wait 호출이 이미 반환한 뒤 실행된다.
			defer wg.Done()
			completed.Add(1)
			done <- struct{}{}
		}(i)
	}

	wg.Wait() // counter=0 이므로 즉시 반환한다.
	returnedBefore = completed.Load()
	close(start)

	// WaitGroup이 기다리지 않았으므로 별도의 done 채널로 자식을 수거한다.
	for i := 0; i < n; i++ {
		<-done
	}
	finalCompleted = completed.Load()
	return returnedBefore, finalCompleted
}

// runNegativeAddPanic triggers the negative counter panic by calling Done
// without Add. This shows why the high 32 bits are treated as signed int32.
func runNegativeAddPanic() (msg string) {
	defer func() {
		if r := recover(); r != nil {
			msg = fmt.Sprint(r)
		}
	}()
	var wg sync.WaitGroup
	wg.Done() // counter 0 -> -1
	return "no panic"
}

// runOnce exercises sync.Once under concurrency. Exactly one goroutine must
// execute the function even when n goroutines race through Do.
func runOnce(n int) int64 {
	var once sync.Once
	var calls atomic.Int64
	var wg sync.WaitGroup

	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			once.Do(func() {
				calls.Add(1)
			})
		}()
	}
	wg.Wait()
	return calls.Load()
}

// countingDoubleOnce is a teaching copy of sync.Once's double-checked locking.
// It records how many calls hit the lock-free fast path and how many enter the
// slow path. sync.Once itself does not expose these counters.
type countingDoubleOnce struct {
	mu       sync.Mutex
	done     atomic.Uint32
	fastPath atomic.Int64
	slowPath atomic.Int64
}

func (o *countingDoubleOnce) Do(f func()) {
	if o.done.Load() == 1 {
		o.fastPath.Add(1)
		return
	}
	o.slowPath.Add(1)
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.done.Load() == 0 {
		defer o.done.Store(1)
		f()
	}
}

// runCountingOnceAfterFirst warms up the Once, then launches n more calls.
// After warm-up every call should use the atomically checked fast path.
func runCountingOnceAfterFirst(n int) (fast, slow int64) {
	var once countingDoubleOnce
	once.Do(func() {}) // first call goes to slow path and stores done

	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			once.Do(func() {})
		}()
	}
	wg.Wait()
	return once.fastPath.Load(), once.slowPath.Load()
}

// runOnceRecursiveDeadlock shows that calling the same Once from its own f
// deadlocks. The outer call holds the mutex while f waits for the inner call.
func runOnceRecursiveDeadlock() string {
	var once sync.Once
	done := make(chan struct{})
	go func() {
		defer close(done)
		once.Do(func() {
			once.Do(func() {}) // inner Do needs the mutex held by outer Do
		})
	}()

	select {
	case <-done:
		return "no deadlock"
	case <-time.After(500 * time.Millisecond):
		return "deadlock: inner Do waits for outer Do"
	}
}

func main() {
	fmt.Println("=== 1. WaitGroup packed state simulation ===")
	simulatePackedState()

	fmt.Println()
	fmt.Println("=== 2. WaitGroup correct Add-before-go ===")
	completed := runCorrectWaitGroup(defaultWorkers)
	fmt.Printf("Wait 반환 시 completed = %d/%d\n", completed, defaultWorkers)

	fmt.Println()
	fmt.Println("=== 3. WaitGroup misuse: Add inside goroutine ===")
	returned, final := runWrongAddInGoroutine(defaultWorkers)
	fmt.Printf("Wait 반환 시 completed=%d/%d, 실제 최종 완료=%d/%d\n",
		returned, defaultWorkers, final, defaultWorkers)
	fmt.Println("따라서 Wait 가 작업을 기다렸다고 말할 수 없다.")

	fmt.Println()
	fmt.Println("=== 4. WaitGroup negative counter panic ===")
	fmt.Println("panic:", runNegativeAddPanic())

	fmt.Println()
	fmt.Println("=== 5. sync.Once exactly once under race ===")
	fmt.Printf("실행된 f 횟수 = %d (기대 1)\n", runOnce(defaultWorkers))

	fmt.Println()
	fmt.Println("=== 6. Double-checked locking observation ===")
	fast, slow := runCountingOnceAfterFirst(defaultWorkers)
	fmt.Printf("warm-up 후 %d회 호출: fast path=%d, slow path=%d\n", defaultWorkers, fast, slow)

	fmt.Println()
	fmt.Println("=== 7. Once recursive deadlock ===")
	fmt.Println(runOnceRecursiveDeadlock())
}