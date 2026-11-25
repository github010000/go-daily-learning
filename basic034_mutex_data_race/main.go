package main

import (
	"fmt"
	"sync"
	"sync/atomic"
)

// RacyCounter 는 아무 보호 없이 n 을 올리는 카운터입니다.
// n++ 는 "읽기 → 더하기 → 쓰기" 세 단계라서, 두 고루틴이 같은 값을 읽으면
// 한 번의 증가가 사라집니다.
type RacyCounter struct{ n int }

func (c *RacyCounter) Inc() { c.n++ }

// SafeCounter 는 Mutex 로 n 을 보호합니다.
// Mutex 는 반드시 구조체 안에 두고, 메서드는 포인터 리시버로 받아야 합니다.
// 값 리시버를 쓰면 호출할 때마다 Mutex 가 복사되어 잠금이 서로 무관해집니다.
type SafeCounter struct {
	mu sync.Mutex
	n  int
}

func (c *SafeCounter) Inc() {
	c.mu.Lock()
	defer c.mu.Unlock() // 중간에 return 하거나 panic 이 나도 반드시 풀리게 defer 로 둡니다.
	c.n++
}

// incLocked 는 "이미 잠금을 쥔 상태에서만 부른다"는 약속의 내부 함수입니다.
// Go 의 Mutex 는 재진입(reentrant)이 안 되므로, 잠금을 쥔 채 Inc 를 또 부르면
// 자기 자신을 영원히 기다립니다. 그래서 잠금 없는 버전을 따로 둡니다.
func (c *SafeCounter) incLocked() { c.n++ }

func (c *SafeCounter) IncTwice() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.incLocked()
	c.incLocked()
}

func (c *SafeCounter) Value() int {
	c.mu.Lock() // 읽기도 잠가야 합니다. 쓰기만 잠그면 여전히 data race 입니다.
	defer c.mu.Unlock()
	return c.n
}

// runConcurrently 는 workers 개의 고루틴이 각각 perWorker 번 inc 를 부르고,
// 모두 끝날 때까지 기다립니다. wg.Wait() 이후의 읽기는 안전합니다(happens-before).
func runConcurrently(workers, perWorker int, inc func()) {
	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range perWorker {
				inc()
			}
		}()
	}
	wg.Wait()
}

// RunRacy 는 실수 버전입니다. 결과가 실행마다 다르고, -race 로 돌리면 경고가 뜹니다.
// 테스트에서는 부르지 않습니다.
func RunRacy(workers, perWorker int) int {
	c := &RacyCounter{}
	runConcurrently(workers, perWorker, c.Inc)
	return c.n
}

// RunSafe 는 고친 버전입니다. 항상 workers*perWorker 가 나옵니다.
func RunSafe(workers, perWorker int) int {
	c := &SafeCounter{}
	runConcurrently(workers, perWorker, c.Inc)
	return c.Value()
}

// RunAtomic 은 단순 카운터라면 Mutex 보다 가벼운 atomic 으로 충분하다는 것을 보여 줍니다.
func RunAtomic(workers, perWorker int) int64 {
	var n atomic.Int64
	runConcurrently(workers, perWorker, func() { n.Add(1) })
	return n.Load()
}

// Cache 는 읽기가 많은 맵을 RWMutex 로 보호합니다.
// 읽기끼리는 동시에 들어올 수 있고, 쓰기는 혼자만 들어옵니다.
type Cache struct {
	mu sync.RWMutex
	m  map[string]int
}

func NewCache() *Cache { return &Cache{m: make(map[string]int)} }

func (c *Cache) Get(k string) (int, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	v, ok := c.m[k]
	return v, ok
}

func (c *Cache) Set(k string, v int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.m[k] = v
}

// IncrBroken: 읽고(RLock) → 풀고 → 쓰는(Lock) 사이에 다른 고루틴이 끼어듭니다.
// 맵 접근 자체는 모두 잠겨 있어서 race detector 는 아무 말도 하지 않지만,
// "읽은 값이 쓸 때까지 유효하다"는 가정이 깨져 증가가 사라집니다(논리적 race).
func (c *Cache) IncrBroken(k string) {
	c.mu.RLock()
	v := c.m[k]
	c.mu.RUnlock()

	c.mu.Lock()
	c.m[k] = v + 1
	c.mu.Unlock()
}

// Incr 는 읽기와 쓰기를 하나의 잠금 구간 안에 넣은 고친 버전입니다.
func (c *Cache) Incr(k string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.m[k]++
}

// RunCacheIncr 는 같은 키를 여러 고루틴이 올립니다. broken 이 true 면 실수 버전입니다.
func RunCacheIncr(workers, perWorker int, broken bool) int {
	c := NewCache()
	inc := func() { c.Incr("hit") }
	if broken {
		inc = func() { c.IncrBroken("hit") }
	}
	runConcurrently(workers, perWorker, inc)
	v, _ := c.Get("hit")
	return v
}

func demoCounter() {
	const workers, per = 100, 1000
	want := workers * per
	fmt.Println("== Mutex 없는 카운터 vs Mutex 카운터 ==")
	fmt.Println("기대값:", want)
	fmt.Println("실수 버전(보호 없음):", RunRacy(workers, per), "← 보통 기대값보다 작다(우연히 같을 수도 있음)")
	fmt.Println("고친 버전(Mutex):    ", RunSafe(workers, per))
	fmt.Println("고친 버전(atomic):   ", RunAtomic(workers, per))
}

func demoCache() {
	const workers, per = 50, 1000
	fmt.Println("== RWMutex: 읽기-수정-쓰기를 나눠 잠그면 ==")
	fmt.Println("기대값:", workers*per)
	fmt.Println("실수 버전(RLock 후 Lock):", RunCacheIncr(workers, per, true), "← race 경고는 없는데 값이 틀린다")
	fmt.Println("고친 버전(한 번에 Lock): ", RunCacheIncr(workers, per, false))
}

func demoReentrant() {
	fmt.Println("== 잠금 쥔 채 내부 함수 호출 ==")
	c := &SafeCounter{}
	c.IncTwice()
	fmt.Println("IncTwice 한 번 호출 후 값:", c.Value())
}

func main() {
	demoCounter()
	demoCache()
	demoReentrant()
}