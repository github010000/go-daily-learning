package main

import (
	"fmt"
)

// --- 도형 구조체 정의 ---

type Rectangle struct {
	Width, Height float64
}

// Circle는 반지름을 저장하지만, 원주/넓이 계산에는 사용하지 않는 필드가 있을 수 있다.
type Circle struct {
	Radius       float64
	LastUpdated  string // 상태 변경 시 기록하는 필드
}

// --- 메서드 정의: 리시버 유형별 분류 ---

// Area는 면적을 반환한다. 내부 상태를 바꾸지 않으므로 값 리시버를 사용한다.
// 왜 값 리시버? 함수형이므로 안전하고, 복사 비용이 작기 때문이다.
func (r Rectangle) Area() float64 {
	return r.Width * r.Height
}

// Area는 Circle의 면적을 계산한다.
func (c Circle) Area() float64 {
	return 3.14159 * c.Radius * c.Radius
}

// Grow는 도형의 크기를 키운다. 호출 후 값이 바뀌어야 하므로 포인터 리시버다.
// 왜 포인터 리시버? 리시버 자체를 수정해야 하기 때문이다.
func (r *Rectangle) Grow(factor float64) {
	r.Width *= factor
	r.Height *= factor
}

// Grow는 원의 반지름을 키운다.
func (c *Circle) Grow(factor float64) {
	c.Radius *= factor
	// 상태도 갱신
	c.LastUpdated = "grown"
}

// --- 실전 시연: 함수 파라미터 타입과 메서드 호출 관계 ---

// processRectangleByValue는 값으로 도형을 받는다. 내부에서 Grow()를 호출할 수 있을까?
// 정답: 할 수 없다. Rectangle (값)은 포인터 리시버 메서드인 Grow()를 호출할 수 없다.
// (컴파일 에러 발생. 이 시연은 컴파일되지 않으므로 README에만 표시)
func processRectangleByValue(r Rectangle) {
	// r.Grow(2.0) // <-- 에러: cannot use r (variable of type Rectangle) as type *Rectangle in argument to Grow
	_ = r.Area() // 값 리시버 메서드는 호출 가능
}

// processRectangleByPointer는 포인터로 도형을 받는다. 모든 메서드 호출이 가능하다.
func processRectangleByPointer(r *Rectangle) {
	r.Grow(2.0) // 포인터 리시버 메서드 호출 가능
	fmt.Printf("포인터 파라미터로 처리 후 너비: %.2f, 높이: %.2f\n", r.Width, r.Height)
}

// --- 실수 시연: 값 리시버로Grow() 메서드를 정의하는 경우 ---
// 이 함수는 "실수한 버전"을 보여준다.
// 리시버를 값으로 받으면, 원본이 바뀌지 않고 복사본만 커진다.
func (r Rectangle) GrowByValue(factor float64) {
	// r은 복사본이다. 이 값을 수정해도 원본 r은 변하지 않는다.
	r.Width *= factor
	r.Height *= factor
	// return 없음. 호출한 쪽에는 영향이 없다.
}

// --- 실수 시연: interface 메서드 집합 혼동 ---
// Shape는 Area() 메서드를 요구한다.
// Rectangle는 값 리시버로 Area()를 구현했으므로, Rectangle와 *Rectangle 모두 interface를 만족한다.
// Circle도 마찬가지.

// printShapeInfo는 interface를 받는다.
func printShapeInfo(s interface {
	Area() float64
	Name() string
}) {
	fmt.Printf("이름: %s, 넓이: %.2f\n", s.Name(), s.Area())
}

// CircleName은 Circle의 이름 반환 메서드 (값 리시버)
func (c Circle) Name() string { return "Circle" }

// RectangleName은 Rectangle의 이름 반환 메서드 (값 리시버)
func (r Rectangle) Name() string { return "Rectangle" }

// --- panic 시연: nil 포인터 리시버 ---
// nil 포인터로 메서드를 호출하면 panic이 발생한다.
// 포인터 리시버를 쓸 때는 nil 체크가 필수다.
func (c *Circle) SafeArea() float64 {
	if c == nil {
		// nil일 때 예외 처리 로직
		return 0
	}
	return c.Area()
}

func demonstrateNilPanic() {
	var c *Circle // nil Circle 포인터
	fmt.Println("nil 포인터로 SafeArea() 호출:", c.SafeArea()) // 안전하게 0 반환
	// 만약 c.Area()를 직접 호출했다면 panic 발생:
	// panic: runtime error: invalid memory address or nil pointer dereference
}

func main() {
	fmt.Println("=== 1. 값 리시버 vs 포인터 리시버 기본 동작 ===")

	// 값 리시버 메서드 (Area): 복사되어도 문제 없음
	rect := Rectangle{Width: 10, Height: 5}
	fmt.Printf("사각형 넓이: %.2f\n", rect.Area())

	circle := Circle{Radius: 7}
	fmt.Printf("원 넓이: %.2f\n", circle.Area())

	// 포인터 리시버 메서드 (Grow): 원본이 수정됨
	fmt.Println("\n--- 포인터 리시버 Grow() 실행 ---")
	fmt.Printf("Grow 전 사각형: %.2f x %.2f\n", rect.Width, rect.Height)
	rect.Grow(2.0)
	fmt.Printf("Grow 후 사각형: %.2f x %.2f\n", rect.Width, rect.Height)

	// --- 실수 버전: 값 리시버로 Grow()를 정의한 경우 ---
	fmt.Println("\n--- 실수 버전: 값 리시버 GrowByValue() ---")
	rect2 := Rectangle{Width: 10, Height: 5}
	fmt.Printf("GrowByValue 전 사각형: %.2f x %.2f\n", rect2.Width, rect2.Height)
	rect2.GrowByValue(2.0)
	fmt.Printf("GrowByValue 후 사각형: %.2f x %.2f (원본은 변하지 않음!)\n", rect2.Width, rect2.Height)

	// --- 함수 파라미터와 메서드 호환성 ---
	fmt.Println("\n=== 2. 함수 파라미터와 메서드 호환성 ===")
	rect3 := Rectangle{Width: 10, Height: 5}
	processRectangleByPointer(&rect3) // 포인터 전달: Grow 가능
	// processRectangleByValue(rect3) // 값 전달: Grow 불가 (컴파일 에러)

	// --- interface 메서드 집합 ---
	fmt.Println("\n=== 3. interface 메서드 집합 ===")
	var r Rectangle = Rectangle{Width: 10, Height: 5}
	var c Circle = Circle{Radius: 7}
	printShapeInfo(r)
	printShapeInfo(c)
	printShapeInfo(&r) // *Rectangle도 Area()를 가짐 (값 리시버 메서드는 포인터에서도 호출 가능)
	printShapeInfo(&c)

	// --- nil 포인터 리시버 안전 처리 ---
	fmt.Println("\n=== 4. nil 포인터 리시버 안전 처리 ===")
	demonstrateNilPanic()

	fmt.Println("\n모든 시연 완료.")
}