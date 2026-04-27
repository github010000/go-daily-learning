package main

import (
	"fmt"
	"time"
)

func main() {
	// 1. 채널 생성
	ch1 := make(chan string)
	ch2 := make(chan string)

	// 2. 첫 번째 고루틴: 1초 후에 ch1에 데이터 전송
	go func() {
		time.Sleep(1 * time.Second)
		ch1 <- "Ch1 데이터 수신 완료"
	}()

	// 3. 두 번째 고루틴: 3초 후에 ch2에 데이터 전송 (타임아웃보다 늦음)
	go func() {
		time.Sleep(3 * time.Second)
		ch2 <- "Ch2 데이터 수신 완료"
	}()

	fmt. *Println("--- [Case 1] 다중 채널 대기 및 타임아웃 테스트 ---")
	
	// 4. select 문을 이용한 다중 채널 제어
	// select는 여러 채널 중 하나가 준비될 때까지 대기합니다.
	select {
	case msg1 := <-ch1:
		fmt.Println("받은 메시지:", msg1)
	case msg2 := <-ch2:
		fmt.Println("받은 메시지:", msg2)
	case <-time.After(2 * time.Second):
		// 2초 동안 아무런 채널도 준비되지 않으면 타임아웃 발생
		fmt.Println("타임아웃: 2초 동안 응답이 없습니다.")
	}

	fmt.Println("\n--- [Case 2] Non-blocking (default) 테스트 ---")

	// 5. default 케이스를 이용한 Non-blocking 통신
	// 채널에 데이터가 즉시 준비되어 있지 않아도 멈추지 않고 바로 실행됩니다.
	unbufferedCh := make(chan string)

	select {
	case msg := <-unbufferedCh:
		fmt.Println("데이터 수신:", msg)
	default:
		// 채널에서 데이터를 즉시 가져올 수 없을 때 실행됨
		fmt.Println("데이터가 준비되지 않아 default 케이스를 실행합니다. (Non-blocking)")
	}

	fmt.Println("\n--- [Case 3] 모든 채널이 준비되었을 때의 동작 ---")
	// 채널에 데이터가 미리 채워져 있는 상황
	readyCh := make(chan string, 1)
	readyCh <- "즉시 준비된 데이터"

	select {
	case msg := <-readyCh:
		fmt.Println("즉시 수신 가능:", msg)
	case <-time.After(1 * time.Second):
		fmt.Println("타임아웃 발생")
	}
}