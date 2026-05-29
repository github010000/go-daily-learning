package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	// CLI 실행 시 최소 인자 수 확인
	if len(os.Args) < 2 {
		fmt.Println("사용법: mycli <서브커맨드>")
		fmt.Println("서브커맨드: greet, version, help")
		os.Exit(1)
	}

	// 첫 번째 인자를 서브커맨드로 라우팅
	subCmd := os.Args[1]
	var exitCode int

	switch subCmd {
	case "greet":
		exitCode = handleGreet()
	case "version":
		exitCode = handleVersion()
	case "help":
		exitCode = handleHelp()
	default:
		fmt.Printf("알 수 없는 서브커맨드: %s\n", subCmd)
		exitCode = 1
	}

	// 모든 로직의 종료 코드를 한 곳에서 관리
	os.Exit(exitCode)
}

// greet 서브커맨드 처리 함수
// flag.NewFlagSet을 사용하여 독립적인 인자 파싱 컨텍스트 생성
func handleGreet() int {
	fs := flag.NewFlagSet("greet", flag.ContinueOnError)
	name := fs.String("name", "World", "인사할 이름")

	// os.Args[2:]부터 파싱 (서브커맨드 이름 제외)
	if err := fs.Parse(os.Args[2:]); err != nil {
		// flag.ContinueOnError은 에러 발생 시 0을 반환하지 않고 에러를 리턴
		fmt.Fprintf(os.Stderr, "greet 명령 에러: %v\n", err)
		return 1
	}

	fmt.Printf("안녕하세요, %s님! (실행 시간: %s)\n", *name, "Day 55")
	return 0
}

// version 서브커맨드 처리 함수
func handleVersion() int {
	fmt.Println("mycli v1.0.0")
	fmt.Println("Go 표준 라이브러리 학습용 미니 CLI 도구")
	return 0
}

// help 서브커맨드 처리 함수
func handleHelp() int {
	fmt.Println("=== mycli 도움말 ===")
	fmt.Println("  greet   -name=홍길동   특정 이름으로 인사 메시지 출력")
	fmt.Println("  version             버전 정보 출력")
	fmt.Println("  help                이 도움말 표시")
	return 0
}