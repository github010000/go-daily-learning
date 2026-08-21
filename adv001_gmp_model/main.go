package main

import (
	"fmt"
	"runtime"
	"runtime/pprof"
	"sync"
	"sync/atomic"
	"time"
)

const localRunQueueSize = 256

// osThreadCount는 Go 런타임이 지금까지 만든 OS 스레드(M) 수를 근사해서 돌려준다.
// runtime/pprof의 threadcreate 프로파일은 스레드가 새로 생길 때마다 1씩 증가하므로
// 방향성을 보기에 충분하다. 정확한 현재 M 수는 런타임 내부에만 있지만 관찰 목적으로는
// 이 값이면 된다.
func osThreadCount() int {
	p := pprof.Lookup("threadcreate")
	if p == nil {
		return -1
	}
	return p.Count()
}

// cpuBurn은 관찰을 위해 CPU를 일정 시간 태우는 함수다.
// 단순 덧셈 반복문은 컴파일러가 없애려고 할 수 있으므로 조건문을 넣어
// 실제 계산이 일어나도록 강제한다.
func cpuBurn(n int64) {
	var sum int64
	for i := int64(0); i < n; i++ {
		sum += i
	}
	if sum < 0 {
		println(sum)
	}
}

func main() {
	fmt.Println("== GMP 모델 관찰 시작 ==")
	fmt.Printf("NumCPU: %d\n", runtime.NumCPU())
	fmt.Printf("초기 P(GOMAXPROCS): %d\n", runtime.GOMAXPROCS(0))
	fmt.Printf("초기 G(NumGoroutine): %d\n", runtime.NumGoroutine())
	fmt.Printf("초기 M(thread create profile count): %d\n", osThreadCount())

	procsBefore := runtime.GOMAXPROCS(0)

	// 지역 run queue와 글로벌 run queue의 관계를 보기 위해 P를 1개로 제한한다.
	// 이렇게 하면 main goroutine이 만드는 300개 G가 모두 같은 P에 쌓인다.
	runtime.GOMAXPROCS(1)
	fmt.Println("\n-- GOMAXPROCS=1에서 300개 goroutine 생성 순서 기록 --")

	const n = 300
	var counter int64
	startOrder := make([]int64, n)
	var wg sync.WaitGroup
	wg.Add(n)

	// 각 goroutine은 생성 순서 id를 받고, 실제 실행되는 순서를 atomic으로 기록한다.
	// GOMAXPROCS=1이므로 한 번에 하나씩만 실행된다.
	for i := 0; i < n; i++ {
		go func(id int) {
			defer wg.Done()
			seq := atomic.AddInt64(&counter, 1)
			atomic.StoreInt64(&startOrder[id], seq)
		}(i)
	}

	fmt.Printf("생성 직후 G count: %d\n", runtime.NumGoroutine())
	wg.Wait()
	fmt.Printf("완료 후 G count: %d\n", runtime.NumGoroutine())

	// 위에서 300개 G를 만들었을 때 런타임은 마지막에 만든 G를 runnext에 넣고,
	// 지역 runq가 256개로 가득 차면 절반을 global runq로 보낸다.
	// 따라서 실행 순서는 생성 순서와 다르다. 아래 숫자가 그 증거가 된다.
	fmt.Println("\n선택된 생성 index의 실행 순서:")
	selected := []int{299, 128, 255, 257, 0, 127, 256}
	for _, idx := range selected {
		fmt.Printf("생성 index %3d -> 실행 순서 %3d\n", idx, atomic.LoadInt64(&startOrder[idx]))
	}
	fmt.Println("runnext(LIFO), 지역 runq(256), global runq(절반 이동)의 효과가 드러난다.")

	// 이제 P를 2개로 늘려 병렬 실행이 실제로 M(OS thread)을 늘리는지 확인한다.
	// P는 실행 슬롯이고 M은 그 슬롯을 실제로 돌리는 OS 스레드다.
	if procsBefore >= 2 {
		runtime.GOMAXPROCS(2)
	} else {
		runtime.GOMAXPROCS(1)
	}
	fmt.Println("\n-- 병렬 CPU 버스트 실행 --")
	fmt.Printf("현재 P(GOMAXPROCS): %d\n", runtime.GOMAXPROCS(0))
	fmt.Printf("버스트 직전 M(thread create profile count): %d\n", osThreadCount())

	start := time.Now()
	var wg2 sync.WaitGroup
	wg2.Add(2)
	for j := 0; j < 2; j++ {
		go func() {
			defer wg2.Done()
			cpuBurn(300_000_000)
		}()
	}
	wg2.Wait()
	elapsed := time.Since(start)

	fmt.Printf("CPU 버스트 완료 시간: %v\n", elapsed)
	fmt.Printf("버스트 직후 M(thread create profile count): %d\n", osThreadCount())
	fmt.Println("M이 늘어났다면 P가 2개이므로 2개의 G를 병렬로 돌리기 위해 새 M을 만들었기 때문이다.")

	runtime.GOMAXPROCS(procsBefore)
	fmt.Printf("\nGOMAXPROCS 복원: %d\n", procsBefore)
}