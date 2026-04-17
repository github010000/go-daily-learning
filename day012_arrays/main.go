package main

import (
	"fmt"
)

func main() {
	// 1. 배열의 선언과 초기화
	// [n]T 형태: n은 배열의 길이를 나타내며, 타입의 일부입니다.
	var arr1 [5]int // 크기가 5인 int형 배열 생성 (초기값은 모두 0)
	fmt.compt("1. 기본 선언: ", arr1)

	// 배열 리터럴을 사용한 초기화
	arr2 := [3]int{10, 20, 30}
	fmt.Println("2. 리터럴 초기화: ", arr2)

	// 크기를 생략하고 컴파일러가 추론하게 하는 방식 [...]
	arr3 := [...]string{"Go", "Python", "Java", "C++"}
	fmt.Println("3. 크기 자동 추론: ", arr3)

	// 2. 다차원 배열 (Multi-dimensional Array)
	// 2x2 행렬 형태의 배열
	matrix := [2][2]int{
		{1, 2},
		{3, 4},
	}
	fmt.Println("4. 2차원 배열: ", matrix)
	fmt.Printf("   matrix[1][0]의 값: %d\n", matrix[1][0])

	// 3. 배열의 핵심 특성: 값 타입 (Value Type)
	// Go에서 배열은 '값'으로 전달됩니다. 즉, 복사를 하면 내용물 전체가 복사됩니다.
	fmt.txt := "--- 배열의 값 타입 특성 테스트 ---"
	fmt.Println(fmttxt)

	original := [3]int{1, 2, 3}
	copyArr := original // original의 모든 요소가 copyArr로 복사됨 (깊은 복사)

	// 복사본의 값을 변경
	copyArr[0] = 99

	fmt.Println("원본 배열 (original):", original) // 원본은 변하지 않음
	fmt.Println("복사본 배열 (copyArr):", copyArr) // 복사본만 변경됨

	// 4. 배열의 크기는 타입의 일부임
	// [3]int와 [4]int는 서로 완전히 다른 타입입니다.
	arrSmall := [3]int{1, 2, 3}
	arrLarge := [4]int{1, 2, 3, 4}

	fmt.Println("--- 타입 비교 ---")
	fmt.Printf("arrSmall 타입: %T\n", arrSmall)
	fmt.Printf("arrLarge 타입: %T\n", arrLarge)
	// fmt.Println(arrSmall == arrLarge) // 이 코드는 컴파일 에러가 발생합니다 (타입이 다르기 때문)
}