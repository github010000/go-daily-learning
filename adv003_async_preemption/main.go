package main

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// demoIterations는 시연용 반복 횟수다.
// 함수 호출이 없는 tight loop가 협조적 선점에서 왜 문제가 되는지
// 충분히 오래 실행되도록 크게 잡는다.
const demoIterations = 2_000_000_000

// tightLoop는 의도적으로 어떤 함수도 호출하지 않는다.
// Go 1.14 이전에는 이런 루프에 안전점이 없어서 GC의 stop-the-world가
// 끝날 때까지 main goroutine이 기다려야 했다.
func tightLoop(iterations uint64) uint64 {
	var sum uint64
	for i := uint64(0); i < iterations; i++ {
		sum += i
	}
	return sum
}

// loopDemoResult는 runTightLoopDemo가 측정한 값을 모은다.
type loopDemoResult struct {
	sum          uint64
	resume       time.Duration
	total        time.Duration
	stillRunning bool
}

// runTightLoopDemo는 tightLoop를 별도 goroutine에서 실행하고,
// runtime.Gosched로 양보한 뒤 main goroutine이 다시 스케줄링될 때까지
// 걸린 시간을 측정한다. 호출 전에 GOMAXPROCS를 1로 설정해야
// 하나의 P에서만 실행되어 선점 차이가 극명하게 드러난다.
func runTightLoopDemo(iterations uint64) loopDemoResult {
	done := make(chan uint64, 1)
	start := time.Now()

	go func() {
		done <- tightLoop(iterations)
	}()

	// Gosched는 현재 goroutine을 run queue에 넣고 다른 goroutine을 고른다.
	// 선점이 없으면 tightLoop가 끝날 때까지 여기서 반환되지 않는다.
	runtime.Gosched()

	resume := time.Since(start)

	stillRunning := true
	var sum uint64
	select {
	case sum = <-done:
		stillRunning = false
	default:
	}

	if stillRunning {
		sum = <-done
	}

	return loopDemoResult{
		sum:          sum,
		resume:       resume,
		total:        time.Since(start),
		stillRunning: stillRunning,
	}
}

// printResult는 측정 결과를 보기 좋게 출력한다.
func printResult(mode string, r loopDemoResult) {
	fmt.Printf("[%s] main resumed after %v (tight loop still running: %v)\n",
		mode, r.resume, r.stillRunning)
	fmt.Printf("[%s] tight loop finished after %v, sum=%d\n",
		mode, r.total, r.sum)
}

// childEnv는 기존 GODEBUG 값을 제거하고 asyncpreemptoff=1로 설정한 환경을
// 반환한다. 단순히 append하면 중복 GODEBUG이 생겨 어떤 값이 적용될지
// 명확하지 않으므로 명시적으로 교체한다.
func childEnv() []string {
	env := os.Environ()
	out := make([]string, 0, len(env)+1)
	for _, e := range env {
		if strings.HasPrefix(e, "GODEBUG=") {
			continue
		}
		out = append(out, e)
	}
	return append(out, "GODEBUG=asyncpreemptoff=1", "DEMO_CHILD=1")
}

func main() {
	// GOMAXPROCS를 1로 설정해 tight loop와 main이 같은 P를 쓰게 한다.
	// 그래야 협조적 선점이 없을 때 starvation이 재현된다.
	runtime.GOMAXPROCS(1)

	// 자식 모드로 실행되면 async preemption을 끈 상태만 시연하고 종료한다.
	if os.Getenv("DEMO_CHILD") == "1" {
		fmt.Println("== child: asynchronous preemption OFF ==")
		result := runTightLoopDemo(demoIterations)
		printResult("OFF", result)
		return
	}

	// 부모 모드에서는 먼저 async preemption이 켜진 상태로 시연한다.
	fmt.Println("== parent: asynchronous preemption ON ==")
	result := runTightLoopDemo(demoIterations)
	printResult("ON", result)

	fmt.Println()
	fmt.Println("spawning child with GODEBUG=asyncpreemptoff=1 ...")

	exe, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "cannot find executable: %v\n", err)
		os.Exit(1)
	}

	cmd := exec.Command(exe)
	cmd.Env = childEnv()
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "child failed: %v\n", err)
		os.Exit(1)
	}
}