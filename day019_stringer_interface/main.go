package main

import (
	"fmt"
)

// Person 구조체는 Stringer 인터페이스를 구현하지 않았습니다.
// 이 경우 fmt 패키지는 구조체의 필드 값을 기본 형식으로 출력합니다.
type Person struct {
	Name string
	Age  int
}

// Product 구조체는 Stringer 인터페이스를 구현합니다.
type Product struct {
	ID    int
	Name  string
	Price int
}

// String 메서드는 fmt.Stringer 인터페이스의 핵심 메서드입니다.
// 이 메서드를 구현함으로써 Product 구조체의 출력 형식을 직접 정의할 수 있습니다.
func (p Product) String() string {
	return fmt.Sprintf("[상품 ID: %d] 이름: %s | 가격: %d원", p.ID, p.Name, p.Price)
}

// Admin 구조체는 보안을 위해 정보를 숨기기 위해 Stringer를 구현합니다.
type Admin struct {
	Username string
	Password string // 민감한 정보
}

// String 메서드를 통해 Password는 출력되지 않도록 커스텀 출력 로직을 작성합니다.
func (a Admin) String() string {
	return fmt.Sprintf("관리자: %s (비밀번호 숨김)", a.Username)
}

func main() {
	// 1. Stringer를 구현하지 않은 구조체 출력
	person := Person{Name: "홍길동", Age: 30}
	fmt.Println("--- 1. Stringer 미구현 (기본 출력) ---")
	fmt.Printf("Person 구조체: %+v\n", person) // %+v는 필드명까지 출력하지만 기본 포맷을 따름

	// 2. Stringer를 구현한 구조체 출력
	product := Product{ID: 101, Name: "Go 언어 마스터 가이드", Price: 25000}
	fmt.Println("\n--- 2. Stringer 구현 (커스텀 출력) ---")
	// fmt.Println 내부에서 자동으로 product.String()을 호출합니다.
	fmt.Println("Product 객체:", product)

	// 3. 슬라이스 내부의 객체 출력
	// 슬라이스 내의 요소가 Stringer를 구현했다면, 반복문 출력 시 커스텀 포맷이 적용됩니다.
	products := []Product{
		{ID: 1, Name: "키보드", Price: 50000},
		{ID: 2, Name: "마우스", Price: 30000},
	}
	fmt.Println("\n--- 3. Stringer 구현 슬라이스 출력 ---")
	for _, p := range products {
		fmt.Println(p)
	}

	// 4. 보안을 위한 Stringer 활용
	admin := Admin{Username: "super_admin", Password: "secret_password_1234"}
	fmt.
		Println("\n--- 4. 보안을 위한 Stringer 활용 ---")
	fmt.Println("Admin 객체:", admin) // Password가 노출되지 않음
}