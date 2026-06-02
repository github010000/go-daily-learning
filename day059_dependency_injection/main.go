package main

import "fmt"

// 1단계: 의존성 추상화 (인터페이스 정의)
// Go의 인터페이스는 암시적 구현을 지원하여, 구체적인 타입에 묶이지 않고 의존성을 정의할 수 있습니다.
type Storage interface {
	Get(key string) string
	Set(key string, value string)
}

// 2단계: 실제 프로덕션 구현체
type SQLiteStorage struct{}

func (SQLiteStorage) Get(key string) string {
	return fmt.Sprintf("[DB] '%s' 조회 성공", key)
}

func (SQLiteStorage) Set(key string, value string) {
	fmt.Printf("[DB] '%s' = '%s' 저장 완료\n", key, value)
}

// 3단계: 생성자 주입을 받는 서비스 레이어
type UserService struct {
	store Storage // 구체적인 구현체가 아닌 인터페이스를 의존성으로 보유
}

// 생성자 함수를 통해 외부에서 의존성을 주입받습니다.
// 이는 구조체 내부에서 직접 new()를 호출하는 의존성 수집(anti-pattern)을 피합니다.
func NewUserService(store Storage) *UserService {
	return &UserService{store: store}
}

func (u *UserService) GetUserProfile(name string) string {
	return u.store.Get(name)
}

func (u *UserService) UpdateProfile(name string, data string) {
	u.store.Set(name, data)
}

// 4단계: 테스트 더블 (Mock/Stub) 작성
// 단위 테스트에서 외부 시스템(DB, 외부 API 등)을 대체하여 테스트 안정성을 확보합니다.
type MockStorage struct {
	Data map[string]string
}

func (m *MockStorage) Get(key string) string {
	if val, ok := m.Data[key]; ok {
		return val
	}
	return "[MOCK] 데이터 없음"
}

func (m *MockStorage) Set(key string, value string) {
	m.Data[key] = value
}

func main() {
	// 5단계: wire 없이 수동 DI 구현 (수동 객체 그래프 구성)
	// main 함수에서 직접 의존성 관계를 연결하여 실행 흐름을 명확하게 추적 가능하게 합니다.

	// 프로덕션 환경: 실제 구현체 주입
	db := SQLiteStorage{}
	prodService := NewUserService(db)
	fmt.Println(prodService.GetUserProfile("user1"))
	prodService.UpdateProfile("user1", "홍길동")

	fmt.Println("--- 테스트 환경 분리 ---")

	// 테스트 환경: Mock 구현체 주입
	mockStore := &MockStorage{Data: make(map[string]string)}
	testService := NewUserService(mockStore)
	fmt.Println(testService.GetUserProfile("user1"))
	testService.UpdateProfile("user1", "테스트용_홍길동")
	fmt.Println("✅ 수동 DI 적용 완료: 생성자 주입 & Mock 테스트 더블 검증")
}