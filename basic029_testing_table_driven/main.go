package main

import (
	"fmt"
	"strconv"
	"strings"
)

// parsePositiveInt는 양의 정수를 파싱하지만, 분기마다 에러 메시지 포맷이 제각각인
// "분산된 검증" 패턴의 실수 예시입니다. 조건이 늘어날수록 일관성이 무너지고 유지보수가
// 어려워집니다.
func parsePositiveInt(s string) (int, error) {
	if len(s) == 0 {
		return 0, fmt.Errorf("empty input")
	}
	if s == "0" {
		return 0, nil
	}
	if strings.HasPrefix(s, "-") {
		return 0, fmt.Errorf("negative not allowed")
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("invalid integer: %v", err)
	}
	if n <= 0 {
		return 0, fmt.Errorf("must be positive")
	}
	return n, nil
}

// parsePositiveIntFixed는 에러 처리를 단순화하고, 올바른 파싱 동작을 합니다.
// 빈 문자열과 음수는 에러를 반환하고, 그 외 정상 양수는 정수로 변환합니다.
func parsePositiveIntFixed(s string) (int, error) {
	if len(s) == 0 {
		return 0, fmt.Errorf("empty input")
	}
	if strings.HasPrefix(s, "-") {
		return 0, fmt.Errorf("negative not allowed")
	}
	return strconv.Atoi(s)
}

func main() {
	fmt.Println("테이블 주도 테스트 기초 시연")
	fmt.Println("==============================")

	// 1. 실수 버전: 분기별 에러 메시지 일관성 붕괴
	fmt.Println("\n[1] 실수 버전: 조건문으로 분기 처리")
	fmt.Println("    - 새로운 케이스 추가 시 에러 메시지 포맷이 각기 다릅니다.")
	fmt.Println("    - '0' 처리가 별도로 분리되어 있어 로직이 산만합니다.")
	fmt.Println()

	samples := []string{"123", "0", "-42", "abc", "12a", ""}
	for _, s := range samples {
		n, err := parsePositiveInt(s)
		if err != nil {
			fmt.Printf("  %q -> 에러: %v\n", s, err)
		} else {
			fmt.Printf("  %q -> %d\n", s, n)
		}
	}

	// 2. 고친 버전: 테이블 주도 테스트 시연
	fmt.Println("\n[2] 고친 버전: 테이블 주도 테스트")
	fmt.Println("    - 케이스(데이터)와 검증 로직(코드)를 분리합니다.")
	fmt.Println("    - 새로운 케이스는 테이블에 행을 추가하는 것만으로 끝납니다.")
	fmt.Println()

	table := []struct {
		name     string
		input    string
		expected int
		wantErr  bool
	}{
		{"양수", "123", 123, false},
		{"영", "0", 0, false},
		{"음수", "-42", 0, true},
		{"문자", "abc", 0, true},
		{"혼합", "12a", 0, true},
		{"빈 문자열", "", 0, true},
	}

	for _, tc := range table {
		fmt.Printf("  [%s] input=%q\n", tc.name, tc.input)
		result, err := parsePositiveIntFixed(tc.input)
		if tc.wantErr {
			if err == nil {
				fmt.Printf("    기대: 에러 / 실제: result=%d (에러 없음)\n", result)
			} else {
				fmt.Printf("    기대: 에러 / 실제: 에러(%v) ✓\n", err)
			}
		} else {
			if err != nil {
				fmt.Printf("    기대: %d / 실제: 에러(%v) ✗\n", tc.expected, err)
			} else if result != tc.expected {
				fmt.Printf("    기대: %d / 실제: %d ✗\n", tc.expected, result)
			} else {
				fmt.Printf("    기대: %d / 실제: %d ✓\n", tc.expected, result)
			}
		}
	}

	// 3. t.Run() 활용: 서브테스트
	fmt.Println("\n[3] t.Run() 활용: 서브테스트")
	fmt.Println("    - 각 케이스에 이름을 부여하면 -run 플래그로 선택 실행 가능합니다.")
	fmt.Println("    - 예: go test -run '테이블/양수'")
	fmt.Println("    - 실패 시 어느 케이스가 문제인지 명확히 알 수 있습니다.")

	// 4. t.Helper() 활용: 헬퍼 함수
	fmt.Println("\n[4] t.Helper() 활용")
	fmt.Println("    - 에러가 헬퍼 함수 안에서 발생해도, 에러 위치가 호출부로 표시됩니다.")
	fmt.Println("    - 디버깅 시 어디서 검증이 실패했는지 한눈에 파악됩니다.")

	// 5. 실패 메시지 잘 쓰기
	fmt.Println("\n[5] 실패 메시지 잘 쓰기")
	fmt.Println("    - t.Errorf(\"expected %d, got %d\", want, got)")
	fmt.Println("    - 메시지에 기대값과 실제값을 모두 포함시킵니다.")
	fmt.Println("      (테스트 로그를 볼 때 둘 다 있어야 왜 틀렸는지 알 수 있습니다)")

	fmt.Println("\n시연 완료!")
}