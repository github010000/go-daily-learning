package main

import (
	"fmt"
	"math"
)

func main() {
	// 1. 타입 추론 ( := 사용 시 컴파일러가 자동으로 타입 결정)
	name := "Go"          // string 타입으로 추론
	age := 25             // int 타입으로 추론
	pi := 3.14159         // float64 타입으로 추론 (기본 부동소수점 타입)
	isTrue := true        // bool 타입으로 추론

	fmt.Println("=== 타입 추론 예제 ===")
	fmt.Printf("name: %v (type: %T)\n", name, name)
	fmt.Printf("age: %v (type: %T)\n", age, age)
	fmt.Printf("pi: %v (type: %T)\n", pi, pi)
	fmt.Printf("isTrue: %v (type: %T)\n", isTrue, isTrue)

	// 2. 명시적 타입 변환: T(value) 형태로 변환
	fmt.Println("\n=== 명시적 타입 변환 예제 ===")
	var x int = 10
	var y float64 = float64(x) // int -> float64로 명시적 변환
	fmt.Printf("x (int): %d, y (float64): %.2f\n", x, y)

	// 3. 숫자형 간 변환 시 주의사항
	fmt.Println("\n=== 숫자형 간 변환 주의사항 ===")
	
	// 정수 -> 정수 변환 (오버플로우 가능)
	var small int8 = 120
	var big int8 = small + 20 // 오버플로우 발생! (정의되지 않은 동작)
	fmt.Printf("small (int8): %d\n", small)
	fmt.Printf("big (int8): %d (오버플로우 발생: 120 + 20 = 140, int8 최대 127)\n", big)

	// 실수 -> 정수 변환 시 소수점 버림
	var f float64 = 9.78
	var i int = int(f) // 소수점 이하 버림 (9로 변환)
	fmt.Printf("f (float64): %.2f -> i (int): %d (소수점 버림)\n", f, i)

	// 4. 정밀도 손실 주의 (큰 int를 float으로 변환 시)
	fmt.Println("\n=== 정밀도 손실 예시 ===")
	var bigInt int64 = 100000000000000007
	var asFloat float64 = float64(bigInt)
	fmt.Printf("bigInt (int64): %d\n", bigInt)
	fmt.Printf("asFloat (float64): %.0f (정밀도 손실 발생 가능성 있음)\n", asFloat)

	// 5. math 패키지 사용 예: 제곱근 계산 후 int로 변환
	fmt.Println("\n=== math 패키지 + 변환 예제 ===")
	num := 26.0
	sqrt := math.Sqrt(num)
	sqrtInt := int(sqrt) // 소수점 버림
	fmt.Printf("sqrt(%.1f) = %.4f -> int로 변환: %d\n", num, sqrt, sqrtInt)

	// 6. rune ↔ char ↔ string 변환 예시 (유니코드 코드포인트)
	fmt.Println("\n=== rune (int32) ↔ string 변환 예제 ===")
	var r rune = '한' // rune은 int32의 별칭
	fmt.Printf("rune r = %c (코드포인트: %d)\n", r, r)
	s := string(r)
	fmt.Printf("string(s) = %s\n", s)
}
