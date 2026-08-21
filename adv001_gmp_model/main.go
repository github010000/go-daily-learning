package main

import (
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

const (
	// cpuLoad 는 CPU-bound 작업 반복 횟수다.
	// main 시연에서는 GOMAXPROCS 변화를 볼 수 있을 만큼 크게 잡고,
	// 테스트에서는 시간이 오래 걸리지 않도록 더 작은 값을 직접 넘긴다.
	cpuLoad = 1 << 24

	// demoWorkers 는 시연용 goroutine 개수다. P보다 많게 두어
	// 로컬 런큐에 실행을 기다리는 G가 생기는 상황을 만든다.
	demoWorkers = 16
)

// cpuBound 는 CPU 연산만으로 시간을 소모해 GOMAXPROCS 변화를 관찰하기 위한 함수다.
// 반환값을 호출부에서 사용하도록 해 컴파일러가 루프를 제거하지 못하게 한다.
// 같은 입력에는 항상 같은 결과를 내므로 테스트에서 결정적으로 검증할 수 있다.
func cpuBound(workerID, iterations int) uint64 {
	var sum uint64
	for i := 0; i < iterations; i++ {
		sum += uint64(i)
	}
	// workerID를 더해 각 goroutine이 자기 몫을 계산했는지 테스트에서 확인 가능하게 한다.
	return sum + uint64(workerID)
}

// runParallel 은 workers 개수만큼 goroutine을 생성해 CPU-bound 작업을 동시에 수행한다.
// 각 goroutine은 results 슬라이스의 서로 다른 인덱스에만 쓴다.
// 따라서 -race 로 돌려도 데이터 경쟁이 없다.
func runParallel(workers, iterations int) []uint64 {
	results := make([]uint64, workers)
	var wg sync.WaitGroup
	wg.Add(workers)

	for wid := 0; wid < workers; wid++ {
		go func(id int) {
			defer wg.Done()
			// id는 고유하므로 results[id]는 이 goroutine만 쓴다.
			results[id] = cpuBound(id, iterations)
		}(wid)
	}

	wg.Wait()
	return results
}

// measureParallelism 은 GOMAXPROCS 를 p 로 바꾼 뒤 CPU-bound 작업 시간을 측정한다.
// 측정이 끝나면 원래 GOMAXPROCS 로 되돌려 다른 시연 순서에 영향을 주지 않는다.
// P 개수가 GMP 모델에서 동시에 실행될 수 있는 M 개수를 제한하는 모습을 보여준다.
func measureParallelism(workers, iterations, p int) ([]uint64, time.Duration, int) {
	old := runtime.GOMAXPROCS(p)
	start := time.Now()
	results := runParallel(workers, iterations)
	elapsed := time.Since(start)
	runtime.GOMAXPROCS(old)
	return results, elapsed, p
}

// printSchedulerStatus 는 G/M/P 모델 중 코드에서 직접 관찰 가능한 지표를 출력한다.
// NumCPU는 물리/논리 코어 수, GOMAXPROCS는 P 개수, NumGoroutine은 현재 G 개수다.
func printSchedulerStatus() {
	fmt.Printf("NumCPU=%d\n", runtime.NumCPU())
	fmt.Printf("GOMAXPROCS=%d\n", runtime.GOMAXPROCS(0))
	fmt.Printf("NumGoroutine=%d\n", runtime.NumGoroutine())
}

// demonstrateCurrentP 는 실행 시점의 GOMAXPROCS 를 바꾸지 않고 그대로 사용한다.
// GOMAXPROCS=1 go run . 과 GOMAXPROCS=4 go run . 을 비교할 때
// 이 첫 시연의 elapsed 차이가 가장 직접적인 증거가 된다.
func demonstrateCurrentP() {
	start := time.Now()
	results := runParallel(demoWorkers, cpuLoad)
	elapsed := time.Since(start)
	fmt.Printf("현재 GOMAXPROCS=%d  workers=%d  elapsed=%v  firstResult=%d\n",
		runtime.GOMAXPROCS(0), demoWorkers, elapsed, results[0])
}

// demonstrateScaling 은 같은 개수의 goroutine이라도 P 개수에 따라 CPU-bound 작업
// 완료 시간이 어떻게 달라지는지 보여준다.
// P 가 1 이면 모든 goroutine이 한 OS 스레드에서 순차 실행된다.
// P 를 2, 4, NumCPU 로 늘리면 동시에 실행될 수 있는 M 이 늘어나 elapsed 가 줄어든다.
func demonstrateScaling() {
	fmt.Println("--- 같은 goroutine 16개, P 개수에 따른 CPU-bound 처리 시간 ---")
	for _, p := range []int{1, 2, 4, runtime.NumCPU()} {
		// 물리 CPU 개수보다 많은 P는 비교 의미가 없으므로 건너뛴다.
		if p > runtime.NumCPU() {
			continue
		}
		results, elapsed, usedP := measureParallelism(demoWorkers, cpuLoad, p)
		// 결과가 0이 아니어야 실제 계산이 수행된 것이다.
		// firstResult를 출력해 컴파일러가 루프를 최적화로 없애지 않았음을 함께 보여준다.
		fmt.Printf("GOMAXPROCS=%2d  workers=%d  elapsed=%8v  firstResult=%d\n",
			usedP, demoWorkers, elapsed, results[0])
	}
}

// cooperativeYieldWorkers 는 runtime.Gosched 를 호출하며 1000회씩 카운터를 증가시킨다.
// Gosched 는 현재 G 가 자발적으로 P 를 내놓아 로컬 런큐의 다른 G 가 실행되게 한다.
// 카운터는 atomic으로 증가시키므로 -race 에서도 안전하다.
func cooperativeYieldWorkers(workers int) int {
	var counter atomic.Int32
	var wg sync.WaitGroup
	wg.Add(workers)

	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < 1000; j++ {
				// 현재 G를 P의 로컬 런큐에 양보하고 스케줄러를 다시 호출한다.
				runtime.Gosched()
				counter.Add(1)
			}
		}()
	}

	wg.Wait()
	return int(counter.Load())
}

func main() {
	printSchedulerStatus()
	fmt.Println()

	fmt.Println("--- 현재 GOMAXPROCS를 그대로 사용하는 시연 ---")
	demonstrateCurrentP()

	fmt.Println()
	demonstrateScaling()

	fmt.Printf("\nGosched 로 협조적으로 양보한 횟수: %d\n", cooperativeYieldWorkers(4))

	// GODEBUG=schedtrace=1000 go run . 처럼 실행하면 M/P/G 상태가 주기적으로 출력된다.
	// 이 코드에서는 그 출력을 직접 만들지 않고 위 명령으로 확인하도록 README 에 안내한다.
}