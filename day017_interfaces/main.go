package main

import (
	"fmt"
)

// Shape 인터페이스 정의: Area() 메서드를 가진 모든 타입은 Shape가 될 수 있습니다.
type Shape interface {
	Area() float64
}

// Rectangle 구조체 정의
type Rectangle struct {
	width, height float64
}

// Rectangle의 Area 메서드 구현 (Rectangle은 이제 Shape를 암묵적으로 구현합니다)
func (r Rectangle) Area() float64 {
	return r.width * r.height
}

// Circle 구조체 정의
type Circle struct {
	radius float64
}

// Circle의 Area 메서드 구현 (Circle은 이제 Shape를 암묵적으로 구현합니다)
func (c Circle) Area() float64 {
	return 3.14 * c.radius * c.radius
}

// printArea 함수는 Shape 인터페이스를 매개변수로 받습니다.
// 구체적인 타입(Rectangle, Circle)이 무엇인지 몰라도 Area() 메서드만 있으면 동작합니다.
func printArea(s Shape) {
	if s == nil {
		fmt.Println("도형 정보가 없습니다 (nil interface).")
		return
	}
	fmt.Printf("도형의 넓이: %.2f\n", s.Area())
}

func main() {
	// 1. 인터페이스를 통한 다형성 활용
	rect := Rectangle{width: 10, height: 5}
	circ := Circle{radius: 5}

	fmt.Println("--- 인터페이스 다형성 테스트 ---")
	printArea(rect) // Rectangle 전달
	printArea(circ) // Circle 전달

	// 2. 인터페이스 값과 nil 인터페이스
	var s Shape // 인터페이스 변수 선언 (초기값은 nil)
	fmt.Println("\n--- nil 인터페이스 테스트 ---")
	printArea(s) // s는 타입도 값도 nil인 상태

	// 3. 주의할 점: 인터페이스가 nil 포인터를 담고 있는 경우
	// 인터페이스는 (Type, Value) 쌍으로 구성됩니다.
	// 타입이 지정되면 값(Value)이 nil이라도 인터페이스 자체는 nil이 아닙니다.
	var rectPtr *Rectangle = nil
	s = rectPtr // s의 타입은 *Rectangle, 값은 nil

	fmt.compt
	fmt.Printf("s의 타입: %T, s의 값: %v\n", s, s)

	if s != nil {
		fmt.Println("s는 nil이 아닙니다! (타입 정보가 존재하기 때문)")
	} else {
		fmt.Println("s는 nil입니다.")
	}

	// 하지만 s.Area()를 호출하려고 하면 런타임 에러(panic)가 발생할 수 있습니다.
	// 위 예제에서는 Rectangle의 메서드가 value receiver(r Rectangle)를 사용하므로
	// nil 포인터에서도 안전하게 동작하지만, pointer receiver라면 주의가 필요합니다.
}