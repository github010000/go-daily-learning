package main

import (
	"fmt"
)

// Day 4: 상수와 iota
// 상수(const)는 컴파일 시점에 값이 결정되며, runtime에 변경할 수 없음
// iota는 const 블록 내에서 자동으로 증가하는 정수 상수 생성자

// 타입이 명시되지 않은 상수: 기본 타입 없음 → 숫자형 리터럴처럼 임의의 정밀도 가능
const UntypedConst = 42           // UntypedConst는 untyped integer
const UntypedFloatConst = 3.14    // untyped float

// 타입이 명시된 상수: 해당 타입으로 고정
const TypedIntConst int = 100
const TypedStringConst string = "Hello"

// iota를 사용한 열거형 패턴 (Go에는 enum 키워드 없음 → const + iota 패턴 사용)
// 각 상수는 iota에 의해 0, 1, 2, ... 순으로 할당됨
const (
	Red iota   // 0
	Green      // 1
	Blue       // 2
)

// iota는 const 블록에서만 유효하며, 블록 시작 시 0, 이후 줄마다 1씩 증가
// 블록 간에는 재설정됨 (다른 const 블록에서는 다시 0부터 시작)
const (
	Monday iota  // 0
	Tuesday      // 1
	Wednesday    // 2
	Thursday     // 3
	Friday       // 4
)

// 복잡한 패턴: iota와 비트 시프트 조합 (단위 표현 등에 유용)
// 1, 2, 4, 8, 16 ... (2^n)
const (
	KB = 1 << (10 * iota) // 1 * 2^0 = 1
	MB                     // 1 * 2^10 = 1024
	GB                     // 1 * 2^20 = 1048576
	TB                     // 1 * 2^30 = 1073741824
)

// 이름이 있는 타입으로 상수 정의 (타입 안전성 강화 가능)
type Color int

const (
	Purple Color = iota + 5 // 5부터 시작
	Orange                  // 6
	Pink                    // 7
)

func main() {
	// Untyped constants는 다른 숫자 타입에 자연스럽게 캐스팅 가능
	var n int64 = UntypedConst
	var f float64 = UntypedFloatConst
	fmt.Printf("[1] Untyped constants → int64: %d, float64: %.2f\n", n, f)

	// Typed constants는 정확한 타입으로만 할당 가능
	// var wrongType float64 = TypedIntConst // ❌ compile error
	var typedInt int = TypedIntConst // ✅ ok
	var typedStr string = TypedStringConst
	fmt.Printf("[2] Typed constants → int: %d, string: %s\n", typedInt, typedStr)

	// iota로 정의된 색상 열거형
	fmt.Printf("[3] Color enum (Red, Green, Blue): Red=%d, Green=%d, Blue=%d\n", Red, Green, Blue)

	// 요일 열거형
	fmt.Printf("[4] Days of week (Monday~Friday): Monday=%d, Wednesday=%d, Friday=%d\n", Monday, Wednesday, Friday)

	// iota와 비트 시프트 조합을 통한 단위 변환
	fmt.Printf("[5] Size units (KB, MB, GB, TB): KB=%d, MB=%d, GB=%d, TB=%d\n", KB, MB, GB, TB)

	// Custom type + iota offset
	fmt.Printf("[6] Custom Color type (offset 5): Purple=%d, Orange=%d, Pink=%d\n", Purple, Orange, Pink)

	// iota는 const 블록 시작 시 0, 이후 줄마다 증가
	// 같은 블록 내 같은 상수 표현식에서 반복 사용 가능
	const (
		a = iota // 0
		b        // 1
		c = iota // 2 (explicit use 가능)
	)
	fmt.Printf("[7] iota 사용 패턴 확장: a=%d, b=%d, c=%d\n", a, b, c)

	// 블록 간 iota는 재설정됨
	const (
		x = iota // 0 (새 블록 시작 → 다시 0)
		y        // 1
	)
	fmt.Printf("[8] 블록 재시작 시 iota 리셋: x=%d, y=%d\n", x, y)
}
