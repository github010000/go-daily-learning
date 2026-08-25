package main

import (
	"fmt"
	"os"
	"runtime"
	"time"
)

// sink 는 tight loop 가 값을 써 넣는 전역 변수다.
// 함수 호출을 만들지 않으면서도 컴파일러가 루프를 제거하지 못하게 하는 역할이다.
var sink int

// tightLoopNoYield 는 협조적 선점 지점(함수 호출)을 전혀 만들지 않는다.
// 1.13까지는 이 루프가 P를 독점해 STW나 다른 goroutine 기아를 일으켰다.
// 1.14부터는 비동기 선점이 시그널로 이 루프를 끊어낸다.
func tightLoopNoYield() {
	i := 0
	for {
		sink = i
		i++
	}
}

// tightLoopWithYield 는 매 반복마다 runtime.Gosched()를 호출해
// 협조적 선점 지점을 만든다. 비동기 선점이 없던 시절에는 이렇게
// 양보 지점을 직접 넣어야 다른 goroutine이 실행될 수 있었다.
func tightLoopWithYield() {
	i := 0
	for {
		sink = i
		i++
		runtime.Gosched()
	}
}

// ticker 는 0..n-1 을 ch 로 보낸다.
// GOMAXPROCS=1 이면 tight loop 가 먼저 실행되고 나서야
// 이 함수가 실행 기회를 얻는다(비동기 선점이 없으면 영영 못 얻는다).
func ticker(ch chan int, n int) {
	for i := 0; i < n; i++ {
		ch <- i
	}
}

// runPreemptionDemo 는 GOMAXPROCS=1 환경에서
//   - useYield=false: tight loop 가 preemption point 없이 계속 돌 때
//   - useYield=true:  tight loop 가 Gosched 로 양보할 때
// 다른 goroutine(ticker)이 진행되는지 관찰한다.
//
// 반환값:
//   - ok: 10개의 값이 모두 수신되면 true, 시간 내 진행이 없으면 false
//   - elapsed: 첫 수신을 시작한 시점부터 10개를 받기까지 걸린 시간
//
// 이 함수는 async preemption 이 꺼져 있으면 useYield=false 일 때
// false 를 반환한다. 즉 GODEBUG=asyncpreemptoff=1 로 실행하면
// 옛날(1.13 이전) 동작을 재현할 수 있다.
func runPreemptionDemo(useYield bool) (ok bool, elapsed time.Duration) {
	old := runtime.GOMAXPROCS(1)
	defer runtime.GOMAXPROCS(old)

	ch := make(chan int)

	if useYield {
		go tightLoopWithYield()
	} else {
		go tightLoopNoYield()
	}

	// ticker 는 0..9 를 ch 에 보낸다.
	// GOMAXPROCS=1 이므로 tight loop 가 먼저 실행되고 나서야
	// ticker 가 실행 기회를 얻는다. 비동기 선점이 없으면
	// 이 goroutine 은 계속 run queue 에서 대기하게 된다.
	go ticker(ch, 10)

	start := time.Now()
	count := 0
	// 비동기 선점이 켜져 있으면 tight loop 가 10ms 주기로 선점되어
	// ticker 가 보낸 값을 받을 수 있다. 넉넉히 1초를 주고,
	// 그래도 진행이 없으면 false 를 반환한다(옛날 동작 재현).
	timeout := time.After(1 * time.Second)

	for count < 10 {
		select {
		case v := <-ch:
			// v 는 0부터 9까지 증가한다.
			count = v + 1
		case <-timeout:
			return false, time.Since(start)
		}
	}
	return true, time.Since(start)
}

func main() {
	mode := "noyield"
	if len(os.Args) > 1 {
		mode = os.Args[1]
	}

	useYield := mode == "yield"
	ok, elapsed := runPreemptionDemo(useYield)

	fmt.Printf("mode=%s, progress=%v, elapsed=%v\n", mode, ok, elapsed)
	if !ok {
		fmt.Println("진행이 없습니다. 비동기 선점이 꺼져 있거나 tight loop 가 선점되지 않았습니다.")
		fmt.Println("GODEBUG=asyncpreemptoff=1 로 실행하면 협조적 선점만 쓰던 1.13 이전 동작을 볼 수 있습니다.")
		os.Exit(1)
	}
	fmt.Println("진행 완료: tight loop 가 실행 중이어도 다른 goroutine 이 동작했습니다.")
}