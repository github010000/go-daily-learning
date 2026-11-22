package main

import (
	"fmt"
	"sync"
	"sync/atomic"
)

// runWorkers 는 n 개의 고루틴을 띄워 results[i] = i*i 를 채웁니다.
// 고루틴마다 서로 다른 인덱스에만 쓰기 때문에 락이 필요 없습니다.
// 같은 슬라이스에 append 하면 경쟁이 생기므로, 미리 크기를 정해 두고 인덱스로 씁니다.
func runWorkers(n int) []int {
	results := make([]int, n)
	var wg sync.WaitGroup
	for i := range n {
		// Add 는 go 문 "앞에서", 부르는 쪽(main 흐름)에서 호출합니다.
		wg.Add(1)
		go func() {
			// Done 은 defer 로 걸어 두어야 중간에 return 해도 빠뜨리지 않습니다.
			defer wg.Done()
			// Go 1.22 부터 for 의 i 는 반복마다 새 변수라서 그대로 캡처해도 안전합니다.
			results[i] = i * i
		}()
	}
	wg.Wait()
	return results
}

// sumWithMutex 는 n 개의 고루틴이 공유 카운터를 1000 번씩 올립니다.
// 공유 변수를 여러 고루틴이 건드리면 Mutex 로 보호해야 합니다.
func sumWithMutex(n int) int {
	var (
		wg  sync.WaitGroup
		mu  sync.Mutex
		sum int
	)
	for range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 1000 {
				mu.Lock()
				sum++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	return sum
}

// mistakeNoWait 는 「기다리지 않는」 실수를 재현합니다.
// 고루틴들을 gate 에서 막아 두므로, 기다리지 않고 센 값은 항상 0 입니다.
// 실제 코드에서는 "main 이 먼저 끝나 버려서 고루틴이 일을 못 하고 죽는" 상황과 같습니다.
// (시연이 끝나면 gate 를 열고 cleanup 으로 정리해서 고루틴이 새지 않게 합니다.)
func mistakeNoWait(n int) int {
	var finished atomic.Int32
	gate := make(chan struct{})
	var cleanup sync.WaitGroup
	for range n {
		cleanup.Add(1)
		go func() {
			defer cleanup.Done()
			<-gate
			finished.Add(1)
		}()
	}
	seen := int(finished.Load()) // Wait 없이 바로 확인
	close(gate)
	cleanup.Wait()
	return seen
}

// mistakeAddInside 는 wg.Add 를 고루틴 안에서 부르는 실수입니다.
// 고루틴이 Add 하기 전에 main 이 Wait 에 도달하면, 카운터가 0 이라 Wait 는 즉시 반환합니다.
// gate 로 "고루틴이 늦게 시작하는 상황"을 결정적으로 만들어 두었습니다.
func mistakeAddInside(n int) int {
	var wg, cleanup sync.WaitGroup
	var finished atomic.Int32
	gate := make(chan struct{})
	for range n {
		cleanup.Add(1)
		go func() {
			defer cleanup.Done()
			<-gate
			wg.Add(1) // 너무 늦다: main 의 Wait 는 이미 지나갔다
			defer wg.Done()
			finished.Add(1)
		}()
	}
	wg.Wait() // 카운터가 0 이므로 바로 통과
	seen := int(finished.Load())
	close(gate)
	cleanup.Wait()
	return seen
}

// fixedAddBefore 는 Add 를 go 문 앞에서 호출한 고친 버전입니다.
// Wait 가 반환된 시점에 n 개가 모두 끝났음이 보장됩니다.
func fixedAddBefore(n int) int {
	var wg sync.WaitGroup
	var finished atomic.Int32
	for range n {
		wg.Add(1) // 고루틴을 만들기 전에 카운터를 올린다
		go func() {
			defer wg.Done()
			finished.Add(1)
		}()
	}
	wg.Wait()
	return int(finished.Load())
}

func main() {
	fmt.Println("기본 사용: 제곱 결과 =", runWorkers(5))
	fmt.Println("Mutex 로 보호한 합계 (50 고루틴 x 1000) =", sumWithMutex(50))

	fmt.Println("실수 버전 (Wait 없음): 끝난 고루틴 수 =", mistakeNoWait(10))
	fmt.Println("실수 버전 (Add 를 고루틴 안에서): Wait 직후 끝난 수 =", mistakeAddInside(10))
	fmt.Println("고친 버전 (Add 를 go 앞에서): Wait 직후 끝난 수 =", fixedAddBefore(10))
}