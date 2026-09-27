package main

import (
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

type byte32 struct {
	b [32]byte
}

var (
	sinkPtr *byte32
	sinkVal byte32
)

// victimCacheResults 는 sync.Pool 이 GC 와 상호작용하는 방식을 관찰한다.
// 객체는 1회 GC 에서 victim cache 로 이동해 재사용 가능하지만,
// 2회 연속 GC 동안 Get 되지 않으면 사라진다.
//
// sync.Pool 의 local private 필드는 P 에 귀속된다. 다른 P 의 private 은
// 훔칠 수 없기 때문에, goroutine 이 GC 를 거치며 다른 P 로 이동하면
// private 에 있던 객체를 victim cache 에서도 찾지 못할 수 있다.
// 여기서는 victim cache 수명을 결정적으로 보여주기 위해 GOMAXPROCS 를
// 잠시 1 로 고정한다. 함수를 빠져나갈 때 이전 값을 복원한다.
func victimCacheResults() (reusedAfterOneGC bool, clearedAfterTwoGC bool, totalNewCalls int64) {
	old := runtime.GOMAXPROCS(1)
	defer runtime.GOMAXPROCS(old)

	var count int64
	pool := sync.Pool{
		New: func() any {
			atomic.AddInt64(&count, 1)
			return new(int)
		},
	}

	original := pool.Get().(*int)
	*original = 42
	pool.Put(original)

	runtime.GC() // 첫 번째 GC: local -> victim
	got := pool.Get()
	reusedAfterOneGC = got.(*int) == original

	pool.Put(got) // 재사용한 객체를 다시 local 에 넣음
	runtime.GC()   // 두 번째 GC: local -> victim, 이전 victim 은 비워짐
	runtime.GC()   // 세 번째 GC: victim 제거
	got2 := pool.Get()
	clearedAfterTwoGC = got2.(*int) != original

	totalNewCalls = atomic.LoadInt64(&count)
	return
}

func printVictimCacheDemo() {
	fmt.Println("== 1) GC 1회는 victim cache 에서 재사용, GC 2회는 소멸 ==")
	reused, cleared, calls := victimCacheResults()
	fmt.Printf("1번 GC 직후 Get: 같은 객체 재사용=%v\n", reused)
	fmt.Printf("2번 GC 직후 Get: 새 객체 생성=%v\n", cleared)
	fmt.Printf("총 New 호출 수=%d (처음 1번 + 마지막 새 객체 1번)\n", calls)
	fmt.Println()
}

// measureLoop 는 fn 실행 동안 새로 만들어진 객체 수(allocation events)와
// 경과 시간을 반환한다. 시작 전에 명시적으로 GC 를 돌려 이전 측정의 영향을 줄인다.
func measureLoop(fn func()) (allocCount uint64, elapsed time.Duration) {
	var ms runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&ms)
	before := ms.Mallocs
	start := time.Now()
	fn()
	elapsed = time.Since(start)
	runtime.ReadMemStats(&ms)
	allocCount = ms.Mallocs - before
	return
}

// allocationCounts 는 같은 32바이트 객체를 세 가지 방식으로 다룰 때의
// allocation event 수와 시간을 측정한다. 작은 객체에서는 방식을 잘못 고르면
// pool 이 오히려 이득이 없음을 보여준다.
func allocationCounts(n int) (direct, pointerPool, valuePool uint64, directTime, ptrTime, valTime time.Duration) {
	// 1) 직접 new(byte32) — 매 회 heap allocation
	direct, directTime = measureLoop(func() {
		for i := 0; i < n; i++ {
			x := new(byte32)
			x.b[0] = byte(i)
			sinkPtr = x
		}
	})

	// 2) pool 에 *byte32 를 넣는 올바른 방식. warm up 으로 poolLocal, poolChain
	//    내부 구조를 초기화하고, 이후 GC 에서도 재사용되도록 한다.
	p := sync.Pool{New: func() any { return new(byte32) }}
	warm := p.Get().(*byte32)
	p.Put(warm)
	pointerPool, ptrTime = measureLoop(func() {
		for i := 0; i < n; i++ {
			x := p.Get().(*byte32)
			x.b[0] = byte(i)
			p.Put(x)
			sinkPtr = x
		}
	})

	// 3) pool 에 byte32 값을 직접 넣는 잘못된 방식. Put/Get 마다 interface boxing 이
	//    일어나 heap allocation 이 발생한다.
	vp := sync.Pool{New: func() any { return byte32{} }}
	warmVal := vp.Get().(byte32)
	vp.Put(warmVal)
	valuePool, valTime = measureLoop(func() {
		for i := 0; i < n; i++ {
			x := vp.Get().(byte32)
			x.b[0] = byte(i)
			vp.Put(x)
			sinkVal = x
		}
	})

	return
}

func printAllocationBoundary() {
	fmt.Println("== 2) 작은 객체(32B)에서 pool 의 할당 횟수/시간 비교 ==")
	n := 200_000
	direct, ptrPool, valPool, dTime, pTime, vTime := allocationCounts(n)
	fmt.Printf("직접 할당      : %6d alloc / %6d loop, %v\n", direct, n, dTime)
	fmt.Printf("pool(포인터)   : %6d alloc / %6d loop, %v\n", ptrPool, n, pTime)
	fmt.Printf("pool(값 타입) : %6d alloc / %6d loop, %v\n", valPool, n, vTime)
	fmt.Println()
}

func main() {
	printVictimCacheDemo()
	printAllocationBoundary()
}