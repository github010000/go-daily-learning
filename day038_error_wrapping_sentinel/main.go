package main

import (
	"errors"
	"fmt"
)

// 1. 센티널 에러 (Sentinel Errors) 정의
// 애플리케이션 전반에서 특정 상태를 나타내기 위해 정의된 전역 변수입니다.
var (
	ErrNotFound    = errors.New("리소스를 찾을 수 없습니다")
	ErrInvalidData = errors.New("데이터 형식이 잘못되었습니다")
)

// 2. 커스텀 에러 타입 정의
// 기존 에러를 감싸거나, 추가 정보(예: 코드)를 포함하여 에러를 확장합니다.
type AppError struct {
	Code    int
	Message string
	Err     error // 내부 에러
}

// Error 메서드를 구현하여 error 인터페이스를 만족시킵니다.
func (e *AppError) Error() string {
	return fmt.Sprintf("[%d] %s (내부 에러: %v)", e.Code, e.Message, e.Err)
}

// Unwrap 메서드를 구현하여 에러 체이닝(연결)을 지원합니다.
// Go 1.13 이상에서 errors.Is 및 errors.As가 내부 에러를 따라가게 하는 핵심입니다.
func (e *AppError) Unwrap() error {
	return e.Err
}

// --- 레이어별 함수들 ---

// 레이어 1: 데이터 접근 계층 (저수준)
func findUser(id string) error {
	if id == "unknown" {
		// 센티널 에러 반환
		return ErrNotFound
	}
	return nil
}

// 레이어 2: 비즈니스 로직 계층 (중간 수준)
func getUserProfile(id string) error {
	// %w 지시어를 사용하여 에러를 감쌉니다 (Wrapping).
	// 이는 에러를 감싸는 "새로운" 에러를 생성하지만, 기존 에러와의 연결고리를 유지합니다.
	err := findUser(id)
	if err != nil {
		// 에러 메시지 추가 및 감싸기
		return fmt.Errorf("사용자 프로필 조회 실패: %w", err)
	}
	return nil
}

// 레이어 3: API/서비스 계층 (고수준)
func handleRequest(id string) {
	// 비즈니스 로직 호출
	err := getUserProfile(id)
	if err != nil {
		// 에러가 우리가 정의한 센티널 에러인지를 확인합니다.
		// errors.Is는 에러 체인을 따라가면서 타겟 에러가 있는지 찾아줍니다.
		if errors.Is(err, ErrNotFound) {
			fmt.Printf("✅ API 응답: 404 Not Found (에러: %v)\n", err)
			return
		}

		// 에러를 커스텀 타입으로 변환하여 추가 정보를 얻어냅니다.
		// errors.As는 에러 체인을 탐색하며, 만약 에러가 AppError 타입이거나 그 타입을 감싸고 있다면 변환합니다.
		var appErr *AppError
		if errors.As(err, &appErr) {
			// 만약 에러가 AppError 타입이라면 코드와 메시지를 출력할 수 있습니다.
			fmt.Printf("✅ API 응답: %d Error Code (에러: %v)\n", appErr.Code, err)
		} else {
			// 예상치 못한 에러 발생
			fmt.Printf("❌ API 응답: 500 Internal Server Error (에러: %v)\n", err)
		}
	}
}

func main() {
	fmt.Println("=== 에러 래핑 및 센티널 에러 데모 ===\n")

	// 성공 케이스
	fmt.Println("1. 성공 케이스 (존재하는 ID):")
	handleRequest("valid_id")

	fmt.Println("\n2. 실패 케이스 (존재하지 않는 ID):")
	// findUser에서 ErrNotFound가 반환되고, getUserProfile에서 %w로 감싸지고,
	// handleRequest에서 errors.Is를 통해 ErrNotFound를 감지합니다.
	handleRequest("unknown")
}