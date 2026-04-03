package main

import "fmt"

// add는 두 정수를 더한 값을 반환하는 함수입니다.
func add(a, b int) int {
	return a + b
}

// multiReturn는 두 개의 정수를 받아, 합과 곱을 각각 반환합니다.
// Go는 다중 반환값을 지원합니다.
func multiReturn(a, b int) (int, int) {
	sum := a + b
	product := a * b
	return sum, product
}

// namedReturn는 이름 있는 반환값을 사용하는 예제입니다.
// 반환값에 이름을 붙이면 함수 내에서 선언 없이 직접 사용 가능합니다.
func namedReturn(a, b int) (sum int, product int) {
	sum = a + b      // sum과 product는 자동으로 선언된 지역 변수처럼 동작
	product = a * b
	// return 문에서 이름을 생략해도 됩니다 (implicit return)
	return
}

// higherOrderFunc는 함수를 인자로 받아, 함수를 반환하는 higher-order 함수입니다.
// Go의 함수는 first-class citizen이므로, 변수에 할당, 인자/반환값으로 사용 가능합니다.
func higherOrderFunc(op func(int, int) int) func(int, int) int {
	// op 함수를 감싸는 새로운 함수 반환
	return func(x, y int) int {
		fmt.Println("[ высший порядок ] op 함수를 적용합니다:")
		return op(x, y)
	}
}

func main() {
	// 1. 기본 함수 호출
	result1 := add(10, 20)
	fmt.Println("add(10, 20) =", result1) // 출력: add(10, 20) = 30

	// 2. 다중 반환값 예제
	sum, product := multiReturn(3, 4)
	fmt.Printf("multiReturn(3, 4): sum=%d, product=%d\n", sum, product) // 7, 12

	// _는 무시할 반환값 (여기선 product를 무시)
	_onlySum, _ := multiReturn(5, 6)
	fmt.Println("multiReturn(5, 6)의 합만: