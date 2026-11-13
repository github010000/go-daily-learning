package main

import (
	"fmt"
	"strconv"
)

// ---------- 도메인 타입 ----------

// Value 는 문자열 표현을 제공하는 인터페이스입니다.
type Value interface {
	String() string
}

// StringVal 은 텍스트 데이터를 담는 구조체입니다.
type StringVal struct {
	V string
}

func (s StringVal) String() string { return s.V }

// IntVal 은 정수 데이터를 담는 구조체입니다.
type IntVal struct {
	V int
}

func (i IntVal) String() string { return strconv.Itoa(i.V) }

// ---------- 함수 1: 타입 단언 ----------

// typeAssertDemo 은 타입 단언의 올바른 사용법과 흔한 실수를 보여줍니다.
func typeAssertDemo() {
	fmt.Println("=== 1. 타입 단언(Type Assertion) ===")

	// 1.1 올바른 사용법: comma-ok 패턴
	// interface{} 안에 든 실제 타입이 StringVal 이라 가정하고 꺼내려 합니다.
	fmt.Println("\n[올바른 사용법: comma-ok]")
	var unknown interface{} = StringVal{V: "Hello Go"}

	// 두 번째 반환값 ok 가 true 일 때만 값이 유효합니다.
	// 만약 unknown 이 StringVal 이 아니라면? ok 는 false 가 되고,
	// sv 의 값은 해당 타입의 영값(영문자열의 경우 "")이 됩니다.
	// 이 조건 분기를 생략하면 런타임 panic 이 터집니다.
	if sv, ok := unknown.(StringVal); ok {
		fmt.Printf("성공: StringVal 타입, 내용 = %q\n", sv.V)
	} else {
		fmt.Println("실패: StringVal 이 아님")
	}

	// 1.2 타입이 맞지 않을 때
	fmt.Println("\n[타입 불일치 처리]")
	var another interface{} = IntVal{V: 100}
	if _, ok := another.(StringVal); ok {
		fmt.Println("아니, 이건 StringVal 이 아닙니다!")
	} else {
		fmt.Println("성공: IntVal 을 StringVal 로 단언하면 실패합니다 (ok=false)")
	}
}

// panicOnWrongTypeAssert 는 타입을 확인하지 않은 채 무조건 단언하는 실수를 시연합니다.
// 이 함수는 panic 을 일으킨 후 recover 로 잡히지 않으면 프로그램이 종료됩니다.
// main 에서 defer/recover 로 감싸서 안전하게 보여줍니다.
func panicOnWrongTypeAssert(val interface{}) {
	// val 이 IntVal 이라고 단정하고 무조건 꺼냅니다.
	// 만약 val 이 StringVal 이라면 여기서 런타임 panic 이 발생합니다.
	// 컴파일은 되지만, 실행 시에야 문제가 들통 납니다.
	iv := val.(IntVal)
	fmt.Printf("실수 버전: %d 를 꺼냈다고 믿었습니다.\n", iv.V)
}

// ---------- 함수 2: 타입 스위치 ----------

// typeSwitchDemo 은 타입 스위치를 사용해 여러 타입을 한 번에 처리합니다.
func typeSwitchDemo() {
	fmt.Println("\n=== 2. 타입 스위치(Type Switch) ===")

	// interface{} 슬라이스: 다양한 타입이 섞여 있습니다.
	data := []interface{}{
		StringVal{V: "Go"},
		IntVal{V: 123},
		42.5,          // float64 (표준 float 리터럴은 float64)
		"plain string", // string 리터럴
	}

	for _, val := range data {
		// val.(type) 은 타입 스위치 전용 구문입니다.
		// 각 케이스의 v 는 해당 타입으로 좁혀진 변수가 됩니다.
		switch v := val.(type) {
		case StringVal:
			// v.V 로 직접 접근 가능합니다 (타입이 좁혀졌기 때문).
			fmt.Printf("StringVal 타입: %s\n", v.V)
		case IntVal:
			fmt.Printf("IntVal 타입: %d\n", v.V)
		case float64:
			fmt.Printf("float64 타입: %.1f\n", v)
		case string:
			fmt.Printf("string 타입: %s\n", v)
		default:
			// 예상 못한 타입이 들어오면 여기로 옵니다.
			// %T 로 실제 동적 타입을 확인할 수 있습니다.
			fmt.Printf("알 수 없는 타입 (%T): %v\n", v, v)
		}
	}
}

// ---------- 함수 3: 실수 vs 고침 (슬라이스 복사) ----------

// copySliceUnsafe 는 원본 슬라이스를 직접 반환합니다.
// 호출자가 원본을 수정하면 원본도 함께 바뀝니다.
func copySliceUnsafe(src []int) []int {
	return src // 참조만 복사, 데이터는 공유
}

// copySliceSafe 는 원본과 독립적인 새 슬라이스를 반환합니다.
// 호출자가 반환값을 수정해도 원본에는 영향을 주지 않습니다.
func copySliceSafe(src []int) []int {
	dst := make([]int, len(src))
	copy(dst, src)
	return dst
}

// sliceCopyDemo 는 슬라이스 복사의 올바른 사용법을 보여줍니다.
func sliceCopyDemo() {
	fmt.Println("\n=== 3. 슬라이스 복사 (보조 주제) ===")

	original := []int{1, 2, 3}
	fmt.Printf("원본: %v\n", original)

	unsafe := copySliceUnsafe(original)
	unsafe[0] = 999
	fmt.Printf("unsafe 수정 후 -> 원본: %v, unsafe: %v\n", original, unsafe)
	// ↑ 원본이 함께 바뀐다! 참조 타입이기 때문입니다.

	// 원본을 복원
	original = []int{1, 2, 3}

	safe := copySliceSafe(original)
	safe[0] = 999
	fmt.Printf("safe 수정 후 -> 원본: %v, safe: %v\n", original, safe)
	// ↑ 원본은 그대로, safe 만 바뀐다. 독립적인 복사이기 때문입니다.
}

// ---------- main ----------

func main() {
	// 1. 타입 단언 기본 실습
	typeAssertDemo()

	// 2. 타입 스위치 실습
	typeSwitchDemo()

	// 3. 슬라이스 복사 실습
	sliceCopyDemo()

	// 4. 실수 시연: panic 발생 (recover 로 안전히 잡음)
	fmt.Println("\n=== 4. 실수 시연: 타입 단언 실수 ===")

	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("실수 버전에서 panic 발생: %v\n", r)
			fmt.Println("(타입 단언을 할 때 타입을 확인하지 않아 발생합니다.)")
		}
	}()

	// StringVal 을 IntVal 로 단언하면 panic 이 발생합니다.
	panicOnWrongTypeAssert(StringVal{V: "Not an Int"})

	// 위 panic 이 recover 되면 이 줄도 실행됩니다.
	fmt.Println("위 panic 은 recover 되었으므로 이 줄은 실행됩니다.")
}