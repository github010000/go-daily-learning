package main

import (
	"fmt"
)

// --- 타입 정의 ---

// Person은 Stringer 인터페이스를 구현합니다.
type Person struct {
	Name string
	Age  int
}

// String은 Person을 "이름 (나이세)" 형태로 출력합니다.
func (p Person) String() string {
	return fmt.Sprintf("%s (%d세)", p.Name, p.Age)
}

// Point는 좌표 정보를 담습니다.
type Point struct {
	X, Y int
}

// String은 Point를 "(x,y)" 형태로 출력합니다.
func (p Point) String() string {
	return fmt.Sprintf("(%d,%d)", p.X, p.Y)
}

// BadStringer는 String() 구현의 함정을 보여주기 위한 예시입니다.
// String() 내부에서 fmt.Sprintf("값: %v", b.Value) 처럼 %v를 쓰면
// fmt가 %v 포맷터가 필요할 때 다시 String()을 호출하여 무한 재귀에 빠집니다.
// 이 타입은 실수 버전을 시연하기 위한 용도로, 테스트에서는 사용하지 않습니다.
type BadStringer struct {
	Value int
}

// String은 BadStringer의 잘못된 구현 예시입니다.
// 실제로는 %v 대신 %d를 써서 재귀를 피해야 합니다.
func (b BadStringer) String() string {
	return fmt.Sprintf("값: %v", b.Value)
}

// Plain은 Stringer를 구현하지 않은 구조체입니다.
// %v와 %+v의 차이를 비교하기 위해 사용합니다.
type Plain struct {
	Name string
}

// --- 시연 함수들 ---

// demonstrateVerbDiff는 %v·%+v·%#v·%T·%q의 차이를 보여줍니다.
// Stringer가 있으면 %v·%+v는 String()을, 없으면 구조체 값을 출력합니다.
func demonstrateVerbDiff() {
	p := Person{Name: "보아", Age: 25}
	pt := Point{X: 10, Y: 20}
	plain := Plain{Name: "기본"}

	fmt.Println("=== Stringer가 있는 타입 ===")
	fmt.Printf("Person %%v : %v\n", p)
	fmt.Printf("Person %%+v: %+v\n", p)
	fmt.Printf("Person %#v: %#v\n", p)
	fmt.Printf("Person %%T : %T\n", p)

	fmt.Printf("Point %%v : %v\n", pt)
	fmt.Printf("Point %%+v: %+v\n", pt)
	fmt.Printf("Point %#v: %#v\n", pt)

	fmt.Println("\n=== Stringer가 없는 타입 ===")
	fmt.Printf("Plain %%v : %v\n", plain)
	fmt.Printf("Plain %%+v: %+v\n", plain)
	fmt.Printf("Plain %#v: %#v\n", plain)

	fmt.Println("\n=== %q 동작 확인 ===")
	fmt.Printf("문자열 %%q: %q\n", "안녕")
	fmt.Printf("숫자 %%q : %q\n", 42)
	fmt.Printf("rune %%q : %q\n", '가')
}

// demonstrateRecursionTrap은 String() 내부에서 %v를 쓰는 것이
// 왜 위험한지 panic을 통해 보여줍니다.
func demonstrateRecursionTrap() {
	fmt.Println("\n=== String() 내부 %v 재귀 트랩 ===")

	// BadStringer는 현재 %v를 사용해 재귀가 발생합니다.
	b := BadStringer{Value: 42}

	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("panic 발생 (재귀): %v\n", r)
		}
	}()

	fmt.Printf("BadStringer: %v\n", b)
}

// demonstrateSafeStringer는 재귀 없이 안전하게 작동하는 Stringer를 보여줍니다.
func demonstrateSafeStringer() {
	fmt.Println("\n=== 안전한 Stringer 구현 ===")

	// %v 대신 %d를 사용해 재귀를 피합니다.
	safe := &SafeStringer{Value: 42}
	fmt.Printf("SafeStringer: %v\n", safe)
}

// SafeStringer는 String() 내부에서 %d를 쓰는 안전 버전입니다.
type SafeStringer struct {
	Value int
}

func (s *SafeStringer) String() string {
	return fmt.Sprintf("값: %d", s.Value)
}

// demonstrateInterface는 Stringer를 인터페이스로 사용하는 방법을 보여줍니다.
func demonstrateInterface() {
	fmt.Println("\n=== Stringer 인터페이스 사용 ===")

	var items []fmt.Stringer
	items = append(items, Person{Name: "철수", Age: 20})
	items = append(items, Point{X: 5, Y: 15})
	items = append(items, SafeStringer{Value: 100})

	for i, item := range items {
		fmt.Printf("항목 %d: %s\n", i, item)
	}
}

// demonstrateLogFormat은 로깅 상황에서 %v와 %+v가 어떻게 다른지 보여줍니다.
func demonstrateLogFormat() {
	fmt.Println("\n=== 로깅에서의 활용 ===")

	p := Person{Name: "영희", Age: 35}
	plain := Plain{Name: "김영희"}

	fmt.Printf("Person 로그: %v 처리 완료\n", p)
	fmt.Printf("Plain 로그: %v 처리 완료\n", plain)
	fmt.Printf("Plain 로그: %+v 상태 확인\n", plain)
}

func main() {
	demonstrateVerbDiff()
	demonstrateRecursionTrap()
	demonstrateSafeStringer()
	demonstrateInterface()
	demonstrateLogFormat()
}