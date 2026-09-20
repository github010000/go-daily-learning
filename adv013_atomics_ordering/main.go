package main

import (
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

// publishState는 atomic.Bool을 발행 플래그로 사용하는 올바른 패턴이다.
// Go의 sync/atomic은 순차 일관성(sequential consistency)만 제공하므로
// Store 이전에 쓴 일반 메모리 값은 그 Store를 관찰한 Load 이후에 반드시 보인다.
// 이 성질 때문에 개발자가 memory_order_relaxed/acquire 같은 것을 고르지 않아도 된다.
type publishState struct {
	data  int64
	ready atomic.Bool
}

func newPublishState() *publishState {
	return &publishState{}
}

func (p *publishState) publish(v int64) {
	p.data = v            // 1. 일반 메모리에 데이터 쓰기
	p.ready.Store(true)   // 2. 원자 연산으로 발행 신호를 준다
}

func (p *publishState) read() int64 {
	// ready가 true가 될 때까지 양보하며 대기한다.
	// atomic.Bool.Load는 원자 연산이므로 비원자 bool 플래그와 달리 data race가 없다.
	for !p.ready.Load() {
		runtime.Gosched()
	}
	// ready.Load()가 true를 관찰했다면 그 Store와 happens-before 관계가 생기고
	// 그 이전의 data 쓰기도 반드시 관찰된다.
	return p.data
}

// unsafePublish는 Go가 atomic 없이는 어떤 문제를 겪는지 보여주기 위한 의도적인
// data race 유발용 구조체다. go run -race . 로 실행하면 여기서 경쟁이 보고된다.
type unsafePublish struct {
	data  int64
	ready bool
}

func (u *unsafePublish) publish(v int64) {
	u.data = v
	u.ready = true
}

// read는 maxSpins까지만 대기한다. plain bool은 원자성이 없고 순서 보장도 없어
// 스케줄러나 컴파일러 최적화에 따라 값이 안 보이거나 오래된 값이 보일 수 있다.
func (u *unsafePublish) read(maxSpins int) (int64, bool) {
	for i := 0; i < maxSpins; i++ {
		if u.ready {
			return u.data, true
		}
		runtime.Gosched()
	}
	return 0, false
}

type atomicCounter struct {
	value atomic.Int64
}

func (a *atomicCounter) inc() {
	a.value.Add(1)
}

func (a *atomicCounter) get() int64 {
	return a.value.Load()
}

type mutexCounter struct {
	mu    sync.Mutex
	value int64
}

func (m *mutexCounter) inc() {
	m.mu.Lock()
	m.value++
	m.mu.Unlock()
}

func (m *mutexCounter) get() int64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.value
}

// spinMutex는 CAS 루프로 mutex를 흉내 낸다.
// 잠금 대기 중에도 CAS를 반복하며 CPU를 소모하고, runtime의 park/unpark 체계와
// 무관하게 동작하므로 애플리케이션에서 sync.Mutex를 대체하면 안 된다.
type spinMutex struct {
	locked  atomic.Bool
	retries atomic.Int64
}

func (s *spinMutex) lock() {
	for !s.locked.CompareAndSwap(false, true) {
		s.retries.Add(1)
		runtime.Gosched()
	}
}

func (s *spinMutex) unlock() {
	s.locked.Store(false)
}

// runPublishDemo는 atomic 발행 패턴이 반복 수행에서도 값이 유실되지 않는지 확인한다.
// 여러 goroutine이 동시에 하나의 publishState에 접근하지만 data race는 없다.
func runPublishDemo(trials int) (success int64, total int64) {
	var succ atomic.Int64
	var tot atomic.Int64

	for i := 0; i < trials; i++ {
		expected := int64(i + 1)
		p := newPublishState()
		var wg sync.WaitGroup
		wg.Add(2)

		go func(v int64) {
			defer wg.Done()
			p.publish(v)
		}(expected)

		go func(v int64) {
			defer wg.Done()
			if got := p.read(); got == v {
				succ.Add(1)
			}
			tot.Add(1)
		}(expected)

		wg.Wait()
	}

	return succ.Load(), tot.Load()
}

// runUnsafePublishDemo는 비원자 bool 플래그가 발행 신호로 부적합함을 보여준다.
// 이 함수는 의도적으로 data race를 일으키는 시연 전용이며 테스트에서는 호출하지 않는다.
func runUnsafePublishDemo(trials int) (success int64, total int64, missing int64) {
	var succ atomic.Int64
	var tot atomic.Int64
	var miss atomic.Int64

	for i := 0; i < trials; i++ {
		u := &unsafePublish{}
		expected := int64(i + 1)
		var wg sync.WaitGroup
		wg.Add(2)

		go func(v int64) {
			defer wg.Done()
			time.Sleep(time.Millisecond)
			u.publish(v)
		}(expected)

		go func(v int64) {
			defer wg.Done()
			tot.Add(1)
			got, ok := u.read(10_000_000)
			if !ok {
				miss.Add(1)
				return
			}
			if got == v {
				succ.Add(1)
			}
		}(expected)

		wg.Wait()
	}

	return succ.Load(), tot.Load(), miss.Load()
}

// runAtomicCounterDemo는 여러 goroutine이 atomic.Int64를 동시에 증가시킨다.
func runAtomicCounterDemo(workers, perWorker int) int64 {
	c := &atomicCounter{}
	var wg sync.WaitGroup

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < perWorker; j++ {
				c.inc()
			}
		}()
	}

	wg.Wait()
	return c.get()
}

// runMutexCounterDemo는 동일한 작업을 sync.Mutex로 수행한다.
func runMutexCounterDemo(workers, perWorker int) int64 {
	c := &mutexCounter{}
	var wg sync.WaitGroup

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < perWorker; j++ {
				c.inc()
			}
		}()
	}

	wg.Wait()
	return c.get()
}

// runSpinMutexDemo는 CAS spinlock이 값을 정확히 보호하는지와
// 실제로 얼마나 많은 CAS 재시도를 하는지 수치로 보여준다.
func runSpinMutexDemo(workers, perWorker int) (count int64, retries int64) {
	var c atomic.Int64
	sm := &spinMutex{}
	var wg sync.WaitGroup

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < perWorker; j++ {
				sm.lock()
				c.Add(1)
				sm.unlock()
			}
		}()
	}

	wg.Wait()
	return c.Load(), sm.retries.Load()
}

func main() {
	fmt.Println("1) atomic.Bool 발행 플래그")
	succ, total := runPublishDemo(10_000)
	fmt.Printf("safe publish: success=%d/%d\n", succ, total)

	fmt.Println("\n2) plain bool 발행 플래그")
	unsafeSucc, unsafeTotal, missing := runUnsafePublishDemo(100)
	fmt.Printf("unsafe publish: success=%d/%d missing=%d\n", unsafeSucc, unsafeTotal, missing)
	fmt.Println("   go run -race . 로 실행하면 2번 구간에서 data race 가 보고된다.")

	fmt.Println("\n3) atomic vs mutex 카운터 정합성")
	workers, per := 8, 5000
	atomicSum := runAtomicCounterDemo(workers, per)
	mutexSum := runMutexCounterDemo(workers, per)
	expected := int64(workers * per)
	fmt.Printf("atomic sum=%d mutex sum=%d expected=%d\n", atomicSum, mutexSum, expected)

	fmt.Println("\n4) CAS spinlock 재시도 횟수")
	spinSum, retries := runSpinMutexDemo(workers, per)
	fmt.Printf("spinlocked sum=%d CAS retries=%d\n", spinSum, retries)
	fmt.Println("   CAS retries 가 크다는 것은 잠금 경합에서 goroutine 이")
	fmt.Println("   잠들지 못하고 CPU 를 태우며 재시도한다는 뜻이다.")
}