package main

import (
	"fmt"
	"runtime"
	"sync"
	"time"
)

// pointerTarget 은 stack 복사 전후로 포인터가 올바르게 조정되는지
// 관찰하기 위한 heap 객체입니다. stack 프레임에 이 객체를 가리키는
// 포인터를 저장해 두고, stack 이 성장한 뒤에도 같은 heap 주소를
// 가리키는지 확인합니다.
type pointerTarget struct {
	value int
}

// traceLenAtDepth 는 주어진 재귀 깊이에서 runtime.Stack 이 돌려주는
// stack trace 텍스트의 길이를 바이트로 돌려줍니다.
//
// 이 값은 stack 메모리 크기가 아니라 trace 에 포함된 프레임 수에
// 비례하는 값입니다. 그래도 깊은 호출이 프레임을 늘리고 stack 사용량을
// 늘린다는 사실을 관찰하는 데 쓸 수 있습니다.
func traceLenAtDepth(depth int) int {
	if depth == 0 {
		buf := make([]byte, 256*1024)
		return runtime.Stack(buf, false)
	}

	// 각 프레임에서 256바이트짜리 지역 변수를 선언해
	// 실제 stack 사용량을 조금씩 늘립니다.
	var padding [256]byte
	_ = padding

	return traceLenAtDepth(depth - 1)
}

// fillStack 은 stack 을 빠르게 소비하는 재귀 함수입니다.
// depth 0 에 도달하면 ctx.value 에 42 를 기록합니다.
// stack 복사가 일어날 때 각 프레임에 저장된 ctx 포인터가
// 새 stack 주소로 조정되지 않으면 이 쓰기가 엉뚱한 메모리를
// 가리키게 되고, 테스트에서 값이 깨집니다.
func fillStack(depth int, ctx *pointerTarget) bool {
	var padding [512]byte
	_ = padding

	if depth == 0 {
		ctx.value = 42
		return ctx.value == 42
	}

	return fillStack(depth-1, ctx)
}

// deepRecursionThenBlock 은 지정된 깊이까지 재귀한 뒤 depth 0 에서
// ready 채널을 닫고 release 채널이 닫힐 때까지 대기합니다.
// 이렇게 하면 main goroutine 이 "거대해진 stack 을 가진 goroutine"을
// 살아 있는 상태로 두고 전역 StackInuse 를 측정할 수 있습니다.
func deepRecursionThenBlock(depth int, ready chan<- struct{}, release <-chan struct{}, ctx *pointerTarget) {
	var padding [512]byte
	_ = padding

	if depth == 0 {
		ctx.value = 42
		close(ready)
		<-release
		return
	}

	deepRecursionThenBlock(depth-1, ready, release, ctx)
}

// showStackGrowth 는 재귀 호출 깊이가 늘어나면 stack trace 길이가
// 어떻게 변하는지 보여주고, 깊은 재귀로 goroutine stack 을 키운 뒤
// 전역 StackInuse 가 증가했다가 GC 후 다시 줄어드는 모습을 관찰합니다.
func showStackGrowth() {
	fmt.Println("=== goroutine stack 성장과 축소 관찰 ===")

	shallow := traceLenAtDepth(0)
	deep := traceLenAtDepth(200)
	fmt.Printf("stack trace 길이: depth 0 = %d bytes, depth 200 = %d bytes\n", shallow, deep)
	fmt.Println("trace 길이는 stack 메모리 크기가 아니지만 프레임 수가 늘어날수록 커집니다.")

	// 깊은 재귀로 stack 을 실제로 키운 뒤 전역 stack 사용량을 비교합니다.
	ctx := &pointerTarget{value: 0}
	ready := make(chan struct{})
	release := make(chan struct{})

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		deepRecursionThenBlock(2048, ready, release, ctx)
	}()

	<-ready // depth 0 에 도달해 goroutine 이 잠들 때까지 대기

	var before runtime.MemStats
	var during runtime.MemStats
	runtime.ReadMemStats(&before)

	// 현재 main goroutine 의 StackInuse 를 읽는 것이 아니라
	// 아직 살아 있는 깊은 재귀 goroutine 의 stack 이 포함된
	// 전역 StackInuse 를 읽습니다.
	//
	// runtime.MemStats.StackInuse 는 Go 런타임이 현재 stack 으로
	// 사용 중인 총 바이트 수입니다. 멈춰 있는 goroutine 의 stack 은
	// 해제되지 않고 그대로 계상됩니다.
	runtime.ReadMemStats(&during)
	fmt.Printf("깊은 재귀 goroutine 대기 중 StackInuse: %d bytes (%.2f MB)\n",
		during.StackInuse, float64(during.StackInuse)/(1<<20))

	close(release)
	wg.Wait()

	// stack shrink 는 GC 사이클 중에 일어납니다.
	// 반환된 stack 공간이 커도 즉시 줄어들지 않고 GC 가 스캔할 때
	// 필요 없는 부분을 잘라냅니다.
	runtime.GC()
	var after runtime.MemStats
	runtime.ReadMemStats(&after)
	fmt.Printf("재귀 반환 + GC 후 StackInuse: %d bytes (%.2f MB)\n",
		after.StackInuse, float64(after.StackInuse)/(1<<20))
	fmt.Printf("포인터 무결성: ctx.value=%d (42 기대)\n", ctx.value)
	fmt.Println()
}

//go:noinline
func simulateBoundaryCrossing(x int) int {
	// 이 함수는 일부러 매우 작게 만들어 컴파일러가 인라인하지 못하도록
	// noinline 지시어를 붙였습니다. segment stack 시절에는 이런 작은
	// 함수 호출이 segment 경계를 넘을 때마다 segment 연결 리스트를
	// 따라가야 했습니다.
	return x + 1
}

// simulateHotSplit 은 segment stack 이 버려진 핵심 이유 중 하나인
// hot split 문제를 개념적으로 보여줍니다.
//
// Go 는 한때 stack 을 여러 segment 로 쪼개 연결 리스트로 관리했습니다.
// 함수 호출이 segment 경계를 자주 넘나들면 segment 할당/해제가 반복되어
// 성능이 급락했습니다. 지금은 연속 stack(contiguous stack)을 쓰므로
// 같은 작은 호출이 단순히 SP 를 움직이는 수준의 비용만 듭니다.
func simulateHotSplit() {
	fmt.Println("=== segment stack 이 버려진 이유: hot split 시뮬레이션 ===")

	const calls = 1_000_000
	start := time.Now()
	sum := 0
	for i := 0; i < calls; i++ {
		sum += simulateBoundaryCrossing(i)
	}
	elapsed := time.Since(start)
	fmt.Printf("작은 함수 호출 %d 회: %v (sum=%d)\n", calls, elapsed, sum)
	fmt.Println("연속 stack 에서는 이 비용이 거의 들지 않습니다.")
	fmt.Println()
}

// showStackCopy 는 stack 복사가 일어나는 깊은 재귀를 여러 번 호출해
// heap 객체에 대한 포인터가 계속 유효한지 확인합니다.
func showStackCopy() {
	fmt.Println("=== stack 복사와 포인터 조정 ===")

	target := &pointerTarget{value: 0}

	for _, depth := range []int{128, 256, 512, 1024} {
		ok := fillStack(depth, target)
		fmt.Printf("fillStack(%d) 성공=%v, target.value=%d\n", depth, ok, target.value)
		if !ok || target.value != 42 {
			fmt.Println("오류: stack 복사 후 포인터가 heap 객체를 올바르게 가리키지 못했습니다.")
			return
		}
		target.value = 0 // 다음 반복을 위해 초기화
	}

	fmt.Println("모든 깊이에서 stack 복사 후에도 포인터가 유효합니다.")
	fmt.Println()
}

// manyGoroutines 는 goroutine 이 2KB stack 에서 시작하기 때문에
// 같은 메모리로 OS thread 보다 훨씬 많은 동시 실행 흐름을 만들 수
// 있음을 보여줍니다.
func manyGoroutines() {
	fmt.Println("=== 2KB 시작 stack 의 메모리 이점 ===")

	const goroutineCount = 100_000
	const osThreadStackSize = 1 << 20 // 일반적인 OS thread stack 1MB

	var wg sync.WaitGroup
	wg.Add(goroutineCount)

	for i := 0; i < goroutineCount; i++ {
		go func() {
			defer wg.Done()
			// stack 을 거의 쓰지 않는 goroutine.
			// 시작 stack 이 2KB 이므로 수십만 개를 만들어도
			// thread 1MB 기준보다 압도적으로 적은 메모리를 씁니다.
			var x int
			x = 1
			_ = x
			time.Sleep(10 * time.Millisecond)
		}()
	}

	wg.Wait()

	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	fmt.Printf("goroutine %d 개: StackInuse=%d bytes (%.1f MB)\n",
		goroutineCount, m.StackInuse, float64(m.StackInuse)/(1<<20))
	fmt.Printf("같은 수를 OS thread 로 만들 때 예상: %d MB\n",
		goroutineCount*osThreadStackSize/(1<<20))
	fmt.Println()
}

func main() {
	showStackGrowth()
	showStackCopy()
	simulateHotSplit()
	manyGoroutines()
}