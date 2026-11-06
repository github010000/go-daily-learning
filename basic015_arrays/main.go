package main

import "fmt"

// 배열의 길이가 타입의 일부인 것을 보여줍니다.
func demoArrayLengthIsType() {
	// 길이가 같은 배열은 타입도 같아 서로 비교/대입 가능합니다.
	var a [3]int = [3]int{1, 2, 3}
	var b [3]int = [3]int{1, 2, 3}
	fmt.Println("길이가 같은 배열 비교:", a == b)

	// 길이 3의 배열을 슬라이스로 변환할 수 있습니다.
	// 배열을 직접 []int 인자로 넘기면 컴파일 오류가 납니다 — 이것이 '길이=타입'의 일상적 비용입니다.
	fmt.Println("배열을 슬라이스로 변환:", []int(a))
}

// 배열은 값 타입이라 함수에 넘기면 원본이 아닌 '복사본'이 들어갑니다.
func demoArrayCopyOnPass() {
	before := [3]int{10, 20, 30}
	after := mutateArray(before)

	// before는 여전히 {10, 20, 30}. mutateArray는 복사본의 인덱스 1을 99로 바꿨을 뿐입니다.
	fmt.Println("배열(복사본 전달): before:", before, "after:", after)
}

// 슬라이스는 레퍼런스 타입이라 원본이 바뀝니다.
func demoSliceReferencePass() {
	before := []int{10, 20, 30}
	after := mutateSlice(before)

	// before의 underlying array가 바뀌었으므로 before도 {10, 99, 30}이 됩니다.
	fmt.Println("슬라이스(원본 공유): before:", before, "after:", after)
}

// 배열을 인자로 받아 원소를 99로 바꿉니다.
// 배열은 값 전달이라 외부 영향 없음.
func mutateArray(a [3]int) [3]int {
	a[1] = 99
	return a
}

// 슬라이스를 인자로 받아 원소를 99로 바꿉니다. 원본 배열이 바뀝니다.
func mutateSlice(s []int) []int {
	s[1] = 99
	return s
}

func main() {
	demoArrayLengthIsType()
	demoArrayCopyOnPass()
	demoSliceReferencePass()
}