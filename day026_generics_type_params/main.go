package main

import "fmt"

// 1. any 제약: 모든 타입을 허용하는 기본 제네릭 함수
func PrintAny[T any](value T) {
	fmt.Printf("[any] 타입: %T, 값: %v\n", value, value)
}

// 2. 커스텀 타입 제약: ~ 기호는 원시 타입과 그 하위 타입을 모두 허용
type Number interface {
	~int | ~int64 | ~float64
}

// Number 제약이 적용된 제네릭 함수
func Sum[T Number](a, b T) T {
	return a + b
}

// 3. comparable 제약: ==, != 연산이 가능한 타입만 허용
func FindIndex[T comparable](slice []T, target T) int {
	for i, v := range slice {
		if v == target {
			return i
		}
	}
	return -1
}

// 인터페이스를 제약 조건으로 사용하는 제네릭 함수
type Stringer interface {
	String() string
}

func GenericDescribe[T Stringer](s T) string {
	return s.String()
}

func main() {
	fmt.Println("=== 1. any 제약 예시 ===")
	PrintAny(42)
	PrintAny("Go 언어")
	PrintAny(true)

	fmt.Println("\n=== 2. Number 타입 제약 예시 ===")
	fmt.Printf("정수 합계: %v\n", Sum(10, 20))
	fmt.Printf("실수 합계: %v\n", Sum(10.5, 3.14))

	fmt.Println("\n=== 3. comparable 제약 예시 ===")
	fmt.Printf("문자열 위치: %d\n", FindIndex([]string{"A", "B", "C"}, "B"))
	fmt.Printf("정수 위치: %d\n", FindIndex([]int{100, 200, 300}, 200))

	fmt.Println("\n=== 4. 제네릭 vs 인터페이스 선택 기준 ===")
	fmt.Println("타입이 명확하고 연산/비교가 필요하면 제네릭을 사용하세요.")
	fmt.Println("행동만 규정하고 타입은 유동적이라면 인터페이스를 사용하세요.")
}