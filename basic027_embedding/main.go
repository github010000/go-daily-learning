package main

import (
	"fmt"
)

// ---------- 기본 타입 정의 ----------

// Shape는 평면 도형의 공통 기반이 되는 구조체입니다.
// 이름, 좌표, 기본 정보 문자열 생성, 기본 그리기 동작을 담고 있습니다.
type Shape struct {
	Name string
	X, Y int
}

// Info는 도형의 위치와 이름을 포맷한 문자열을 반환합니다.
func (s Shape) Info() string {
	return fmt.Sprintf("Shape[%s] at (%d,%d)", s.Name, s.X, s.Y)
}

// Draw는 도형을 화면에 그리는 동작을 문자열로 반환합니다.
// Shape의 기본 구현은 "기본 그리기" 메시지만 출력합니다.
func (s Shape) Draw() string {
	return fmt.Sprintf("그림: %s (%d,%d)", s.Name, s.X, s.Y)
}

// Area는 도형의 면적을 반환합니다. Shape 자체는 면적 정의가 없으므로
// 임베딩한 구체적인 타입이 이 메서드를 제공해야 합니다.

// ---------- 승격(Promotion) 시연 타입 ----------

// Circle은 Shape를 임베딩한 원입니다.
// Shape의 Name, X, Y 필드와 Info(), Draw() 메서드가 Circle에 '승격'됩니다.
// 즉, Circle 인스턴스에서 c.Name, c.Info()처럼 Shape의 멤버를 직접 쓸 수 있습니다.
type Circle struct {
	Shape
	Radius int
}

// Area는 원의 면적을 계산합니다.
// Circle에는 Shape의 Area가 없으므로, Circle에 새로 정의한 이 메서드가 사용됩니다.
func (c Circle) Area() float64 {
	return 3.14 * float64(c.Radius) * float64(c.Radius)
}

// Rect는 Shape를 임베딩한 사각형입니다.
type Rect struct {
	Shape
	Width, Height int
}

// Area는 사각형의 면적을 계산합니다.
func (r Rect) Area() float64 {
	return float64(r.Width) * float64(r.Height)
}

// ---------- 인터페이스 임베딩 ----------

// Describable은 설명 가능한 타입에 요구되는 인터페이스입니다.
// Description() 메서드를 가집니다.
type Describable interface {
	Description() string
}

// PrintableCircle은 Shape를 임베딩하고 Description() 메서드를 추가한 구조체입니다.
// Shape의 Area()는 없지만, PrintableCircle 자체에 Area()를 추가함으로써
// Describable 인터페이스를 충족시킬 수 있습니다.
type PrintableCircle struct {
	Shape
}

// Description은 PrintableCircle의 설명을 반환합니다.
// 내부의 Shape를 통해 Name, X, Y에 접근하고, 자체 Area()를 사용합니다.
func (pc PrintableCircle) Description() string {
	return fmt.Sprintf("%s의 면적은 %.2f", pc.Name, pc.Area())
}

// Area는 PrintableCircle의 면적을 반환합니다.
// Shape에는 이 메서드가 없으므로, PrintableCircle에 별도로 정의해야 합니다.
func (pc PrintableCircle) Area() float64 {
	return 3.14 * float64(1) * float64(1) // 단위원 가정
}

// ---------- 이름 충돌 시연 타입 ----------

// NamedCircle은 Shape를 임베딩하면서 Name 필드를 자체적으로 가집니다.
// 'Name' 필드가 충돌하는 대표적인 케이스입니다.
// 이 타입은 실수 시연용으로만 사용되며, 테스트에서는 사용되지 않습니다.
type NamedCircle struct {
	Name string // Circle의 Name(Shape.Name)과 충돌
	Circle
}

// ---------- 메서드 재정의 시연 타입 ----------

// DrawCircle은 Circle을 임베딩하고 Draw() 메서드를 재정의합니다.
type DrawCircle struct {
	Circle
}

// Draw는 Circle의 Draw를 재정의합니다.
func (dc DrawCircle) Draw() string {
	return fmt.Sprintf("원 그리기: 반지름 %d", dc.Radius)
}

// ---------- 함수들 ----------

func main() {
	fmt.Println("=== 1. 필드와 메서드 승격 (Promotion) ===")
	showPromotion()

	fmt.Println("\n=== 2. 이름 충돌 (Name Collision) ===")
	showNameCollision()

	fmt.Println("\n=== 3. 메서드 재정의 (Override)와 인터페이스 ===")
	showInterfaceOverride()

	fmt.Println("\n=== 4. 위임(Delegation)을 통한 인터페이스 충족 ===")
	showDelegation()

	fmt.Println("\n=== 5. 흔한 실수: 상속이 아닌 위임 ===")
	showCommonMistake()
}

// showPromotion: 필드와 메서드가 '승격'되어 직접 접근 가능한 모습 보여줌
func showPromotion() {
	c := Circle{Shape: Shape{Name: "원", X: 10, Y: 20}, Radius: 5}

	// Shape의 필드와 메서드가 Circle의 필드/메서드처럼 접근됨 (승격)
	fmt.Printf("c.Name: %s, c.X: %d\n", c.Name, c.X)
	fmt.Printf("c.Info(): %s\n", c.Info())

	// 하지만 원래 타입의 메서드도 여전히 호출 가능 (명시적)
	fmt.Printf("c.Shape.Info(): %s\n", c.Shape.Info())

	// Circle이 Shape의 Area를 재정의했으므로 c.Area()는 원의 면적을 반환
	fmt.Printf("c.Area(): %.2f\n", c.Area())
}

// showNameCollision: 이름이 충돌할 때 어떻게 동작하는지 보여줌
// (테스트에서는 사용하지 않는 실수 버전 시연입니다)
func showNameCollision() {
	nc := NamedCircle{
		Name: "특별한 원", // NamedCircle의 Name 필드 (우선)
		Circle: Circle{
			Shape:  Shape{Name: "기본 원", X: 0, Y: 0},
			Radius: 3,
		},
	}

	// 이름이 충돌: 최상위 (NamedCircle.Name)가 우선
	fmt.Printf("nc.Name: %s (NamedCircle의 Name)\n", nc.Name)

	// 명시적으로 내부 구조체의 필드/메서드 접근
	fmt.Printf("nc.Shape.Name: %s (Shape의 Name)\n", nc.Shape.Name)
	fmt.Printf("nc.Shape.Info(): %s (Shape의 Info 메서드)\n", nc.Shape.Info())

	// Circle의 메서드 (Info는 Shape에서 승격, Name은 충돌)
	// nc.Info() 는 Shape.Info() 를 호출 (Circle에 Info 없으므로)
	fmt.Printf("nc.Info(): %s (Shape.Info 승격)\n", nc.Info())
}

// showInterfaceOverride: 메서드를 재정의하여 인터페이스를 충족하는 모습 보여줌
func showInterfaceOverride() {
	dc := DrawCircle{Circle{Shape: Shape{Name: "원", X: 0, Y: 0}, Radius: 5}}

	// DrawCircle은 Draw()를 재정의했으므로 DrawCircle 전용 동작을 가짐
	fmt.Printf("dc.Draw(): %s\n", dc.Draw())

	// 하지만 Circle의 다른 메서드들은 그대로 사용 가능
	fmt.Printf("dc.Info(): %s\n", dc.Info())
}

// showDelegation: 임베딩을 통한 인터페이스 충족 (위임)
func showDelegation() {
	pc := PrintableCircle{Shape: Shape{Name: "반지름 5의 원", X: 10, Y: 10}}

	// PrintableCircle은 Shape를 임베딩했으므로 Area()를 쓸 수 있고,
	// Description()을 가지므로 Describable 인터페이스를 충족합니다.
	var d Describable = pc
	fmt.Printf("d.Description(): %s\n", d.Description())

	// Shape 자체는 Area()가 없으므로 Describable을 충족하지 못합니다.
	// pc.Shape.Description()은 컴파일 오류입니다.
}

// showCommonMistake: "상속" vs "위임" 오해 시연
func showCommonMistake() {
	// 초보자가 기대하는 것: 부모 클래스의 필드를 수정하면 자식도 바뀜
	// Go에서는 임베딩이 참조가 아닌 값 복사이므로 다름

	base := Shape{Name: "부모", X: 1}
	child := Circle{Shape: base, Radius: 2}

	fmt.Printf("초기: child.Name = %s, child.X = %d\n", child.Name, child.X)

	// base를 수정해도 child는 변하지 않음 (값 타입 복제)
	base.Name = "수정된 부모"
	base.X = 99
	fmt.Printf("base 수정 후: child.Name = %s, child.X = %d (변동 없음)\n", child.Name, child.X)

	// 반면, 포인터 임베딩을 사용하면 base를 수정하면 child도 변합니다.
	fmt.Println("\n=== 포인터 임베딩 (참조 동작) ===")
	basePtr := &Shape{Name: "부모", X: 1}
	childPtr := Circle{Shape: *basePtr, Radius: 2} // 값 복사

	basePtr.Name = "수정된 부모"
	basePtr.X = 99

	fmt.Printf("basePtr 수정 후: childPtr.Name = %s, childPtr.X = %d\n", childPtr.Name, childPtr.X)
}