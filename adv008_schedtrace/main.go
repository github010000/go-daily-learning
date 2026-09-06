package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"regexp"
	"runtime"
	"runtime/trace"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// SchedStats holds the fields printed by GODEBUG=schedtrace=...
// 이 구조체는 schedtrace 한 줄을 사람이 읽을 수 있는 형태로 바꿔 담는다.
type SchedStats struct {
	Gomaxprocs      int
	Idleprocs       int
	Threads         int
	Spinningthreads int
	Needspinning    int
	Idlethreads     int
	Runqueue        int
	PQueues         []int
}

// schedTraceLineRe 는 runtime 이 출력하는 기본 schedtrace 라인 형식과 정확히 맞아야 한다.
// 예: SCHED 1000ms: gomaxprocs=8 idleprocs=4 threads=7 spinningthreads=0 needspinning=1 idlethreads=3 runqueue=2 [0 1 0 0 0 0 0 0]
var schedTraceLineRe = regexp.MustCompile(
	`^SCHED\s+(\d+)ms:\s+gomaxprocs=(\d+)\s+idleprocs=(\d+)\s+threads=(\d+)\s+spinningthreads=(\d+)\s+needspinning=(\d+)\s+idlethreads=(\d+)\s+runqueue=(\d+)\s+\[(.*)\]$`,
)

// parseSchedtraceLine 은 schedtrace 출력 한 줄을 SchedStats 로 바꾼다.
// 테스트에서 실제 형식과 필드 이름을 검증하기 위해 별도 함수로 분리했다.
func parseSchedtraceLine(line string) (SchedStats, error) {
	m := schedTraceLineRe.FindStringSubmatch(strings.TrimSpace(line))
	if m == nil {
		return SchedStats{}, fmt.Errorf("not a schedtrace line: %q", line)
	}

	stats := SchedStats{}
	var err error

	toInt := func(s string) int {
		if err != nil {
			return 0
		}
		var n int
		n, err = strconv.Atoi(s)
		return n
	}

	stats.Gomaxprocs = toInt(m[2])
	stats.Idleprocs = toInt(m[3])
	stats.Threads = toInt(m[4])
	stats.Spinningthreads = toInt(m[5])
	stats.Needspinning = toInt(m[6])
	stats.Idlethreads = toInt(m[7])
	stats.Runqueue = toInt(m[8])
	if err != nil {
		return SchedStats{}, fmt.Errorf("parse int field: %w", err)
	}

	pq := strings.TrimSpace(m[9])
	if pq != "" {
		for _, s := range strings.Fields(pq) {
			n, convErr := strconv.Atoi(s)
			if convErr != nil {
				return SchedStats{}, fmt.Errorf("parse per-P queue: %w", convErr)
			}
			stats.PQueues = append(stats.PQueues, n)
		}
	}
	return stats, nil
}

// CPUStats 는 runCPUWorkers 가 만든 goroutine 수와 실행한 반복 횟수만 담는다.
// 관찰용이 아니라 테스트에서 결과를 검증하기 위한 최소한의 통계다.
type CPUStats struct {
	Started    int
	Iterations int64
}

// runCPUWorkers 는 ctx 가 취소될 때까지 CPU-bound goroutine 을 돌린다.
// 의도적으로 workers 를 GOMAXPROCS 보다 크게 주면 run queue 에 대기 중인
// goroutine 이 생겨 schedtrace 의 runqueue, idleprocs 변화를 관찰할 수 있다.
func runCPUWorkers(ctx context.Context, workers int) CPUStats {
	if workers < 1 {
		return CPUStats{}
	}

	var iterations atomic.Int64
	var wg sync.WaitGroup
	stats := CPUStats{Started: workers}

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			local := 0
			for {
				select {
				case <-ctx.Done():
					iterations.Add(int64(local))
					return
				default:
				}
				// CPU-bound work: 특별한 I/O 없이 goroutine 이 P 를 계속 사용하게 만든다.
				// select 의 default 는 ctx.Done 이 준비되지 않았으면 즉시 통과한다.
				local++
				if local%10000 == 0 {
					// 메모리 베리어 역할도 하지만, 여기서는 반복 횟수 기록용 임시 지점이다.
					_ = local
				}
			}
		}()
	}

	<-ctx.Done()
	wg.Wait()
	stats.Iterations = iterations.Load()
	return stats
}

// runChannelBurst 는 n 개 goroutine 이 하나의 channel 에서 동시에 멈췄다가
// close 로 한꺼번에 풀리는 블로킹/언블로킹 폭주 상황을 만든다.
// execution tracer 에서 waiting -> runnable -> running 상태 전이가 뚜렷하게 보인다.
func runChannelBurst(n int) int {
	if n < 1 {
		return 0
	}

	start := make(chan struct{})
	done := make(chan struct{}, n)
	var wg sync.WaitGroup

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			done <- struct{}{}
		}()
	}

	close(start) // 모든 대기 goroutine 을 한 번에 runnable 상태로 만든다.
	for i := 0; i < n; i++ {
		<-done
	}
	wg.Wait()
	return n
}

// SchedulerDemoStats 는 runSchedulerDemo 의 결과를 담는다.
type SchedulerDemoStats struct {
	Started   int
	Completed int
}

// cpuTask 는 CPU-bound 샘플 작업이다. 테스트에서는 시간이 아니라 완료 개수로 검증한다.
func cpuTask(n int) int {
	sum := n
	for i := 0; i < 1000; i++ {
		sum = sum*31 + i
	}
	return sum
}

// runSchedulerDemo 는 workers 수만큼 goroutine 이 channel 로 tasks 를 받아
// CPU 작업을 수행하는 전형적인 worker pool 이다.
// context 취소를 지원하지만 테스트에서는 Background context 로 모든 task 완료를 확인한다.
func runSchedulerDemo(ctx context.Context, workers, tasks int) SchedulerDemoStats {
	tasksCh := make(chan int)
	var completed atomic.Int64
	var wg sync.WaitGroup

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case n, ok := <-tasksCh:
					if !ok {
						return
					}
					_ = cpuTask(n)
					completed.Add(1)
				}
			}
		}()
	}

	go func() {
		for i := 0; i < tasks; i++ {
			select {
			case tasksCh <- i:
			case <-ctx.Done():
				close(tasksCh)
				return
			}
		}
		close(tasksCh)
	}()

	wg.Wait()
	return SchedulerDemoStats{Started: workers, Completed: int(completed.Load())}
}

// runTraceDemo 는 runtime/trace 로 짧은 goroutine 상태 전이를 수집해 w 로 쓴다.
// trace.Start 는 반드시 Stop 과 짝을 이뤄야 하며, Stop 이후에 파일 뒤에 기록이 끝난다.
func runTraceDemo(w io.Writer) error {
	if err := trace.Start(w); err != nil {
		return fmt.Errorf("trace start: %w", err)
	}
	defer trace.Stop()

	_ = runChannelBurst(50)
	return nil
}

func main() {
	fmt.Println("scheduler observation demo")
	fmt.Printf("GOMAXPROCS=%d NumCPU=%d\n", runtime.GOMAXPROCS(0), runtime.NumCPU())
	fmt.Println("Run with: GODEBUG=schedtrace=1000 go run .")
	if os.Getenv("GODEBUG") == "" {
		fmt.Println("GODEBUG is not set, so schedtrace lines will not appear.")
	} else {
		fmt.Printf("GODEBUG=%s\n", os.Getenv("GODEBUG"))
	}

	// 3초 동안 CPU-bound goroutine 을 GOMAXPROCS*2 개 돌려 스케줄러 상태가
	// 눈에 띄게 변하도록 만든다. 짧은 burst 와 달리 schedtrace 1초 간격에 걸친다.
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	cpuStats := runCPUWorkers(ctx, runtime.GOMAXPROCS(0)*2)
	fmt.Printf("CPU workers: started=%d iterations=%d\n", cpuStats.Started, cpuStats.Iterations)

	// channel burst 는 다수의 goroutine 이 동시에 기다렸다가 동시에 풀리는 상태를 보여준다.
	burst := runChannelBurst(200)
	fmt.Printf("Channel burst: %d goroutines blocked then released\n", burst)

	// execution tracer 파일 생성. go tool trace 로 열어 goroutine 상태 전이를 확인한다.
	f, err := os.Create("trace.out")
	if err != nil {
		fmt.Fprintf(os.Stderr, "create trace file: %v\n", err)
		os.Exit(1)
	}
	defer f.Close()

	if err := runTraceDemo(f); err != nil {
		fmt.Fprintf(os.Stderr, "trace demo: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("trace written to trace.out")
	fmt.Println("run: go tool trace trace.out")
}