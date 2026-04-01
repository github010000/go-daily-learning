package main

import (
	"fmt"
	"os"
)

// main 함수: 모든 Go 프로그램의 시작 지점
// main 패키지 내에 반드시 하나의 main 함수만 존재해야 하며, 실행 시 가장 먼저 호출됨
func main() {
	// 기본적인 Hello, World! 출력
	fmt.Println("Hello, World!")

	// fmt.Printf를 사용하여 형식화된 텍스트 출력
	fmt.Printf("Go 언어에 오신 것을 환영합니다!\n")

	// os 패키지의 Exit 함수를 사용해 종료 코드 출력 (예시용)
	// 실제로는 생략해도 되지만, exit 코드를 명시적으로 보여주기 위해 추가
	fmt.Printf("프로그램 정상 종료 (exit code 0)\n")
	os.Exit(0)
}
