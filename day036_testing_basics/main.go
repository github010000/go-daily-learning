package main

import (
	"fmt"
	"strings"
)

// Add는 두 정수를 더하여 결과를 반환하는 기본 함수입니다.
func Add(a, b int) int {
	return a + b
}

// FormatString은 입력 문자열의 앞뒤 공백을 제거하고 대문자로 변환합니다.
func FormatString(s string) string {
	return strings.ToUpper(strings.TrimSpace(s))
}

func main() {
	// 1. 기본 검증 로직 실행
	fmt.Println("=== Day 36: 테스팅 기초 (시뮬레이션 실행) ===")
	fmt.Println()

	result := Add(2, 3)
	if result != 5 {
		fmt.Printf("[실패] Add(2, 3) = %d (기대값: 5)\n", result)
	} else {
		fmt.Printf("[성공] Add(2, 3) = %d\n", result)
	}
	fmt.Println()

	// 2. 테이블 드리븐 테스트 패턴 구현
	// 테스트 케이스를 구조체 슬라이스로 관리하여 입력/기대값을 명시적으로 정의합니다.
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "공백 제거 및 대문자 변환", input: "  hello  ", want: "HELLO"},
		{name: "일반 문자열 변환", input: "world", want: "WORLD"},
		{name: "빈 문자열 처리", input: "", want: ""},
		{name: "숫자 및 특수문자 포함", input: "test_123!", want: "TEST_123!"},
	}

	fmt.Println("--- 테이블 드리븐 테스트 ---")
	for _, tt := range tests {
		got := FormatString(tt.input)
		if got != tt.want {
			fmt.Printf("[실패] %s: FormatString(%q) = %q, want %q\n", tt.name, tt.input, got, tt.want)
		} else {
			fmt.Printf("[성공] %s\n", tt.name)
		}
	}
	fmt.Println()

	// 3. t.Error vs t.Fatal 개념 설명
	// 실제 *_test.go 파일에서는 *testing.T 객체를 사용합니다.
	// - t.Error(): 테스트를 실패로 표시하지만, 함수 실행을 계속 진행합니다. 여러 검증을 한 번에 모을 때 유용합니다.
	// - t.Fatal(): 테스트를 실패로 표시하고 즉시 함수를 종료합니다. 이후 리소스 정리나 상태 검증이 필요할 때 사용합니다.
	fmt.Println("--- t.Error vs tFatal ---")
	fmt.Println("t.Error : 실패 기록 후 계속 진행 (여러 케이스 동시 검증에 적합)")
	fmt.Println("t.Fatal : 실패 기록 후 즉시 중단 (에arly exit 및 정리 로직 보호에 적합)")
	fmt.Println()

	// 4. go test -v -run 패턴 설명
	// 실제 개발 환경에서는 *_test.go 파일을 작성하고 터미널에서 아래 명령어로 실행합니다.
	fmt.Println("--- go test 명령어 패턴 ---")
	fmt.Println("go test           : 현재 패키지 모든 테스트 실행")
	fmt.Println("go test -v        : 상세 출력(Verbose) 활성화 (테스트 시작/끝 표시)")
	fmt.Println("go test -run=TestAdd : 정규식으로 TestAdd로 시작하는 테스트만 필터링 실행")
	fmt.Println()
	fmt.Println("학습이 완료되었습니다. 실제 프로젝트에서는 _test.go 파일에 testing 패키지를 직접 사용하세요.")
}