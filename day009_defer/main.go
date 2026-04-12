package main

import (
	"fmt"
	"time"
)

// cleanupResource는 함수가 종료될 때 호출되는 정리(cleanup) 로직을 시뮬레이션합니다.
func cleanupResource(name string) {
	fmt.Printf("[CLEANUP] %s 자원 해제: 연결을 닫거나 파일을 닫는 작업을 수행합니다.\n", name)
}

// processData는 defer를 사용하여 리소스 해제 패턴을 보여주는 함수입니다.
func processData(resourceName string) {
	fmt.Printf("\n--- processData 함수 시작: %s 사용 ---\n", resourceName)

	// 1. 첫 번째 defer 실행 (가장 마지막에 실행됨)
	defer cleanupResource("데이터베이스 연결")

	// 2. 두 번째 defer 실행 (첫 번째 defer보다 먼저 실행됨)
	defer func(msg string) { 
		fmt.Printf("[DEFER 2] 이 함수가 종료되기 직전에 실행되는 로직입니다: %s\n", msg)
	}(resourceName)

	fmt.Println("메인 로직 수행 시작...")
	time.Sleep(100 * time.Millisecond) // 로직 실행 시간 시뮬레이션
	fmt.Println("메인 로직 성공적으로 완료.")

	// 함수가 끝날 때, defer들이 LIFO(Last-In, First-Out) 순서로 호출됩니다.
}

// main 함수는 전체 실행 흐름을 제어합니다.
func main() {
	fmt.Println("===============================================")
	fmt.Println("Day 9: defer 키워드 학습 시작")
	fmt.Println("===============================================")

	// 첫 번째 테스트 케이스
	processData("파일 핸들 A")
	
	fmt.Println("
===================================================")

	// 두 번째 테스트 케이스: 리소스 누수 방지 시뮬레이션
	fmt.Println("--- 예외 상황 시뮬레이션 ---")
	defer func() {
		fmt.Println("[MAIN DEFER] main 함수 전체가 종료될 때 호출되는 최종 정리 로직입니다.")
	}()

	// 여기서 의도적으로 panic을 발생시켜 함수 흐름을 끊어봅니다.
	func() {
		defer func() {
			// panic 발생 여부와 관계없이 호출됩니다.
			fmt.Println("[DEFER 3] panic 발생 전후를 막론하고 실행되는 복구 로직입니다.")
		}()
		fmt.Println("panic을 유발하는 코드 직전 로직 실행.")
		panic("실패 상황 발생으로 인해 함수를 즉시 종료합니다!")
	}
	
	fmt.Println("main 함수 실행 계속 (panic으로 인해 여기까지 도달하기 어려움)")
}
