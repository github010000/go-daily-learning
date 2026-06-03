package main

import (
	"errors"
	"fmt"
)

// UserNotFoundError는 사용자 찾기 실패 시 반환하는 커스텀 에러 타입입니다.
// 에러 처리 관례: errors.Is/As로 검사할 수 있도록 Error() string를 구현합니다.
type UserNotFoundError struct {
	ID string
}

func (e *UserNotFoundError) Error() string {
	return fmt.Sprintf("사용자를 찾을 수 없습니다: %s", e.ID)
}

// FindUser는 에러 반환 관례와 단일 책임 설계 예시입니다.
// Go 관용구: 에러는 마지막 반환값이며, nil이 아닌 경우 즉시 early return합니다.
func FindUser(id string) (*User, error) {
	if id == "" {
		return nil, fmt.Errorf("ID는 비어있을 수 없습니다: %w", errors.New("입력 검증 실패"))
	}
	if id == "notfound" {
		return nil, &UserNotFoundError{ID: id}
	}
	return &User{Name: "Alice", ID: id}, nil
}

// User는 단순한 데이터 구조체입니다.
// 패키지 설계 관례: exported 필드는 명확한 목적을 가져야 하며, 불필요한 공개 지양.
type User struct {
	ID   string
	Name string
}

// GetDisplayName은 nil 체크와 단순 조건문으로 가독성을 높입니다.
func GetDisplayName(u *User) string {
	if u == nil {
		return "Unknown User"
	}
	return u.Name
}

// main 함수: 관용구와 Best Practices를 종합적으로 데모합니다.
func main() {
	fmt.Println("=== Day 60: Go 관용구와 Best Practices 데모 ===")
	fmt.Println()

	// 1. 에러 처리 관례: errors.As를 사용한 타입 단언
	_, err := FindUser("notfound")
	if err != nil {
		var target *UserNotFoundError
		if errors.As(err, &target) {
			fmt.Printf("[1] 커스텀 에러 감지: %v\n", target.ID)
		} else {
			fmt.Printf("[1] 일반 에러: %v\n", err)
		}
	}

	// 2. early return 패턴: 중첩 없이 흐름을 명확히 유지
	users := []string{"alice123", "", "notfound"}
	for _, id := range users {
		user, err := FindUser(id)
		if err != nil {
			fmt.Printf("[2] early return 실패: %v\n", err)
			continue
		}
		fmt.Printf("[2] 성공: %s\n", GetDisplayName(user))
	}

	// 3. godoc 및 Go Proverbs 출력
	fmt.Println()
	fmt.Println("[3] Go Proverb: \"Clear is better than clever.\"")
	fmt.Println("[3] godoc: 주석은 코드의 문서화이자 API 계약입니다.")
	fmt.Println("[3] 코드 리뷰 체크리스트: 에러 처리 완전성, 인터페이스 최소화, 주석 가독성 확인")
}