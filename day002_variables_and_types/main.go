package main

import (
	"fmt"
)

func main() {
	// 1. var 키워드를 사용한 변수 선언: 타입만 지정 (값은 제로값으로 초기화)
	var a int          // int 타입 제로값: 0
	var b float64      // float64 타입 제로값: 0.0
	var c bool         // bool 타입 제로값: false
	var d string       // string 타입 제로값: ""
	var e byte         // byte는 uint8의 별명, 제로값: 0
	var f rune         // rune은 int32의 별명, 제로값: 0 (Unicode code point)

	fmt.Println("=== var 키워드로 선언된 변수 (제로값) ===")
	fmt.Printf("a (int)     = %d (type: %T)\n", a, a)
	fmt.Printf("b (float64) = %.2f (type: %T)\n", b, b)
	fmt.Printf("c (bool)    = %v (type: %T)\n", c, c)
	fmt.Printf("d (string)  = %q (type: %T)\n", d, d)
	fmt.Printf("e (byte)    = %d (type: %T)\n", e, e)
	fmt.Printf("f (rune)    = %d (type: %T)\n", f, f)

	// 2. var 키워드로 타입과 함께 초기값 지정
	var x int = 42
	var y float64 = 3.14
	var z bool = true
	var s string = "Hello, Go!"

	fmt.Println("\n=== var 키워드로 초기화된 변수 ===")
	fmt.Printf("x = %d (type: %T)\n", x, x)
	fmt.Printf("y = %.2f (type: %T)\n", y, y)
	fmt.Printf("z = %v (type: %T)\n", z, z)
	fmt.Printf("s = %q (type: %T)\n", s, s)

	// 3. := 단축 변수 선언 (함수 내에서만 사용 가능)
	// 타입 추론을 통해 자동으로 타입 결정 (타입 생략 가능)
	name := "Alice"
	age := 30
	height := 168.5
	isActive := true

	fmt.Println("\n=== := 단축 변수 선언 ===")
	fmt.Printf("name     = %q (type: %T)\n", name, name)
	fmt.Printf("age      = %d (type: %T)\n", age, age)
	fmt.Printf("height   = %.1f (type: %T)\n", height, height)
	fmt.Printf("isActive = %v (type: %T)\n", isActive, isActive)

	// 4. rune과 byte의 활용 예시: Unicode 문자 처리
	r1 := '한'  // rune 타입 (int32)
	b1 := byte('A') // byte 타입 (uint8)
	fmt.Println("\n=== rune과 byte 비교 ===")
	fmt.Printf("rune r1 = %d ('%c')\n", r1, r1)
	fmt.Printf("byte b1 = %d ('%c')\n", b1, b1)

	// 5. 타입 변환: string → []rune (UTF-8 다국어 처리에 중요)
	text := "Go언어"
	runes := []rune(text)
	fmt.Println("\n=== string을 rune slice로 변환 ===")
	fmt.Printf("original string: %q\n", text)
	fmt.Printf("rune count: %d\n", len(runes))
	for i, r := range runes {
		fmt.Printf("  rune[%d] = %d ('%c')\n", i, r, r)
	}

	// 6. bool 타입의 다양한 비교 및 논리 연산
	fmt.Println("\n=== bool 타입 활용 예 ===")
	p, q := true, false
	fmt.Printf("p && q = %v\n", p && q) // AND
	fmt.Printf("p || q = %v\n", p || q) // OR
	fmt.Printf("!p     = %v\n", !p)     // NOT

	// 7. 제로값의 실용성: 조건문에서의 활용
	var optionalName string // 제로값: ""
	if optionalName == "" {
		fmt.Println("\noptionalName이 비어 있습니다. (제로값)")
	}
	optionalName = "Bob"
	if optionalName != "" {
		fmt.Printf("optionalName: %q\n", optionalName)
	}
}