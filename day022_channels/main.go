package main

import (
	"fmt"
	"time"
)

// producer 함수는 데이터를 생성하여 채널에 보냅니다.
// chan<- int 는 '송신 전용(send-only)' 채널임을 의미합니다.
func producer(ch chan<- int) {
	for i := 1; i <= 5; i++ {
		fmt.Printf("[Producer] 데이터 생성 중: %d\n", i)
		ch <- i // 채널로 데이터 송신
		time.Sleep(200 * time.Millisecond) // 작업 시뮬레이션
	}
	fmt.Println("[Producer] 모든 데이터 전송 완료. 채널을 닫습니다.")
	close(ch) // 채널을 닫음 (더 이상 보낼 데이터가 없음을 알림)
}

// consumer 함수는 채널로부터 데이터를 읽어옵니다.
// <-chan int 는 '수신 전용(receive-only)' 채널임을 의미합니다.
func consumer(ch <-chan int) {
	// range를 사용하여 채널이 닫힐 때까지 데이터를 계속 수신합니다.
	for val := range ch {
		fmt.Printf("[Consumer] 데이터 수신 완료: %d\n", val)
		time.Sleep(500 * time.Millisecond) // 수신 처리 시뮬레이션
	}
	fmt.Println("[Consumer] 채널이 닫혀 수신을 종료합니다.")
}

func main() {
	fmt.Println("--- 1. Unbuffered Channel (비버퍼드 채널) 테스트 ---")
	unbufferedCh := make(chan int) // 버퍼 크기가 0인 채널 생성

	go producer(unbufferedCh)
	consumer(unbufferedCh)

	fmt.Println("\n--- 2. Buffered Channel (버퍼드 채널) 테스트 ---")
	// 버퍼 크기가 3인 채널 생성 (3개까지는 수신자가 없어도 송신이 가능)
	bufferedCh := make(chan int, 3)

	go func() {
		for i := 1; i <= 5; i++ {
			fmt.Printf("[Producer] 버퍼 채널 데이터 송신: %d\n", i)
			bufferedCh <- i
			fmt.Println("[Producer] 송신 직후 상태 (버퍼가 가득 차지 않았다면 바로 다음으로 진행)")
		}
		close(bufferedCh)
	}()

	// 버퍼가 다 차면 수신자가 읽을 때까지 송신자가 대기하게 됩니다.
	for val := range bufferedCh {
		fmt.Printf("[Consumer] 버퍼 채널 데이터 수신: %d\n", val)
		time.Sleep(100 * time.Millisecond)
	}

	fmt.Println("\n모든 채널 테스트 종료.")
}