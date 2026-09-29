package main

import (
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"
)

// cacheLineSize 는 x86-64 계열 CPU 에서 가장 흔한 캐시 라인 크기다.
// arm64 계열 중에는 128바이트 라인도 있지만, 여기서는 관찰 가능한
// 대표값으로 사용한다. 실제 오프셋은 unsafe 로 출력해서 검증한다.
const cacheLineSize = 64

// FalseSharingCounters 는 의도적으로 a 와 b 를 같은 캐시 라인에 배치한다.
// a 가 offset 0, b 가 offset 8 이므로 두 필드는 같은 64바이트 라인에
// 들어간다. 이게 false sharing 을 일으키는 배치다.
type FalseSharingCounters struct {
	a int64
	b int64
}

// PaddedCounters 는 a 가 있는 캐시 라인을 패딩으로 채우고 b 를 다음
// 캐시 라인으로 밀어낸다. 이렇게 하면 a 와 b 는 서로 다른 라인을
// 사용하므로 한쪽 쓰기가 다른 쪽 라인을 무효화하지 않는다.
type PaddedCounters struct {
	a int64
	_ [cacheLineSize - 8]byte
	b int64
}

// falseSharingOffsetB 는 잘못된 배치에서 b 가 몇 바이트 오프셋에
// 있는지 돌려준다. 이 값이 64 미만이면 같은 캐시 라인이다.
func falseSharingOffsetB() uintptr {
	var c FalseSharingCounters
	return unsafe.Offsetof(c.b)
}

// paddedOffsetB 는 패딩 배치에서 b 가 몇 바이트 오프셋에 있는지
// 돌려준다. 이 값이 64 이상이어야 다른 캐시 라인이다.
func paddedOffsetB() uintptr {
	var c PaddedCounters
	return unsafe.Offsetof(c.b)
}

// falseSharingSize 와 paddedSize 는 두 배치가 실제 메모리에서
// 얼마나 다른지 보여주기 위해 구조체 전체 크기를 돌려준다.
func falseSharingSize() uintptr { return unsafe.Sizeof(FalseSharingCounters{}) }
func paddedSize() uintptr { return unsafe.Sizeof(PaddedCounters{}) }

// runFalseSharing 은 두 goroutine 이 각각 a, b 를 n 번 증가시킨다.
// 두 필드가 같은 캐시 라인에 있어서 한쪽 코어가 쓰면 다른 코어의
// 라인을 무효화하는 캐시 일관성 트래픽이 반복된다.
//
// atomic.AddInt64 를 쓰는 이유는 카운터를 정확하게 만들기 위해서다.
// false sharing 은 정확성 문제가 아니라 성능 문제이므로, atomic 을
// 쓰든 plain write 를 쓰든 라인 경쟁은 동일하게 생긴다.
func runFalseSharing(n int) (int64, int64) {
	var c FalseSharingCounters
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		for i := 0; i < n; i++ {
			atomic.AddInt64(&c.a, 1)
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < n; i++ {
			atomic.AddInt64(&c.b, 1)
		}
	}()

	wg.Wait()
	return c.a, c.b
}

// runPadded 는 같은 횟수의 atomic increment 를 패딩 배치에서 수행한다.
// a 와 b 가 서로 다른 캐시 라인에 있으므로 각 코어는 자기 라인만
// 계속 소유하게 된다. 따라서 runFalseSharing 보다 보통 빠르다.
func runPadded(n int) (int64, int64) {
	var c PaddedCounters
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		for i := 0; i < n; i++ {
			atomic.AddInt64(&c.a, 1)
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < n; i++ {
			atomic.AddInt64(&c.b, 1)
		}
	}()

	wg.Wait()
	return c.a, c.b
}

// timeRun 은 fn 을 한 번 실행하고 걸린 시간과 결과 카운터를 출력한다.
// 타이밍은 머신마다 달라지므로 main 은 "이 상황에서 이렇다"를 보여줄
// 뿐이고, 정확한 성능 비교는 benchmark 로 한다.
func timeRun(label string, n int, fn func(int) (int64, int64)) {
	start := time.Now()
	a, b := fn(n)
	elapsed := time.Since(start)
	fmt.Printf("%-18s n=%-9d a=%-9d b=%-9d elapsed=%-12v\n", label, n, a, b, elapsed)
}

func main() {
	fmt.Printf("cacheLineSize(대표값) = %d bytes\n", cacheLineSize)
	fmt.Printf("GOMAXPROCS = %d, NumCPU = %d\n", runtime.GOMAXPROCS(0), runtime.NumCPU())
	fmt.Println("단일 CPU 이거나 GOMAXPROCS=1 이면 false sharing 효과가 잘 안 보일 수 있다.")

	// 구조체 메모리 배치가 false sharing 조건인지 확인한다.
	// 잘못된 배치는 b offset 이 8, 패딩 배치는 b offset 이 64 여야 한다.
	fmt.Printf(
		"FalseSharingCounters size=%-3d offset(a)=0 offset(b)=%d -> 같은 캐시 라인\n",
		falseSharingSize(), falseSharingOffsetB(),
	)
	fmt.Printf(
		"PaddedCounters       size=%-3d offset(a)=0 offset(b)=%d -> 서로 다른 캐시 라인\n",
		paddedSize(), paddedOffsetB(),
	)

	n := 5_000_000
	fmt.Printf("\n각 goroutine 이 %d 번씩 atomic increment 를 수행합니다.\n", n)
	timeRun("false sharing", n, runFalseSharing)
	timeRun("padded", n, runPadded)
}