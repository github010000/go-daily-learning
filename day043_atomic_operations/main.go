package main

import (
	"fmt"
	"sync"
	"sync/atomic"
)

// 전역 변수로 카운터 선언
// 여러 고루틴에서 동시에 접근할 수 있는 변수입니다.
var counter int64

// worker 함수: 고루틴으로 실행되며 카운터를 n번 증가시킵니다.
func worker(id int, wg *sync.WaitGroup, n int) {
	defer wg.Done()

	for i := 0; i < n; i++ {
		// atomic.AddInt64: 원자적으로 값을 추가합니다.
		// Mutex 사용 없이 안전하게 += 1 연산을 수행합니다.
		atomic.AddInt64(&counter, 1)
	}
}

func main() {
	// === 1. Atomic Add 및 Load 테스트 ===
	fmt.Println("=== 1. Atomic Add 및 Load 테스트 ===")
	var wg sync.WaitGroup

	workers := 5
	iterations := 1000

	// 5개의 고루틴을 동시에 실행하여 counter를 증가시킵니다.
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go worker(i, &wg, iterations)
	}

	// 모든 고루틴이 작업을 끝낼 때까지 대기합니다.
	wg.Wait()

	// atomic.LoadInt64: 현재 저장된 값을 안전합니다.
	// Mutex 없이 데이터 레이스(Race Condition) 없이 값을 읽어옵니다.
	currentVal := atomic.LoadInt64(&counter)
	fmt.Printf("최종 카운터 값: %d (기대값: %d)\n", currentVal, int64(workers*iterations))

	// === 2. Atomic Store 테스트 ===
	fmt.Println("\n=== 2. Atomic Store 테스트 ===")
	// atomic.StoreInt64: 값을 덮어씁니다.
	atomic.StoreInt64(&counter, 0)
	fmt.Printf("Store 후 값: %d\n", atomic.LoadInt64(&counter))

	// === 3. Atomic CompareAndSwap (CAS) 테스트 ===
	fmt.Println("\n=== 3. Atomic CompareAndSwap (CAS) 테스트 ===")
	// CAS: "현재 값이 expected와 같다면 newValue로 바꾼다"
	// 락(lock) 없이 특정 조건이 충족될 때만 변경하고 싶을 때 유용합니다.

	// 현재 counter는 0입니다. 0 -> 100으로 변경 시도 (성공해야 함)
	oldVal := atomic.LoadInt64(&counter)
	success := atomic.CompareAndSwapInt64(&counter, oldVal, 100)
	fmt.Printf("CAS (0 -> 100) 성공 여부: %v, 현재 값: %d\n", success, atomic.LoadInt64(&counter))

	// counter는 이제 100입니다. 50 -> 200으로 변경 시도 (실패해야 함, 값이 50이 아님)
	success2 := atomic.CompareAndSwapInt64(&counter, 50, 200)
	fmt.Printf("CAS (50 -> 200) 성공 여부: %v, 현재 값: %d\n", success2, atomic.LoadInt64(&counter))
}