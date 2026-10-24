package main

import (
	"fmt"
)

// main: var vs :=, 기본 타입, 제로값, 쓰지 않는 변수(오류), 문자열 불변성 시연
func main() {
	showVarVsAssign()
	showZeroValues()
	showStringImmutable()
}

// showVarVsAssign: var 선언과 := 단축 할당의 차이와 타입 추론 행동을 보여준다
func showVarVsAssign() {
	fmt.Println("=== var vs := ===")

	// := 단축 할당은 '선언+초기화'를 한 번에 한다.
	// 컴파일러가 우변의 타입을 추론해서 변수 타입을 결정한다.
	name := "고고"
	fmt.Println("단축 할당(name):", name, "- 타입:", fmt.Sprintf("%T", name))

	// var 로 선언하면 타입을 명시적으로 쓸 수 있고, 타입 추론에 맡기지 않는다.
	var age int = 10
	fmt.Println("var 선언(age):", age, "- 타입:", fmt.Sprintf("%T", age))

	// var 선언 시 초기값을 생략하면 제로값(0)이 들어간다.
	var score float64
	fmt.Println("var 선언(score, 초기값 없음):", score, "- 타입:", fmt.Sprintf("%T", score))

	// := 은 같은 패키지 내에서 같은 변수를 재선언하지 않는다(오류).
	// var 은 재할당(대입)은 허용된다.
	name = "바보" // 재할당은 허용
	fmt.Println("재할당 후 name:", name)

	fmt.Println()
}

// showZeroValues: 초기화하지 않은 변수의 제로값과, '쓰지 않는 변수' 컴파일 오류를 설명한다
func showZeroValues() {
	fmt.Println("=== 제로값과 쓰지 않는 변수 ===")

	// Go 에서 변수는 반드시 초기화된다. 초기화하지 않아도 제로값이 들어간다.
	var i int
	var b bool
	var s string
	var f float64

	fmt.Printf("int 제로값: %d\n", i)
	fmt.Printf("bool 제로값: %v\n", b)
	fmt.Printf("string 제로값: %q\n", s)
	fmt.Printf("float64 제로값: %f\n", f)

	// Go 는 '선언하고 쓰지 않는 변수'를 컴파일 오류로 만든다.
	// 이유: 실수로 변수를 선언만 하고 사용하는 것을 막아 버그를 줄이기 위함.
	// 다른 언어(예: Python)는 경고만 하거나 무시하므로, Go 초보자에게는 낯설다.
	//
	// // 틀린 코드 (컴파일 오류: i declared and not used)
	// var unused int
	// fmt.Println("이 아래 코드는 컴파일 오류가 난다")

	// 쓰지 않는 변수를 막는 방법: _ (blank identifier) 사용
	var _ = 42
	fmt.Println("쓰지 않는 값을 _ 에 버리기로 선언:", 42)

	fmt.Println()
}

// showStringImmutable: string 은 불변(immutable) 타입임을 보여준다
func showStringImmutable() {
	fmt.Println("=== string 불변성 ===")

	s := "hello"
	fmt.Println("원본 문자열 s:", s)

	// string 은 불변: s[0] = 'H' 는 컴파일 오류다.
	// Go 에서 문자열 리터럴은 읽기 전용 바이트 배열처럼 취급된다.
	// 문자열을 바꾸려면 []byte 나 []rune 으로 변환한 뒤 조작해야 한다.
	fmt.Println("string s 의 첫 번째 바이트(읽기만 가능):", s[0])

	// 실수: string 에 인덱스로 대입하려고 하면?
	// // 틀린 코드
	// s[0] = 'H' // cannot assign to s[0]
	fmt.Println("string 은 인덱스로 대입 불가: 컴파일 오류가 난다")

	// 고친 방법: []byte 로 변환해서 조작 후 다시 string 으로 반환
	b := []byte(s)
	b[0] = 'H'
	newS := string(b)
	fmt.Println("고친 버전: []byte 변환 후 조작 →", newS)
	fmt.Println("원본 s 는 여전히 변경되지 않음:", s)
}