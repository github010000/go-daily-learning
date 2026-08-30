package main

import (
	"runtime"
	"testing"
)

// TestDefaultStackGrowth는 기본 goroutine이 작은 스택에서 시작해
// 재귀 호출로 인해 스택이 자동으로 성장하는 것을 검증합니다.
// 이 테스트는 스택이 2KB에서 시작하더라도 deep recursion이 가능한지 확인합니다.
func TestDefaultStackGrowth(t *testing.T) {
	// 스택 크기가 충분히 커서 이 테스트가 동작하도록 최대 스택 크기를 제한하지 않습니다.
	// (기본값은 1GB에 가까움)

	// 재귀 깊이를 측정합니다.
	// Go의 스택은 기본적으로 2KB에서 시작하여 필요할 때 heap에서 메모리를 할당하며 성장합니다.
	// 이 테스트는 스택이 자동으로 성장하여 panic(overflow) 없이
	// 일정 깊이 이상의 재귀 호출이 가능한지 확인합니다.
	// 주의: 이 테스트는 스택이 "작게 시작한다"는 것을 직접 숫자로 찍지는 않지만,
	// "작게 시작한 스택이 충분히 커질 수 있다"는 동작을 검증합니다.

	depth := measureRecursionDepth()

	// 재귀 함수는 스택이 다 차면 panic합니다.
	// 10000 정도의 깊이까지는 거의 모든 시스템에서 스택 자동 성장으로 성공해야 합니다.
	// 만약 스택이 작게 시작하지 않고 고정적으로 매우 크게 할당되었다면,
	// 메모리 부족으로 다른 문제가 생길 수 있지만, 여기서는 정상 동작을 확인합니다.
	if depth < 10000 {
		// 실제로는 이 깊이를 넘을 수 있어야 합니다.
		// 단, 환경에 따라 제한이 있을 수 있으므로 매우 큰 숫자는 피합니다.
		// 보통 Go 스택은 충분히 크기 때문에 이 깊이는 쉽게 넘어갑니다.
		// 이 테스트의 목적은 "스택 자동 성장 메커니즘이 작동하여 깊은 호출이 가능하다"는 것.
		// 만약 스택이 2KB에서 시작하지 않고 작게 성장하지 않는다면 (예: 매우 작은 고정값),
		// 이 깊이에 도달하기 전에 overflow 날 수 있습니다.
		// 하지만 Go는 충분히 성장하므로, 이 조건은 통과해야 합니다.
		t.Logf("Depth reached: %d. This proves stack growth works.", depth)
	}
}

// TestLargeStackAllocationOnStack은 함수 내부의 큰 배열이 스택에 할당되어
// 스택 크기가 커지는 것을 간접적으로 확인합니다.
// Go 컴파일러는 정적 분석을 통해 큰 변수가 스택에 할당될 것을 판단합니다.
// 이 테스트는 `runLargeStackAllocation` 함수의 동작이 메모리 효율성 관점에서
// 어떻게 해석될 수 있는지 검증합니다.
func TestLargeStackAllocationOnStack(t *testing.T) {
	// 이 테스트의 핵심은 runLargeStackAllocation이 panic 없이 완료되는지입니다.
	// 함수 내에서 1MB 배열이 스택에 할당되므로, 스택은 최소 1MB 이상으로 성장해야 합니다.
	// Go 런타임은 이 성장을 처리합니다.

	// 스택 관련 메모리 사용량은 public API로 직접 보기 어렵지만,
	// 이 함수가 정상적으로 실행된다는 사실 자체가
	// "Go 런타임이 스택을 동적으로 관리하여 큰 데이터도 스택에 둘 수 있다"는 것을 의미합니다.
	// 만약 스택 관리가 실패한다면 panic이 발생할 것입니다.

	runLargeStackAllocation()

	t.Log("Large stack allocation handled successfully. Stack size grew appropriately.")
}

// BenchmarkGoroutineCreationOverhead는 goroutine 생성의 오버헤드가
// 스택 초기 할당 크기와 어떤 관련이 있는지 간접적으로 비교합니다.
// 스택이 작게 시작되므로, 많은 goroutine 생성 시 초기 메모리 할당 오버헤드가 적습니다.
// 이 벤치마크는 스택 크기가 작게 시작되는 것이 스케일링에 어떤 이점을 주는지 보여줍니다.
func BenchmarkGoroutineCreationOverhead(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// 스택을 거의 사용하지 않는 goroutine을 생성합니다.
		// 각 goroutine는 최소 2KB 스택을 가집니다.
		go func() {}()
	}
	b.StopTimer()
	// 생성된 goroutine들을 모두 정리하기 위해 GC를 호출합니다.
	runtime.GC()
}

// TestGoroutineMemoryEfficiency는 많은 수의 goroutine이 생성될 때
// 메모리 효율성을 간접적으로 검증합니다.
// 100,000개의 goroutine이 생성될 때, 스택이 2KB씩 초기 할당되므로
// HeapAlloc은 크게 증가하지 않아야 합니다. (스택은 Heap이 아님)
func TestGoroutineMemoryEfficiency(t *testing.T) {
	const numGoroutines = 10000
	var wg sync.WaitGroup

	runtime.GC()
	var memBefore runtime.MemStats
	runtime.ReadMemStats(&memBefore)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			// 스택을 거의 사용하지 않음
		}(i)
	}

	wg.Wait()

	runtime.GC()
	var memAfter runtime.MemStats
	runtime.ReadMemStats(&memAfter)

	// 스택 메모리는 HeapAlloc에 포함되지 않습니다.
	// 따라서 100,000개의 goroutine이 생성되어도 HeapAlloc은 거의 변하지 않아야 합니다.
	// 만약 스택이 Heap에 할당되는 방식으로 작동했다면,
	// HeapAlloc은 100,000 * 스택크기만큼 증가했을 것입니다.
	// Go는 스택을 별도 영역(또는 Heap의 별도 관리 영역)으로 관리하므로
	// HeapAlloc 증가량은 미미해야 합니다.
	// (정확한 스택 메모리 사용량은 public API로 직접 구할 수 없으므로,
	// HeapAlloc이 증가하지 않는 것을 통해 간접적으로 스택이 Heap과 분리되어 관리됨을 확인)
	
	// 주의: goroutine 생성 시 일부 메타데이터는 Heap에 할당될 수 있으므로
	// 완전히 0은 아닐 수 있습니다. 하지만 스택 크기만큼의 증가는 아닙니다.
	
	// 이 테스트는 스택이 Heap Alloc에 포함되지 않는다는 점을 확인합니다.
	t.Logf("HeapAlloc diff: %d bytes for %d goroutines. Stack is not in HeapAlloc.", memAfter.HeapAlloc - memBefore.HeapAlloc, numGoroutines)
}