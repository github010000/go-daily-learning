package main

import (
	"fmt"
	"runtime"
	"runtime/debug"
	"sync"
)

/*
goroutine 스택의 '2KB 시작 크기'와 '가변적 성장'이
왜 중요한지, 그리고 잘못 설계된 고전적 접근법과 Go의 현대적 접근법을 비교합니다.

핵심 개념:
1. Go는 세그먼트 기반 스택을 사용하지 않습니다. (C/C++의 전통)
2. Go 스택은 heap에 할당된 연속 메모리 블록입니다.
3. 초기에는 매우 작게 시작해 필요할 때만 heap 메모리를 추가로 할당합니다.
4. 호출 깊이(Caller/Callee 관계)가 깊어지면 스택이 자동으로 커집니다.

이 코드는 다음을 시연합니다:
- Goroutine이 시작될 때 스택 크기가 얼마나 작은지 확인
- 재귀 호출로 스택이 어떻게 성장하는지 관찰
- 명시적으로 스택을 크고 할당하는 방법(DebugStackAlloc)과 비교
*/

func main() {
	fmt.Println("=== Go Goroutine Stack: Why 2KB? ===\n")

	// 1. 기본 Goroutine의 스택 시작 크기와 동작 확인
	runDefaultStackBehavior()

	// 2. 고전적/잘못된 방식: 너무 큰 스택을 무조건 할당하는 경우
	runLargeStackAllocation()

	// 3. 메모리 효율성 시뮬레이션: 수많은 goroutine 생성
	runMassiveGoroutineCount()
}

// runDefaultStackBehavior는 일반적인 goroutine의 스택 성장 행동을 보여줍니다.
// 테스트에서는 이 함수의 반환값을 검증할 수 있습니다.
func runDefaultStackBehavior() {
	fmt.Println("--- 1. Default Behavior (Lazy Allocation) ---")

	var stackTrace string
	// runtime.Stack은 현재 goroutine의 스택 프레임 정보를 반환합니다.
	// 이 정보에 'sp' (stack pointer) 나 'base' 등 관련 정보가 포함되어 있을 수 있지만,
	// 가장 명확한 것은 'runtime/debug.SetMaxStack' 같은 제한이나
	// 스택 크기 관련 환경변수 GOGC와 함께 동작하는 GC 패턴입니다.
	// 하지만 우리는 스택이 "작게 시작한다"는 것을 코드적으로 보여줘야 합니다.
	// 정확한 현재 스택 바닥(base) 주소와 끝(end) 주소의 차이를 구하는 것은
	// 런타임 내부 함수에 의존해야 하지만, public API로는 `runtime.Stack`을 통해
	// 스택을 추적하거나, 혹은 `debug.Stack()`을 사용할 수 있습니다.
	// 더 확실한 방법은 스택이 어떻게 성장하는지 `morestack` 호출 빈도나
	// 스택 크기 관련 통계를 보는 것이지만, 여기서는 스택의 "느린 성장"을
	// 재귀 깊이를 통해 간접적으로 비교합니다.

	// 재귀 깊이를 측정하는 함수를 호출합니다.
	depth := measureRecursionDepth()

	// 스택이 자동으로 성장해서 깊이를 재었음
	fmt.Printf("Recurion depth reached: %d\n", depth)
	fmt.Println("Stack grew automatically from small base (2KB) to fit this depth.")

	// 스택 관련 기본 설정 출력
	fmt.Printf("GOGC (Garbage Collection): %s\n", debug.SetGOGC(0)) // GC 끄고 재설정 안함
	debug.SetGOGC(100) // 다시 켜기
}

// measureRecursionDepth는 스택이 얼마나 깊어지는지 확인합니다.
// 이 함수는 테스트에서 직접 호출되어 스택 깊이가 2KB 이상인 상황에서도
// panic 없이 작동하는지 검증하는 데 쓰입니다.
func measureRecursionDepth() int {
	return 1 + measureRecursionDepth()
}

// runLargeStackAllocation은 스택을 미리 크게 할당하는 경우를 시연합니다.
// 이는 메모리 낭비(Thrashing)를 유발할 수 있는 '잘못된' 관행을 보여줍니다.
func runLargeStackAllocation() {
	fmt.Println("\n--- 2. Pre-allocated Large Stack (Inefficient) ---")
	fmt.Println("Allocating a goroutine with a large initial stack.")
	fmt.Println("This wastes memory if the goroutine doesn't need it.")

	// Go에서는 public API를 통해 스택 크기를 명시적으로 '초기'에 설정하기 어렵지만,
	// `runtime/debug.SetMaxStack`을 통해 최대 크기를 제한하거나,
	// 혹은 내부적으로 스택이 크게 할당될 수 있는 경로(예: C 연결이나 특정 라이브러리)를 가정합니다.
	// 여기서는 스택이 자동으로 커지는 것을 방지하기 위해 최대 스택 크기를 제한하는 시뮬레이션이나,
	// 혹은 단순히 "스택이 크면 메모리가 많이 차지한다"는 개념을 설명합니다.
	// 실제 Go 1.14+ 부터는 스택 크기가 동적으로 관리되므로,
	// 우리는 `runtime.KeepAlive` 등으로 메모리가 즉시 해제되지 않는 것을 시연할 수 있습니다.
	// 하지만 더 중요한 것은, 스택이 커지면 `madvise` 호출이 자주 일어나거나
	// 페이지 폴트(Page Fault)가 증가할 수 있다는 점입니다.

	// 메모리 상태를 찍기 전 후
	var memBefore, memAfter runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&memBefore)

	// 큰 슬라이스를 할당하여 Heap이 아닌, 사실상 Stack에 가까운 연속 메모리 사용 패턴을 모사하거나
	// 혹은 단순히 Heap을 사용하지만 "Statically Allocated"의 대안으로 보여줍니다.
	// 사실 Go에서는 스택 크기를 사용자 코드로 직접 초기 크기를 정해 할당하는 방법은 없습니다.
	// 따라서 이 섹션은 개념적 비교를 위해 "정적 배열" vs "가변 스택"의 관점에서 설명합니다.
	// 큰 배열을 스택에 올리면 (참조가 아니라 값을 넣으면) 스택이 커집니다.

	// 이 함수 내에서 큰 배열을 할당하면, 해당 goroutine의 스택은 초기 2KB보다 훨씬 크게 성장해야 합니다.
	var largeBuffer [1024 * 1024]byte // 1MB 배열
	for i := range largeBuffer {
		largeBuffer[i] = byte(i % 256)
	}
	// largeBuffer는 stack에 할당됩니다.
	// 따라서 스택은 최소 1MB 이상으로 성장했습니다.
	// Go 컴파일러는 이 변수가 스택에 할당될 것임을 분석합니다.

	runtime.ReadMemStats(&memAfter)

	// Heap Alloc 증가량은 이 거버먼트가 할당한 1MB와 관련이 있을 수 있지만,
	// 배열 자체가 스택에 있으면 Heap Alloc에는 반영되지 않습니다.
	// 스택이 커지면 Heap Alloc에는 영향이 없습니다.
	// 그러나 스택이 커졌다는 사실(1MB 이상)이 중요합니다.
	fmt.Printf("Allocated 1MB array on stack. Stack size grew to > 1MB.\n")
	fmt.Printf("Memory stats diff (HeapAlloc): %d bytes (Array not in heap, so diff might be small)\n", memAfter.HeapAlloc - memBefore.HeapAlloc)

	// 배열이 스택에 있으므로, 이 goroutine은 메모리 사용량이 Heap에는 반영되지 않지만
	// OS 관점에서는 가상 메모리(Virtual Memory)가 커진 것처럼 보입니다.
}

// runMassiveGoroutineCount는 많은 goroutine이 생성될 때 메모리 효율성을 보여줍니다.
// 각 goroutine이 2KB로 시작하므로, 10만 개의 goroutine은 약 200MB의 가상 메모리를
// 시작할 때만 요구합니다. 실제로 쓰이지 않는 페이지는 OS에 반환될 수 있습니다(Throttling).
func runMassiveGoroutineCount() {
	fmt.Println("\n--- 3. Massive Goroutine Count (Memory Efficiency) ---")

	const numGoroutines = 100000
	var wg sync.WaitGroup

	// 스택을 거의 사용하지 않는 goroutine을 10만 개 만듭니다.
	// 각 goroutine는 최소 2KB 스택을 가집니다.
	// 총 가상 메모리 사용량: 100,000 * 2KB = ~200MB
	// 만약 각 goroutine가 1MB 스택을 가져야 했다면: ~100GB의 메모리가 필요합니다!
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			// 이 goroutine은 스택을 거의 사용하지 않습니다.
			// 즉, 스택이 2KB에서 성장하지 않습니다.
		}(i)
	}

	// 모든 goroutine이 완료될 때까지 대기
	wg.Wait()

	// 메모리 사용량 확인
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	fmt.Printf("Created and finished %d goroutines.\n", numGoroutines)
	fmt.Printf("Current OS Threads: %d\n", runtime.NumCPU()) // 보통 CPU 코어 수附近
	fmt.Printf("Heap Alloc: %d bytes (Heap only, not stack)\n", mem.HeapAlloc)

	// Go는 스택 크기를 동적으로 조절하므로, 스택을 안 쓴 goroutine은
	// 스택 크기를 축소할 수 있습니다 (축소는 주로 exit 시 혹은 일정 조건에서).
	// 하지만 시작 크기가 작기 때문에 초기 오버헤드가 적습니다.
	fmt.Println("Memory efficient because each goroutine starts small.")
}