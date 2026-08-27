package main

import (
	"fmt"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

const (
	defaultSyscallDuration = 100 * time.Millisecond
	defaultSyscallWorkers  = 4
	defaultCPUWorkers      = 2
	defaultCPUIterations   = 1000000
)

// blockingSyscallWorker는 지정한 duration 동안 블로킹하는 select(2) syscall을
// 반복 호출한다. 이 syscall은 진짜 커널 블로킹을 일으키므로, Go 런타임은
// M이 P를 반납하도록 처리한다. syscallCount는 실제 호출 횟수를 누적한다.
func blockingSyscallWorker(id int, duration time.Duration, iterations int, wg *sync.WaitGroup, syscallCount *int64) {
	defer wg.Done()
	tv := syscall.NsecToTimeval(duration.Nanoseconds())
	for i := 0; i < iterations; i++ {
		// select(2)는 파일 디스크립터를 감시하지 않고 그냥 대기하는 용도로 쓰인다.
		// nfds=0 이면 커널은 주어진 시간 동안 현재 스레드를 잠든다.
		_, err := syscall.Select(0, nil, nil, nil, &tv)
		if err != nil {
			fmt.Fprintf(os.Stderr, "worker %d: select error: %v\n", id, err)
			return
		}
		atomic.AddInt64(syscallCount, 1)
	}
}

// cpuBoundWorker는 CPU를 태우는 계산을 반복하며, 진행 상황을 progress에,
// 최종 계산 결과(검증용)를 result에 누적한다.
// progress는 대략적인 진행 척도로, sysmon이 P를 회수해야만 CPU 고루틴이
// 실제로 실행되어 progress가 증가한다.
func cpuBoundWorker(id int, iterations int, wg *sync.WaitGroup, progress *int64, result *int64) {
	defer wg.Done()
	var local int64
	for i := 0; i < iterations; i++ {
		local += int64(i) * 2
		if i%1000 == 0 {
			atomic.AddInt64(progress, 1000)
		}
	}
	atomic.AddInt64(progress, int64(iterations%1000))
	atomic.AddInt64(result, local)
}

func runScenario() {
	// syscall 고루틴과 CPU 고루틴이 경쟁하도록 GOMAXPROCS를 CPU 워커 수와 같게 제한한다.
	runtime.GOMAXPROCS(defaultCPUWorkers)

	fmt.Printf("GOMAXPROCS=%d\n", runtime.GOMAXPROCS(0))
	fmt.Printf("CPU workers: %d, syscall workers: %d\n", defaultCPUWorkers, defaultSyscallWorkers)
	fmt.Printf("syscall duration: %v\n", defaultSyscallDuration)
	fmt.Println("Run with GODEBUG=schedtrace=1000 to see P handoff in detail")

	var wg sync.WaitGroup
	var syscallCount int64
	var cpuProgress int64
	var cpuResult int64

	start := time.Now()

	// 블로킹 syscall 고루틴들을 먼저 시작한다.
	for i := 0; i < defaultSyscallWorkers; i++ {
		wg.Add(1)
		go blockingSyscallWorker(i, defaultSyscallDuration, 5, &wg, &syscallCount)
	}

	// CPU 바운드 고루틴들을 동시에 실행한다.
	for i := 0; i < defaultCPUWorkers; i++ {
		wg.Add(1)
		go cpuBoundWorker(i, defaultCPUIterations, &wg, &cpuProgress, &cpuResult)
	}

	wg.Wait()
	elapsed := time.Since(start)

	fmt.Printf("Elapsed: %v\n", elapsed)
	fmt.Printf("Syscalls completed: %d\n", atomic.LoadInt64(&syscallCount))
	fmt.Printf("CPU progress: %d\n", atomic.LoadInt64(&cpuProgress))
	fmt.Printf("CPU result sum: %d\n", atomic.LoadInt64(&cpuResult))
}

func main() {
	runScenario()
}