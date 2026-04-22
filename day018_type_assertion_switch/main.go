package main

import (
	"fmt"
)

func main() {
	// interface{} 타입은 어떤 타입의 값도 저장할 수 있습니다.
	// 타입 단언은 이 interface{} 타입 안에 숨겨진 실제 타입을 추출하는 과정입니다.
	var i interface{} = "Go Programming"

	// 1. 기본 타입 단언 (Type Assertion)
	// i를 string 타입으로 단언합니다.
	// 만약 i가 string이 아니라면 이 코드는 런타임 패닉(panic)을 발생시💻합니다.
	str := i.(string)
	fmt.Printf("1. 기본 단언 성공: %v (타입: %T)\n", str, str)

	// 2. 안전한 타입 단언 (Comma-ok idiom)
	// 단언 결과와 함께 두 번째 반환값(ok)을 받습니다.
	// ok가 true이면 단언 성공, false이면 단언 실패입니다.
	// 패닉을 방지할 수 있는 가장 권장되는 방법입니다.
	fmt.Println("2. 안전한 타입 단언 시도:")
	
	// 성공 케이스
	val, ok := i.(string)
	if ok {
		fmt.Printf("   - 성공: %v는 문자열입니다.\n", val)
	}

	// 실패 케이스 (int로 단언 시도)
	num, ok := i.(int)
	if !ok {
		fmt.Println("   - 실패: 해당 값은 int 타입이 아닙니다.")
	}
	_ = num // 사용하지 않는 변수 에러 방지

	// 3. 타입 스위치 (Type Switch)
	// interface{}에 담긴 값이 어떤 타입인지 여러 가지 경우에 따라 분기 처리할 때 사용합니다.
	// switch v := i.(type) 구문을 사용하며, v는 각 case의 타입으로 자동 변환됩니다.
	fmt.ly.Println("3. 타입 스위치 실행:")
	printInterfaceValue(i)
	printInterfaceValue(42)
	printInterfaceValue(true)
	printInterfaceValue(3.14)
}

// printInterfaceValue는 interface{}를 받아 내부 타입에 따라 다른 동작을 수행합니다.
func printInterfaceValue(i interface{}) {
	switch v := i.(type) {
	case string:
		// 여기서 v는 string 타입으로 취급됩니다.
		fmt.Printf("   [문자열] 내용: %s, 길이: %d\n", v, len(v))
	case int:
		// 여기서 v는 int 타입으로 취급됩니다.
		fmt.Printf("   [정수] 값: %d, 2배: %d\n", v, v*2)
	case bool:
		// 여기서 v는 bool 타입으로 취급됩니다.
		fmt.Printf("   [불리언] 값: %t\n", v)
	case float64:
		fmt.Printf("   [실수] 값: %f\n", v)
	default:
		// 정의되지 않은 모든 타입은 여기서 처리됩니다.
		fmt.Printf("   [알 수 없음] 타입: %T\n", v)
	}
}