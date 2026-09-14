package main

import (
	"fmt"
	"os"
	"runtime"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"
)

const blockDuration = 100 * time.Millisecond

// goodBlockingSyscall 은 Go 의 syscall wrapper 인 syscall.Select 를 사용한다.
// 이 함수는 syscall.Syscall6 경로를 타면서 runtime.entersyscall 을 호출하기 때문에
// 커널에서 오래 막혀도 sysmon 이 10ms 뒤 P 를 회수해 다른 goroutine 에게 줄 수 있다.
func goodBlockingSyscall(d time.Duration) {
	tv := syscall.NsecToTimeval(d.Nanoseconds())
	_, err := syscall.Select(0, nil, nil, nil, &tv)
	if err != nil {
		fmt.Fprintf(os.Stderr, "good select error: %v\n", err)
	}
}

// badBlockingSyscall 은 같은 select syscall 을 RawSyscall6 로 직접 호출한다.
// RawSyscall6 는 runtime.entersyscall 을 호출하지 않으므로
// P 상태가 _Prunning 인 채로 M 과 함께 커널에 갇힌다.
// sysmon 은 P 를 _Psyscall 로 볼 수 없어 회수할 수 없다.
func badBlockingSyscall(d time.Duration) {
	tv := syscall.NsecToTimeval(d.Nanoseconds())
	_, _, errno := syscall.RawSyscall6(
		syscall.SYS_SELECT,
		0,
		0,
		0,
		0,
		uintptr(unsafe.Pointer(&tv)),
		0,
	)
	if errno != 0 {
		fmt.Fprintf(os.Stderr, "bad raw select error: %v\n", errno)
	}
}

// spinCounter 는 stop 이 닫힐 때까지 count 를 올리는 CPU 일감이다.
// GOMAXPROCS=1 에서 P 를 빼앗겼는지 확인하는 카나리아 역할을 한다.
func spinCounter(count *int64, stop <-chan struct{}) {
	for {
		select {
		case <-stop:
			return
		default:
			atomic.AddInt64(count, 1)
			// 일부러 자주 yield 해서 main goroutine 이 돌아올 기회를 준다.
			// 그래도 P 가 잡혀 있으면 yield 조차 할 수 없다.
			runtime.Gosched()
		}
	}
}

// startSpinWorker 는 spinCounter 를 별도 goroutine 으로 감싸고
// stop 채널과 done 채널을 돌려준다.
// done 은 spinCounter goroutine 이 완전히 끝났을 때 닫힌다.
func startSpinWorker(count *int64) (stop chan struct{}, done chan struct{}) {
	stop = make(chan struct{})
	done = make(chan struct{})
	go func() {
		defer close(done)
		spinCounter(count, stop)
	}()
	return stop, done
}

// measureProgress 는 GOMAXPROCS=1 로 고정한 뒤,
// block 함수가 d 동안 실행되는 동안 spinCounter 가 얼마나 진행됐는지 센다.
// goodBlockingSyscall 은 P 를 반납(handoff)하고, badBlockingSyscall 은 반납하지 못한다.
func measureProgress(block func(time.Duration), d time.Duration) int64 {
	old := runtime.GOMAXPROCS(1)
	defer runtime.GOMAXPROCS(old)

	var count int64
	stop, done := startSpinWorker(&count)

	// 여기서 runtime.Gosched 를 호출하지 않는 것이 중요하다.
	// 호출하면 main 의 P 를 worker 에게 넘겨서 bad 케이스에서도
	// worker 가 시작돼 버려서 차이가 흐려진다.
	block(d)

	close(stop)
	<-done
	return atomic.LoadInt64(&count)
}

// printInterpretation 은 실행 결과를 해석하는 문구를 출력한다.
func printInterpretation() {
	fmt.Println()
	fmt.Println("해석:")
	fmt.Println("- good: syscall.Select 는 Syscall6 을 타면서 entersyscall 을 호출해 P 상태를 _Psyscall 로 바꾼다.")
	fmt.Println("  sysmon 이 10ms 뒤 P 를 회수해 CPU goroutine 이 돈다.")
	fmt.Println("- bad: syscall.RawSyscall6 는 P 상태가 _Prunning 으로 남고,")
	fmt.Println("  sysmon 이 회수할 수 없어 CPU goroutine 이 굶는다.")
	fmt.Println()
	fmt.Println("GODEBUG=schedtrace=1000,scheddetail=1 go run . 으로 P 상태를 직접 볼 수 있다.")
}

func main() {
	fmt.Println("=== P handoff 실험 (GOMAXPROCS=1, block=100ms) ===")
	fmt.Printf("block duration: %s\n\n", blockDuration)

	good := measureProgress(goodBlockingSyscall, blockDuration)
	fmt.Printf("good: 100ms 동안 CPU goroutine 진행 수 = %d\n", good)

	bad := measureProgress(badBlockingSyscall, blockDuration)
	fmt.Printf("bad : 100ms 동안 CPU goroutine 진행 수 = %d\n", bad)

	printInterpretation()
}