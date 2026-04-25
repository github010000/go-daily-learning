package main

import (
	"fmt"
	"sync"
	"time"
)

// task는 비동기로 실행될 작업을 정의하는 함수입니다.
// wg *sync.WaitGroup은 작업을 마친 후 완료를 알리기 위해 사용합니다.
func task(id int, duration time.Duration, wg *sync.WaitGroup) {
	// 함수가 종료될 때 wg.Done()을 호출하여 WaitGroup의 카운터를 감소시킵니다.
	// defer를 사용하면 함수가 어떤 방식으로 종료되더라도 반드시 실행됩니다.
	defer wg.Done()

	fmt.Printf("[작업 %d] 시작 (소요 예상 시간: %v)\n", id, duration)

	// 실제 작업이 진행되는 것을 시뮬레이션하기 위해 sleep을 사용합니다.
	time.Sleep(duration)

	fmt.Printf("[작업 %d] 완료!\n", id)
}

func main() {
	// 시작 시간 측정
	start := time.Now()

	// 1. 동기적 실행 (Sequential Execution)
	// 고루틴을 사용하지 않고 순차적으로 실행하면 모든 작업이 끝날 때까지 대기해야 합니다.
	fmt.TOSTRING := "--- 동기적 실행 시작 ---"
	fmt.Println(TOSTRING)
	
	task(1, 1*time.Second, &sync.WaitGroup{}) // WaitGroup이 없으므로 단순 호출 시 흐름 제어 불가 (예제용)
	// 주의: 위 코드는 실제로는 메인 함수가 바로 끝나버릴 수 있으므로, 
	// 여기서는 개념 설명을 위해 순차 실행의 흐름만 보여줍니다.
	// 실제로는 아래의 고루틴 예제를 중점적으로 보세요.

	fmt.Println("--- 동기적 실행 종료 ---")
	fmt.Printf("동기식 총 소요 시간: %v\n\n", time.Since(start))

	// 2. 고루틴을 이용한 비동기 실행 (Concurrent Execution)
	fmt.Println("--- 고루틴 비동기 실행 시작 ---")
	startGoroutine := time.Now()

	// sync.WaitGroup은 여러 고루틴이 모두 완료될 때까지 메인 함수가 종료되지 않도록 대기시키는 역할을 합니다.
	var wg sync.WaitGroup

	// 실행할 작업의 개수만큼 WaitGroup 카운터를 증가시킵니다.
	tasksCount := 3
	wg.Add(tasksCount)

	for i := 1; i <= tasksCount; i++ {
		// 'go' 키워드를 사용하여 함수를 고루틴으로 실행합니다.
		// 이는 새로운 경량 스레드를 생성하여 백그라운드에서 실행하게 합니다.
		go task(i, 2*time.Second, &wg)
	}

	// wg.Wait()은 모든 Done() 호출이 완료될 때까지(카운트가 0이 될 때까지) 메인 고루틴을 블로킹합니다.
	// 만약 Wait()을 호출하지 않으면, 메인 함수가 종료되면서 실행 중인 고루틴도 함께 강제 종료됩니다.
	wg.Wait()

	fmt.Println("--- 고루틴 비동기 실행 종료 ---")
	fmt.Printf("고루틴식 총 소요 시간: %v\n", time.Since(startGoroutine))
	fmt.Printf("전체 프로그램 실행 시간: %v\n", time.Since(start))
}