package main

import (
	"errors"
	"fmt"
)

// 1. 기본 에러 정의 (errors.New 사용)
var ErrUserNotFound = errors.New("사용자를 찾을 수 없습니다")

// 2. 커스텀 에러 타입 정의
// 에러에 추가적인 정보를 담고 싶을 때 구조체를 사용합니다.
type ValidationError struct {
	Field   string // 에러가 발생한 필드 이름
	Message string // 에러 메시지
}

// error 인터페이스 구현 (Error() string 메서드 정의)
func (e *ValidationError) Error() string {
	return fmt.Sprintf("검증 실패 [%s]: %s", e.Field, e.Message)
}

// 3. 에러 래핑(Wrapping)을 사용하는 함수
// fmt.Errorf의 %w 동사를 사용하여 기존 에러를 포함(wrap)시킵니다.
func findUserByID(id int) (string, error) {
	if id <= 0 {
		// fmt.Errorf와 %w를 사용하여 하위 에러를 감쌉니다.
		return "", fmt.Errorf("잘못된 ID 요청: %w", ErrUserNotFound)
	}
		// 실제 로직에서는 DB 조회 등이 들어갑니다.
	return "홍길동", nil
}

// 4. 커스텀 에러를 반환하는 함수
func validateInput(name string) error {
	if name == "" {
		return &ValidationError{
			Field:   "Name",
			Message: "이름은 비어있을 수 없습니다.",
		}
	}
	return nil
}

func main() {
	fmt.Println("--- 1. 기본 에러 및 에러 래핑 테스트 ---")
	_, err := findUserByID(-1)
	if err != nil {
		fmt.Printf("발생한 에러: %v\n", err)

		// errors.Is를 사용하여 래핑된 에러 내부에 특정 에러가 있는지 확인
		if errors.Is(err, ErrUserNotFound) {
			fmt.Println("결과: ErrUserNotFound 에러임을 확인했습니다.")
		}
	}

	fmt.Println("\n--- 2. 커스텀 에러 타입 및 errors.As 테스트 ---")
	err = validateInput("")
	if err != nil {
		fmt.Printf("발생한 에러: %v\n", err)

		// errors.As를 사용하여 에러를 특정 타입으로 변환(Unwrapping/Casting)
		var vErr *ValidationError
		if errors.As(err, &vErr) {
			fmt.Printf("타입 추출 성공! 필드명: %s, 메시지: %s\n", vErr.Field, vErr.Message)
		}
	}

	fmt.Println("\n--- 3. 정상 케이스 테스트 ---")
	user, err := findUserByID(10)
	if err == nil {
		fmt.Printf("사용자 조회 성공: %s\n", user)
	}
}