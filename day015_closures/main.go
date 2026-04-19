package main

import "fmt"

// newCounter는 정수를 반환하는 함수를 반환합니다.
// 이 함수 내부에서 정의된 변수 i는 클로저에 의해 캡처됩니다.
func newCounter() func() int {
	i := 0 // 이 변수는 클로저가 살아있는 동안 유지됩니다.
	return func() int {
		i++ // 외부 함수의 변수 i에 접근하여 값을 변경합니다.
		return i
	}
}

// fibonacci는 피보나치 수열을 생성하는 함수를 반환합니다.
// 클로저를 사용하여 이전 수열의 상태(a, b)를 유지합니다.
func fibonacci() func() int {
	a, b := 0, 1
	return func() int {
		ret := a
		// 다음 피보나치 수를 계산하기 위해 상태를 업데이트합니다.
		a, b = b, a+b
		return ret
	}
}

// makeAdder는 특정 숫자 x를 더해주는 함수를 생성하는 '함수 생성기'입니다.
// x는 클로저에 의해 캡처되어 함수의 상태로 남게 됩니다.
func makeAdder(x int) func(int) int {
	return func(y int) int {
		return x + y
	}
}

func main() {
	// --- 예제 1: 카운터 (Stateful Counter) ---
	fmt.Println("--- 카운터 예제 ---")
	counter := newCounter()
	fmt.Println(counter()) // 출력: 1
	fmt.Println(counter()) // 출력: 2
	fmt.Println(counter()) // 출력: 3

	// 새로운 카운터를 생성하면 이전 상태와는 독립적인 새로운 상태를 가집니다.
	newCounterInstance := newCounter()
	fmt.Println(newCounterInstance()) // 출력: 1

	// --- 예제 2: 피보나치 수열 생성기 ---
	fmt.Println("\n--- 피보나치 예제 ---")
	f := fibonacci()
	for i := 0; i < 10; i++ {
		fmt.Printf("%d ", f())
	}
			// 출력: 0 1 1 2 3 5 8 13 21 34
	fmt.Println()

	// --- 예제 3: 함수 생성기 (Adder) ---
	fmt.Println("\n--- Adder 예제 ---")
	addTen := makeAdder(10) // 10을 더하는 함수 생성
	addTwenty := makeAdder(20) // 20을 더하는 함수 생성

	fmt.Println(addTen(5))    // 출력: 15
	fmt.Println(addTen(100))  // 출력: 110
	fmt.Println(addTwenty(5)) // 출력: 25
}