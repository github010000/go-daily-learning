package main

import (
	"fmt"
	"strings"
)

// ---------------------------------------------------------------------------
// 1. 가변 인자 (Variadic Parameter)
//
// Go에서 ...T로 선언된 파라미터는 함수 내부에서 T 슬라이스([]T)로 취급됩니다.
// 호출 시에는 개별 인자를 나열하거나, 슬라이스를 '...' 연산자로 펼쳐 전달할 수 있습니다.
// ---------------------------------------------------------------------------

// sumInts는 가변 인자로 받은 정수들의 합을 계산합니다.
// 내부적으로 values는 []int 슬라이스입니다.
func sumInts(values ...int) int {
	total := 0
	for _, v := range values {
		total += v
	}
	return total
}

// maxInts는 가변 인자로 받은 정수들 중 최댓값을 반환합니다.
func maxInts(values ...int) int {
	if len(values) == 0 {
		return 0
	}
	max := values[0]
	for _, v := range values[1:] {
		if v > max {
			max = v
		}
	}
	return max
}

// ---------------------------------------------------------------------------
// 2. 함수 값 (Function Value)과 전략 패턴
//
// Go에서 함수는 일급 객체입니다. 변수에 저장하고, 함수의 파라미터로 전달하고,
// 반환값으로 사용할 수 있습니다. 이를 이용해 "무엇을 할지"가 아닌 "어떻게 할지"를
// 호출 시점에 결정하는 전략 패턴(strategy pattern)을 구현할 수 있습니다.
// ---------------------------------------------------------------------------

// processItems는 문자열 슬라이스와 "처리 전략" 함수를 받아 전략을 적용합니다.
// strategy는 []string과 separator를 받아 최종 문자열을 반환해야 합니다.
func processItems(items []string, strategy func([]string, string) string, separator string) string {
	return strategy(items, separator)
}

// joinStrategy는 인자들을 separator로 이어붙이는 전략 함수입니다.
func joinStrategy(items []string, sep string) string {
	return strings.Join(items, sep)
}

// upperJoinStrategy는 각 요소를 대문자로 변환한 뒤 separator로 이어붙이는 전략 함수입니다.
// 이 함수는 원본 슬라이스(items)를 직접 수정합니다(부수 효과).
func upperJoinStrategy(items []string, sep string) string {
	for i := range items {
		items[i] = strings.ToUpper(items[i])
	}
	return strings.Join(items, sep)
}

// ---------------------------------------------------------------------------
// 시연 및 실수 예시
// ---------------------------------------------------------------------------

func main() {
	showVariadicBasics()
	showSliceSpreading()
	showFunctionValues()
	showCommonMistakes()
}

// showVariadicBasics는 가변 인자의 기본 사용법을 보여줍니다.
func showVariadicBasics() {
	fmt.Println("=== 1. 가변 인자 기본 사용 ===")

	// 개별 인자를 나열하여 호출합니다.
	sum := sumInts(1, 2, 3, 4, 5)
	fmt.Printf("sumInts(1, 2, 3, 4, 5) = %d\n", sum)

	// 인자가 없어도 호출 가능합니다 (빈 슬라이스 전달).
	empty := sumInts()
	fmt.Printf("sumInts() = %d\n", empty)

	// 최댓값 찾기
	maxVal := maxInts(3, 1, 4, 1, 5, 9, 2, 6)
	fmt.Printf("maxInts(3, 1, 4, 1, 5, 9, 2, 6) = %d\n", maxVal)
}

// showSliceSpreading는 슬라이스를 가변 인자 함수에 전달할 때 '...'를 사용하는법을 보여줍니다.
// 여기서 「실수 버전」과 「고친 버전」의 차이를 출력으로 확인합니다.
func showSliceSpreading() {
	fmt.Println()
	fmt.Println("=== 2. 슬라이스 펼치기 (Spreading) ===")

	numbers := []int{10, 20, 30}

	// 고친 버전: '...' 연산자로 슬라이스를 개별 원소로 펼쳐 전달합니다.
	sumSpread := sumInts(numbers...)
	fmt.Printf("고친 버전: sumInts(numbers...) = %d\n", sumSpread)

	// 실수 버전: '...' 없이 슬라이스를 그대로 전달합니다.
	// 이 코드는 컴파일되지 않습니다. 실제 실행에서는 볼 수 없으니,
	// 「컴파일 오류 발생」 메시지를 출력하여 차이를 보여줍니다.
	fmt.Println()
	fmt.Println("실수 버전: sumInts(numbers) -> 컴파일 오류")
	fmt.Println("  오류 메시지:")
	fmt.Println(`  ./main.go:XX:XX: cannot use numbers (type []int) as type []int in argument to sumInts (missing ... in parameter list)`)
	fmt.Println("  이유: ...int 파라미터는 'int를 여러 개' 받는 것이므로, []int 하나를 넣을 수 없습니다.")
	fmt.Println("  해결: numbers... 로 펼쳐야 합니다.")
}

// showFunctionValues는 함수 값을 파라미터로 전달하는 전략 패턴을 보여줍니다.
func showFunctionValues() {
	fmt.Println()
	fmt.Println("=== 3. 함수 값과 전략 패턴 ===")

	items := []string{"apple", "banana", "cherry"}

	// 1. 함수를 변수에 할당합니다.
	strategy := joinStrategy

	// 2. 함수 변수를 다른 함수의 파라미터로 전달합니다 (콜백).
	result1 := processItems(items, strategy, " | ")
	fmt.Printf("joinStrategy 적용: %s\n", result1)

	// 3. 다른 전략 함수를 전달합니다.
	result2 := processItems(items, upperJoinStrategy, " - ")
	fmt.Printf("upperJoinStrategy 적용: %s\n", result2)

	// 4. 익명 함수(람다)를 직접 전달합니다.
	result3 := processItems(items, func(items []string, sep string) string {
		return strings.Join(items, sep) + " (익명)"
	}, " + ")
	fmt.Printf("익명 함수 적용: %s\n", result3)

	// 5. 실수 버전: 함수 타입 불일치
	fmt.Println()
	fmt.Println("실수 버전: 시그니처 불일치 함수 전달 -> 컴파일 오류")
	fmt.Println("  오류 메시지:")
	fmt.Println(`  ./main.go:XX:XX: cannot use badStrategy (type func([]string) string) as type func([]string, string) string`)
	fmt.Println("  이유: processItems는 전략 함수에 separator를 전달해야 하므로, separator 파라미터가 필요합니다.")
}

// showCommonMistakes는 가변 인자와 함수 값에서 자주 하는 실수들을 설명합니다.
func showCommonMistakes() {
	fmt.Println()
	fmt.Println("=== 4. 흔한 실수 및 주의사항 ===")

	fmt.Println("실수 1: 슬라이스를 ... 없이 가변 인자 함수에 전달")
	fmt.Println("  // sumInts(numbers) -> 컴파일 오류")
	fmt.Println("  // 반드시 numbers... 로 펼치세요.")

	fmt.Println()
	fmt.Println("실수 2: 함수 값의 시그니처를 잘못 선언")
	fmt.Println("  // strategy := func(items []string) string { ... }")
	fmt.Println("  // processItems(items, strategy, \" - \") -> 컴파일 오류")
	fmt.Println("  // processItems는 func([]string, string) string 타입을 기대합니다.")

	fmt.Println()
	fmt.Println("실수 3: 가변 인자 함수에 빈 슬라이스를 전달할 때 ... 생략")
	fmt.Println("  // var nums []int")
	fmt.Println("  // sumInts(nums) -> 컴파일 오류 (빈 슬라이스도 ... 필요)")
	fmt.Println("  // sumInts(nums...) -> 정상 (빈 슬라이스 전달)")
}