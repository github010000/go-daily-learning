package main

import (
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"
)

// sync.Mutex 의 state 필드 비트 정의.
// go/src/sync/mutex.go 의 상수와 동일한 값이다.
const (
	mutexLocked = 1 << iota // 1 - 잠김
	mutexWoken              // 2 - 깨어난 goroutine 이 있음
	mutexStarving           // 4 - starvation mode 진입
	mutexWaiterShift = iota // 3 - 대기 goroutine 수가 저장되는 shift
)

// mutexInternal 은 현재 Go 런타임의 sync.Mutex 앞부분 레이아웃을 흉내낸다.
// 실제 정의는 go/src/sync/mutex.go 에서:
//
//	type Mutex struct {
//		state int32
//		sema  uint32
//	}
//
// unsafe 를 쓰는 이유는 런타임 내부를 관찰하기 위한 교육용이다.
// 프로덕션 코드에서는 이런 방식으로 sync.Mutex 내부를 읽지 말 것.
type mutexInternal struct {
	state int32
	sema  uint32
}

// loadMutexState 는 sync.Mutex 의 state 필드를 원자적으로 읽는다.
// atomic.LoadInt32 를 사용하므로 -race 에서도 데이터 레이스로 판정되지 않는다.
func loadMutexState(mu *sync.Mutex) int32 {
	pm := (*mutexInternal)(unsafe.Pointer(mu))
	return atomic.LoadInt32(&pm.state)
}

// readMutexState 는 sync.Mutex 의 내부 상태 비트를 뜯어 반환한다.
// 반환값은 각각 locked, woken, starving 플래그와 대기 goroutine 수다.
// 대기 goroutine 수는 state >> mutexWaiterShift 로 얻는다.
func readMutexState(mu *sync.Mutex) (locked, woken, starving bool, waiterCount int) {
	s := loadMutexState(mu)
	locked = s&mutexLocked != 0
	woken = s&mutexWoken != 0
	starving = s&mutexStarving != 0
	waiterCount = int(s >> mutexWaiterShift)
	return
}

// formatMutexState 는 사람이 알아보기 쉬운 문자열로 내부 상태를 표현한다.
func formatMutexState(mu *sync.Mutex) string {
	locked, woken, starving, waiters := readMutexState(mu)
	raw := loadMutexState(mu)
	return fmt.Sprintf(
		"locked=%v woken=%v starving=%v waiters=%d state=%#x",
		locked, woken, starving, waiters, raw,
	)
}

// demoSpinAndStarvation 은 goroutine 이 락을 기다릴 때 처음에는 spin 하다가
// 시간이 지나면 semaphore 에서 잠드는 과정을 보여준다.
//
// spin 은 짧은 시간 동안만 일어나므로 관찰이 어렵다. 이 함수는 GOMAXPROCS 를
// 올려 여러 goroutine 이 서로 다른 P 위에서 spin 하게 만든 뒤, state 를 주기적으로
// 샘플링해 waiterCount 가 0 -> N 으로 늘어나는 모습을 보여준다.
func demoSpinAndStarvation() {
	// spin 이 일어나려면 P 가 2개 이상이어야 한다. 단일 P 환경에서는
	// runtime_canSpin 이 false 를 반환해 바로 잠든다.
	runtime.GOMAXPROCS(4)

	var mu sync.Mutex
	mu.Lock()

	const waiters = 4
	var wg sync.WaitGroup
	wg.Add(waiters)

	for i := 0; i < waiters; i++ {
		go func(id int) {
			defer wg.Done()
			// Lock() 은 처음에는 spin 을 시도하고, 실패하면 semaphore 에서 잠든다.
			mu.Lock()
			mu.Unlock()
		}(i)
	}

	fmt.Println("spin -> sleep transition: state sampled every 50us")
	for i := 0; i < 10; i++ {
		fmt.Printf("t=%3dus %s\n", i*50, formatMutexState(&mu))
		time.Sleep(50 * time.Microsecond)
	}

	mu.Unlock()
	wg.Wait()
	fmt.Printf("after unlock: %s\n", formatMutexState(&mu))
}

// tryInduceStarvation 은 normal mode 의 barging 이 starvation mode 로 이어지는
// 순간을 재현하려고 시도한다.
//
// 시나리오:
//  1. main 이 락을 오래(2ms) 잡고 있는다.
//  2. goroutine 하나가 락을 기다린다. 대기 시간이 1ms 를 넘긴다.
//  3. main 이 Unlock 과 동시에 TryLock 으로 락을 다시 훔친다(barging).
//  4. 훔치는 데 성공하면 깨어난 waiter 는 락이 아직 잡혀 있음을 보고
//     대기 시간이 1ms 를 초과했으므로 mutexStarving 비트를 세운다.
//
// 성공 여부는 실행 스케줄에 따라 달라진다. main 에서 여러 번 시도하면
// 결국 starving=true 인 상태를 관찰할 수 있다.
func tryInduceStarvation() (bool, string) {
	var mu sync.Mutex
	mu.Lock()

	ready := make(chan struct{})
	done := make(chan struct{})

	go func() {
		close(ready) // 곧 Lock 을 호출할 것임을 알린다.
		mu.Lock()    // main 이 Unlock 할 때까지 블록.
		close(done)
		mu.Unlock()
	}()

	<-ready
	// waiter goroutine 이 실제로 mu.Lock() 안에서 블록하도록 잠시 기다려 준다.
	time.Sleep(100 * time.Microsecond)

	// 1ms starvation threshold 를 확실히 넘기도록 2ms 를 더 붙잡는다.
	time.Sleep(2 * time.Millisecond)

	// 락을 놓자마자 TryLock 으로 새 도착 goroutine 처럼 행동해 훔친다.
	// TryLock 은 성공하면 true, waiter 가 먼저 잡았으면 false 를 반환한다.
	mu.Unlock()
	stolen := mu.TryLock()

	var state string
	var starving bool

	if stolen {
		// waiter 가 깨어나서 우리가 훔친 락을 보고 starvation 플래그를
		// 세울 시간을 준다. Gosched 는 같은 P 에서 waiter 가 실행될
		// 기회를 높여 준다.
		runtime.Gosched()
		time.Sleep(500 * time.Microsecond)

		state = formatMutexState(&mu)
		_, _, starving, _ = readMutexState(&mu)

		// 우리가 훔친 락을 풀어야 waiter 가 진행하고 done 을 닫는다.
		mu.Unlock()
	} else {
		// barging 실패: waiter 가 락을 가져갔으므로 state 는 그냥 관찰만 한다.
		state = formatMutexState(&mu)
		_, _, starving, _ = readMutexState(&mu)
	}

	// 실험용 goroutine 이 살아 있는 채로 끝나지 않도록 done 을 기다린다.
	select {
	case <-done:
	case <-time.After(time.Second):
		// 정상적으로는 즉시 닫힌다. 만약 스케줄이 늦으면 그냥 보고용으로 둔다.
	}

	return starving, state
}

// safeCounter 는 sync.Mutex 가 공유 변수를 어떻게 보호하는지 테스트에서
// 검증하기 위한 단순한 카운터다.
type safeCounter struct {
	mu  sync.Mutex
	val int
}

func (c *safeCounter) Inc() {
	c.mu.Lock()
	c.val++
	c.mu.Unlock()
}

func (c *safeCounter) Value() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.val
}

func main() {
	fmt.Println("=== sync.Mutex internal state via unsafe (education only) ===")

	var mu sync.Mutex
	fmt.Println("initial:", formatMutexState(&mu))

	mu.Lock()
	fmt.Println("after Lock:", formatMutexState(&mu))

	mu.Unlock()
	fmt.Println("after Unlock:", formatMutexState(&mu))

	fmt.Println()
	fmt.Println("=== spin -> sleep transition ===")
	demoSpinAndStarvation()

	fmt.Println()
	fmt.Println("=== induce starvation via barging + 1ms wait ===")
	for i := 1; i <= 20; i++ {
		starving, state := tryInduceStarvation()
		fmt.Printf("trial %02d: %s\n", i, state)
		if starving {
			fmt.Println("starvation mode observed: 오래 기다린 waiter 가 mutexStarving 을 세웠다.")
			break
		}
	}
}