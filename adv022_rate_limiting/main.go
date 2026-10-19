package main

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// chanSemaphore는 버퍼 채널을 세마포어로 활용한다.
// 채널의 내장 대기열(sendq/recvq)이 곧 세마포어의 대기열이 되기 때문에
// 별도의 mutex와 cond를 만들지 않아도 goroutine이 블로킹되고 깨어난다.
type chanSemaphore chan struct{}

func newChanSemaphore(n int) chanSemaphore {
	return make(chan struct{}, n)
}

// acquire는 채널에 값을 보내는 P 연산이다.
// 버퍼가 가득 차면 sendq에 대기열이 생기고 goroutine은 스케줄러에 의해 잠든다.
func (s chanSemaphore) acquire() {
	s <- struct{}{}
}

// release는 채널에서 값을 꺼내는 V 연산이다.
// 대기 중인 sendq가 있으면 런타임이 해당 goroutine을 깨워 다시 보내게 한다.
func (s chanSemaphore) release() {
	<-s
}

// weightedSemaphore는 채널 세마포어와 달리 goroutine마다 서로 다른 양의
// 자원을 소비해야 할 때 사용한다. mutex로 상태를 보호하고 cond로 대기열을 만든다.
// golang.org/x/sync/semaphore의 Weighted와 같은 원리다.
type weightedSemaphore struct {
	mu   sync.Mutex
	cond *sync.Cond
	cur  int64 // 현재 사용 중인 자원 합
	max  int64 // 허용 가능한 자원 합
}

func newWeightedSemaphore(max int64) *weightedSemaphore {
	w := &weightedSemaphore{max: max}
	w.cond = sync.NewCond(&w.mu)
	return w
}

// acquire는 n개의 자원을 확보할 수 있을 때까지 대기한다.
// 조건이 만족되지 않으면 cond.Wait로 뮤텍스를 놓고 잠든다.
// 깨어난 뒤에는 반드시 for로 조건을 다시 검사해야 한다(Spurious Wakeup 대비).
func (w *weightedSemaphore) acquire(n int64) {
	w.mu.Lock()
	for w.cur+n > w.max {
		w.cond.Wait()
	}
	w.cur += n
	w.mu.Unlock()
}

// release는 n개의 자원을 반환한 뒤 대기 중인 모든 goroutine을 깨운다.
// 실제 x/sync/semaphore는 효율을 위해 깨울 대상을 더 정밀하게 고르지만
// 여기서는 올바른 동작에 집중한다.
func (w *weightedSemaphore) release(n int64) {
	w.mu.Lock()
	w.cur -= n
	w.cond.Broadcast()
	w.mu.Unlock()
}

// tokenBucket은 토큰 버킷 rate limiter를 단순하게 구현한 것이다.
// rate는 초당 보충되는 토큰 수, burst는 버킷이 담을 수 있는 최대 토큰 수다.
// burst가 1보다 크면 초기에 몰려오는 요청을 일부 즉시 허용할 수 있다.
type tokenBucket struct {
	mu     sync.Mutex
	tokens float64   // 현재 가용 토큰 수
	rate   float64   // 초당 보충 토큰 수
	burst  int       // 버킷 최대 크기
	last   time.Time // 마지막으로 토큰을 보충한 시각
}

func newTokenBucket(rate float64, burst int) *tokenBucket {
	return &tokenBucket{
		tokens: float64(burst),
		rate:   rate,
		burst:  burst,
		last:   time.Now(),
	}
}

// reserveN은 now 시점에 n개의 토큰을 요청했을 때 기다려야 하는 시간을 계산한다.
// 토큰이 충분하면 0을 반환하고, 부족하면 부족분이 채워질 때까지의 시간을 반환한다.
// x/time/rate의 ReserveN과 동일한 계산 원리이며, 시간을 주입받아 테스트에 유리하다.
func (tb *tokenBucket) reserveN(now time.Time, n int) time.Duration {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	// 마지막 보충 이후 경과 시간만큼 토큰을 추가한다.
	// now가 last보다 과거라면(벽시계 조정 등) 경과를 0으로 본다.
	elapsed := now.Sub(tb.last).Seconds()
	if elapsed < 0 {
		elapsed = 0
	}
	tb.tokens += elapsed * tb.rate
	if tb.tokens > float64(tb.burst) {
		tb.tokens = float64(tb.burst)
	}
	tb.last = now

	need := float64(n)
	if tb.tokens >= need {
		tb.tokens -= need
		return 0
	}

	// 부족한 토큰을 채우는 데 필요한 시간을 계산한다.
	deficit := need - tb.tokens
	waitSeconds := deficit / tb.rate
	tb.tokens = 0 // 남은 토큰은 모두 이번 요청을 위해 소비한 것으로 처리한다.
	return time.Duration(waitSeconds * float64(time.Second))
}

// wait는 reserveN으로 대기 시간을 계산한 뒤 실제로 time.Sleep을 호출한다.
// 실제 사용에서는 context 취소를 지원하는 x/time/rate.Wait를 사용하는 편이 낫다.
func (tb *tokenBucket) wait(n int) time.Duration {
	d := tb.reserveN(time.Now(), n)
	if d > 0 {
		time.Sleep(d)
	}
	return d
}

// updateMax는 CAS를 사용해 여러 goroutine이 동시에 접근해도 정확한 최댓값을 유지한다.
func updateMax(max *int64, cur int64) {
	for {
		old := atomic.LoadInt64(max)
		if cur <= old || atomic.CompareAndSwapInt64(max, old, cur) {
			return
		}
	}
}

// runConcurrencyDemo는 동시성 제한이 없을 때, 채널 세마포어일 때,
// weighted semaphore일 때의 최대 동시 실행 수를 비교 출력한다.
// 모두 가중치 1을 사용하므로 두 세마포어의 동시성 제한 결과는 같아야 한다.
func runConcurrencyDemo() {
	fmt.Println("== 동시성 제한 비교 ==")
	const jobs = 30
	const limit = 3

	max := runWithoutLimit(jobs)
	fmt.Printf("제한 없음: 최대 동시 실행 %d\n", max)

	max = runWithChanSemaphore(limit, jobs)
	fmt.Printf("채널 세마포어(limit=%d): 최대 동시 실행 %d\n", limit, max)

	max = runWithWeightedSemaphore(limit, jobs)
	fmt.Printf("weighted semaphore(limit=%d): 최대 동시 실행 %d\n", limit, max)
}

// runWithoutLimit은 세마포어 없이 모든 작업을 동시에 실행한다.
// 이 수치는 동시성 제한이 없을 때 작업이 얼마나 한꺼번에 실행되는지 보여준다.
func runWithoutLimit(jobs int) int {
	var active, max int64
	var wg sync.WaitGroup
	for i := 0; i < jobs; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cur := atomic.AddInt64(&active, 1)
			updateMax(&max, cur)
			time.Sleep(time.Millisecond)
			atomic.AddInt64(&active, -1)
		}()
	}
	wg.Wait()
	return int(atomic.LoadInt64(&max))
}

// runWithChanSemaphore는 버퍼 채널 세마포어로 동시 실행 수를 limit으로 제한한다.
func runWithChanSemaphore(limit, jobs int) int {
	sem := newChanSemaphore(limit)
	var active, max int64
	var wg sync.WaitGroup
	for i := 0; i < jobs; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem.acquire()
			defer sem.release()
			cur := atomic.AddInt64(&active, 1)
			updateMax(&max, cur)
			time.Sleep(time.Millisecond)
			atomic.AddInt64(&active, -1)
		}()
	}
	wg.Wait()
	return int(atomic.LoadInt64(&max))
}

// runWithWeightedSemaphore는 weighted semaphore로 동시 실행 수를 limit으로 제한한다.
// 가중치 1만 사용하므로 채널 세마포어와 동일한 결과가 나온다.
func runWithWeightedSemaphore(limit, jobs int) int {
	sem := newWeightedSemaphore(int64(limit))
	var active, max int64
	var wg sync.WaitGroup
	for i := 0; i < jobs; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem.acquire(1)
			defer sem.release(1)
			cur := atomic.AddInt64(&active, 1)
			updateMax(&max, cur)
			time.Sleep(time.Millisecond)
			atomic.AddInt64(&active, -1)
		}()
	}
	wg.Wait()
	return int(atomic.LoadInt64(&max))
}

// runRateLimitDemo는 token bucket rate limiter를 시연한다.
// 동시성 제한과 달리 rate limit은 시간당 처리량에 개입한다는 점을 보여준다.
func runRateLimitDemo() {
	fmt.Println("\n== rate limiting: token bucket ==")
	// 초당 2개, 버스트 3개: 처음 3개는 즉시, 이후 500ms 간격으로 허용된다.
	tb := newTokenBucket(2, 3)

	fmt.Println("-- rate limit 없이 (비교) --")
	for i := 0; i < 10; i++ {
		fmt.Printf("요청 %d: 대기 0, 시각 %s\n", i, time.Now().Format("15:04:05.000"))
	}

	fmt.Println("-- rate limit 적용 --")
	for i := 0; i < 10; i++ {
		wait := tb.wait(1)
		fmt.Printf("요청 %d: 대기 %v, 시각 %s\n",
			i, wait, time.Now().Format("15:04:05.000"))
	}
}

func main() {
	runConcurrencyDemo()
	runRateLimitDemo()
}