package main

import (
	"fmt"
)

// Rectangle 구조체를 정의합니다.
type Rectangle struct {
	width, height float64
}

// [값 수신자 (Value Receiver)]
// Area 메서드는 Rectangle의 복사본을 받습니다.
// 구조체의 내부 상태를 변경하지 않고 값을 읽기만 할 때 사용합니다.
func (r Rectangle) Area() float64 {
	return r.width * r.height
}

// [포인터 수신자 (Pointer Receiver)]
// Scale 메서드는 Rectangle의 포인터를 받습니다.
// 구조체의 내부 필드 값을 직접 수정해야 할 때 사용합니다.
func (r *Rectangle) Scale(factor float64) {
	r.width = r.width * factor
	r.height = r.height * factor
}

// [비구조체 타입에 메서드 추가]
// 기본 타입인 int를 기반으로 새로운 타입을 정의합니다.
type MyInt int

// MyInt 타입에 메서드를 추가할 수 있습니다.
// 구조체가 아니더라도 사용자 정의 타입(Defined Type)이라면 메서드 정의가 가능합니다.
func (m My            Int) IsEven() bool {
	return m%2 == 0
}

func main() {
	// 1. 구조체와 값 수신자 메서드 테스트
	rect := Rectangle{width: 10, height: 5}
	fmt.Printf("초기 사각형 면적: %.2f\n", rect.Area())

	// 2. 포인터 수신자 메서드 테스트 (원본 데이터 변경)
	fmt.Println("사각형 크기를 2배로 확대합니다...")
	rect.Scale(2) // rect.Scale(2)는 내부적으로 (&rect).Scale(2)로 동작합니다.
	fmt.Printf("확대된 사각형 면적: %.2f (가로: %.2f, 세로: %.2f)\n", rect.Area(), rect.width, rect.height)

	// 3. 비구조체 타입(MyInt) 메서드 테스트
	var num MyInt = 10
	var oddNum MyInt = 7

	fmt.Printf("숫자 %d는 짝수인가요? %t\n", num, num.IsEven())
	fmt.Printf("숫자 %d는 짝수인가요? %t\n", oddNum, oddNum.IsEven())
}