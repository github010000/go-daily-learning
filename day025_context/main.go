package main

import (
	"context"
	"fmt"
	"time"
)

// worker는 context의 상태를 감시하며 작업을 수행하는 함수입니다.
// Go의 관례에 따라 context는 함수의 첫 번째 인자로 전달됩니다.
func worker(ctx context.Context, id int) {
	fmt.Printf("[Worker %d] 작업 시작\n", id)

	for {
		select {
		// ctx.Done() 채널이 닫히면(취소되면) 작업을 중단합니다.
		case <-ctx.Done():
			fmt.Printf("[Worker %d] 취소됨: %v\n", id, ctx.Err())
			return
		// 실제 작업을 시뮬레이션하기 위해 500ms마다 메시지를 출력합니다.
		case <-time.After(500 * time.Millisecond):
			fmt.Printf("[Worker %d] 작업 중...\n", id)
		}
	}
}

func main() {
	// 1. context.Background(): 가장 기본이 되는 루트 컨텍스트입니다.
	// 보통 메인 함수나 요청의 시작점에서 생성됩니다.
	rootCtx := context.Background()

	// --- 예제 1: context.WithCancel (수동 취소) ---
	fmt.Println("\n--- 예제 1: WithCancel 시작 ---")
	cancelCtx, cancelFunc := context.WithCancel(rootCtx)
	
	go worker(cancelCtx, 1)

	// 2초 후에 수동으로 취소 함수를 호출하여 고루틴을 종료시킵니다.
	time.Sleep(2 * time.Second)
	fmt.Println("[Main] Cancel 함수 호출!")
	cancelFunc() // 이 호출이 자식 고루틴에 취소 신호를 전파합니다.

	// --- 예제 2: context.WithTimeout (시간 제한 취소) ---
	fmt.Println("\n--- 예제 2: WithTimeout 시작 (3초 제한) ---")
	timeoutCtx, timeoutFunc := context.WithTimeout(rootCtx, 3*time.Second)
	defer timeoutFunc() // 리소스 누수를 방지하기 위해 defer로 cancel 호출

	go worker(timeoutCtx, 2)

	// 3초가 지나면 자동으로 ctx.Done()이 닫힙니다.
	time.Sleep(5 * time.Second)

	// --- 예제 3: context.WithDeadline (특정 시점 취소) ---
	fmt.Println("\n--- 예제 3: WithDeadline 시작 (특정 시점 지정) ---")
	// 현재 시간으로부터 2초 뒤를 마감 시한으로 설정합니다.
	deadline := time.Now().Add(2 * time.Second)
	deadlineCtx, deadlineFunc := context.WithDeadline(rootCtx, deadline)
	defer deadlineFunc()

	go worker(deadlineCtx, 3)

	// 마감 시한이 지나갈 때까지 대기합니다.
	time.Sleep(4 * time.Second)

	fmt.Println("\n[Main] 모든 예제 종료")
}