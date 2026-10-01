package main

import (
	"fmt"
	"runtime"
	"sync"
)

// eventRecorder 는 여러 goroutine 이 동시에 기록해도 안전하게
// 사건 순서를 문자열로 모은다. 채널의 동기화 동작을 눈으로 확인하기 위한 도구다.
type eventRecorder struct {
	mu     sync.Mutex
	events []string
}

func (r *eventRecorder) add(format string, args ...any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, fmt.Sprintf(format, args...))
}

func printEvents(title string, events []string) {
	fmt.Println("---", title, "---")
	for _, e := range events {
		fmt.Println("  ", e)
	}
	fmt.Println()
}

// unbufferedHandoffSequence 는 dataqsiz=0 인 채널에서 송신이
// 수신자의 receive 와 짝을 이룰 때까지 완료되지 않는다는 것을 보여준다.
// runtime/chan.go 의 chansend 가 recvq 에서 sudog 를 꺼내
// receiver 의 스택에 직접 값을 복사하는 경로를 떠올리면 된다.
func unbufferedHandoffSequence() []string {
	ch := make(chan int) // hchan.dataqsiz == 0
	rec := &eventRecorder{}
	ready := make(chan struct{})
	got := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		rec.add("receiver goroutine: blocked before receive")
		close(ready) // receiver 가 receive 직전까지 도달했음을 main 에 알린다.
		v := <-ch
		rec.add("receiver goroutine: got value %d", v)
		close(got) // receiver 가 값을 실제로 가져간 뒤에야 닫는다.
	}()

	<-ready // receiver 가 receive 직전까지 실행된 것을 확인한다.
	rec.add("sender goroutine: about to send on unbuffered channel")
	ch <- 42

	// unbuffered 송신은 receiver 에 값이 전달되는 순간 반환되지만,
	// receiver goroutine 이 got 이벤트까지 기록했는지는 스케줄러에 달렸다.
	// got 채널을 기다려 "수신자가 값을 확인했다"는 순서를 확정한다.
	<-got
	rec.add("sender goroutine: send returned (receiver confirmed got value)")

	wg.Wait()
	return rec.events
}

// bufferedSendSequence 는 dataqsiz > 0 인 채널에서는 수신자가 없어도
// 빈 슬롯에 값을 넣고 송신이 바로 반환된다는 것을 보여준다.
func bufferedSendSequence() []string {
	ch := make(chan int, 1) // hchan.buf 에 정수 하나 크기의 배열이 생긴다.
	rec := &eventRecorder{}
	var wg sync.WaitGroup
	wg.Add(1)

	rec.add("sender goroutine: about to send on buffered channel (cap 1)")
	ch <- 42
	rec.add("sender goroutine: send returned even though no receiver yet")

	go func() {
		defer wg.Done()
		rec.add("receiver goroutine: about to receive")
		v := <-ch
		rec.add("receiver goroutine: got value %d", v)
	}()

	wg.Wait()
	return rec.events
}

// bufferedCircularFIFOSequence 는 hchan.buf 가 환형 배열이어서
// sendx/recvx 가 한 바퀴 돌아도 FIFO 순서를 유지한다는 것을 보여준다.
func bufferedCircularFIFOSequence() []string {
	ch := make(chan int, 2) // [_,_]
	rec := &eventRecorder{}

	rec.add("buffered cap 2: send 1 and 2 before any receive")
	ch <- 1 // sendx = 0
	ch <- 2 // sendx = 1

	v1 := <-ch // recvx = 0
	v2 := <-ch // recvx = 1
	rec.add("first receive got: %d", v1)
	rec.add("second receive got: %d", v2)
	return rec.events
}

// channelCapacityAndLength 는 len/channel 로
// hchan.qcount/dataqsiz 를 간접적으로 확인한다.
func channelCapacityAndLength() []string {
	unbuffered := make(chan int)
	buffered := make(chan int, 2)
	buffered <- 10

	return []string{
		fmt.Sprintf("unbuffered len=%d cap=%d (qcount/dataqsiz conceptually)", len(unbuffered), cap(unbuffered)),
		fmt.Sprintf("buffered after one send len=%d cap=%d", len(buffered), cap(buffered)),
	}
}

// runSchedTraceHint 는 GODEBUG=schedtrace 로 채널에 붙어 있는
// 대기 goroutine 들을 어떤 관점에서 봐야 하는지 안내한다.
func runSchedTraceHint() {
	fmt.Println("GODEBUG=schedtrace=1000 go run . 을 실행하면")
	fmt.Println("1초마다 goroutine 상태(runnable, idle, syscall 등)와")
	fmt.Println("runqueue 길이, P 개수 등이 출력된다.")
	fmt.Println("채널에 붙어 대기 중인 goroutine 은 sleep 상태로 보이고 CPU를 먹지 않는다.")
	fmt.Println()
}

func main() {
	fmt.Println("== goroutine count at start ==")
	fmt.Printf("runtime.NumGoroutine()=%d\n\n", runtime.NumGoroutine())

	printEvents("unbuffered handoff (direct stack copy)", unbufferedHandoffSequence())
	printEvents("buffered send (slot in hchan.buf)", bufferedSendSequence())
	printEvents("buffered circular FIFO", bufferedCircularFIFOSequence())

	fmt.Println("== len/cap ==")
	for _, line := range channelCapacityAndLength() {
		fmt.Println("  ", line)
	}
	fmt.Println()

	runSchedTraceHint()
}