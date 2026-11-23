package main

import (
	"fmt"
	"time"
)

// 버퍼 없는 채널: 보내는 쪽과 받는 쪽이 만나야 값이 넘어간다.
// 송신을 고루틴에서 하는 이유는, main 에서 먼저 보내면 받는 쪽이 없어 멈추기 때문이다.
func unbufferedHandoff() string {
	ch := make(chan string)
	go func() { ch <- "안녕" }()
	return <-ch
}

// 버퍼 있는 채널: 칸이 남아 있는 동안 보내는 쪽은 기다리지 않는다.
func bufferedBasics() (length, capacity int, got []int) {
	ch := make(chan int, 3)
	ch <- 1
	ch <- 2
	length = len(ch)
	capacity = cap(ch)
	got = append(got, <-ch, <-ch)
	return
}

// 칸이 가득 찼는지 멈추지 않고 확인하려면 default 가 있는 select 를 쓴다.
func bufferedIsFull() bool {
	ch := make(chan int, 1)
	ch <- 1
	select {
	case ch <- 2:
		return false
	default:
		return true
	}
}

// 닫힌 채널에서 받으면 남은 값을 다 준 뒤 제로값과 ok=false 를 준다.
func readClosed() []string {
	ch := make(chan int, 1)
	ch <- 7
	close(ch)
	var out []string
	for range 2 {
		v, ok := <-ch
		out = append(out, fmt.Sprintf("값=%d ok=%t", v, ok))
	}
	return out
}

// 실수 버전: ok 를 확인하지 않고 개수만 믿고 받으면 제로값이 데이터처럼 섞여 들어온다.
func drainClosedWrong(n int) []int {
	ch := make(chan int, 2)
	ch <- 1
	ch <- 2
	close(ch)
	var out []int
	for range n {
		out = append(out, <-ch)
	}
	return out
}

// 고친 버전: range 는 채널이 닫히고 비면 스스로 끝난다.
func drainClosedRight() []int {
	ch := make(chan int, 2)
	ch <- 1
	ch <- 2
	close(ch)
	var out []int
	for v := range ch {
		out = append(out, v)
	}
	return out
}

// 보내는 쪽(생산자)이 close 한다. defer 로 두면 중간에 return 해도 빠뜨리지 않는다.
func producer(n int) <-chan int {
	ch := make(chan int)
	go func() {
		defer close(ch)
		for i := 1; i <= n; i++ {
			ch <- i
		}
	}()
	return ch
}

func sum(ch <-chan int) int {
	total := 0
	for v := range ch {
		total += v
	}
	return total
}

// 실수 버전: 이미 닫힌 채널에 보내면 panic. 받는 쪽이 먼저 close 해 버렸을 때 생긴다.
func sendOnClosed() (msg string) {
	defer func() {
		if r := recover(); r != nil {
			msg = fmt.Sprint("panic 발생: ", r)
		}
	}()
	ch := make(chan int, 1)
	close(ch)
	ch <- 1
	return "panic 없음"
}

// 일꾼 고루틴: release 가 닫히면 결과를 ch 에 보내고, 끝나면 done 을 닫는다.
// buf 가 0 이면 받는 쪽이 없을 때 영원히 멈추고(누수), 1 이면 보내고 바로 끝난다.
func startWorker(buf int) (ch chan string, release, done chan struct{}) {
	ch = make(chan string, buf)
	release = make(chan struct{})
	done = make(chan struct{})
	go func() {
		defer close(done)
		<-release
		ch <- "결과"
	}()
	return
}

// 호출자가 결과를 포기한(아무도 안 받는) 상황에서 일꾼이 끝나는지 본다.
func workerFinishes(buf int) bool {
	ch, release, done := startWorker(buf)
	close(release)
	finished := false
	select {
	case <-done:
		finished = true
	case <-time.After(300 * time.Millisecond):
	}
	<-ch // 시연 뒤 정리: 받아 주면 누수된 고루틴도 풀려난다
	return finished
}

func main() {
	fmt.Println("버퍼 없는 채널로 받은 값:", unbufferedHandoff())

	l, c, got := bufferedBasics()
	fmt.Println("버퍼 채널 len, cap, 받은 값:", l, c, got)
	fmt.Println("칸이 가득 찼는가:", bufferedIsFull())

	fmt.Println("닫힌 채널에서 받기:", readClosed())
	fmt.Println("실수 버전(ok 무시, 4번 받기):", drainClosedWrong(4))
	fmt.Println("고친 버전(range):", drainClosedRight())

	fmt.Println("생산자가 close 한 채널의 합:", sum(producer(5)))
	fmt.Println("실수 버전(닫힌 채널에 보내기):", sendOnClosed())

	fmt.Println("실수 버전(버퍼 0, 아무도 안 받음) 일꾼이 끝났나:", workerFinishes(0))
	fmt.Println("고친 버전(버퍼 1) 일꾼이 끝났나:", workerFinishes(1))
}