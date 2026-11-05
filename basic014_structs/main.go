package main

import (
	"fmt"
)

// --- 1. 기본 타입 정의 ---

// Point는 2차원 좌표를 표현하는 값 타입입니다.
type Point struct {
	X, Y int
}

// Color는 RGB 색상 값을 담습니다.
type Color struct {
	R, G, B int
}

// Person은 이름과 나이를 가진 구조체입니다.
type Person struct {
	Name string
	Age  int
}

// --- 2. 실수 버전 함수들 ---

// 실수1: 값을 전달했는데 반환값을 무시합니다.
// Go는 모든 인자가 '값 전달'이므로, p.X = 100은 복사본만 수정하고 원본은 변하지 않습니다.
// 이 함수를 호출해도 p의 실제 값은 바뀌지 않았음을 보여주는 예시입니다.
func MutatePointByValue(p Point) {
	p.X = 100
	p.Y = 200
}

// 실수2: 이름만 바꾸는 함수인데 반환값이 없습니다.
// Person도 값 타입이므로, 반환값 없이 호출하면 변경 사항이 유실됩니다.
func MutatePersonName(p Person, name string) {
	p.Name = name
}

// 실수3: 슬라이스가 포함된 구조체는 '얕은 복사(Shallow Copy)'가 됩니다.
// 구조체 자체를 복사해도, 슬라이스 헤더만 복사될 뿐 실제 데이터는 공유됩니다.
func MutateSliceInStruct(ps Person) {
	// 슬라이스 요소 변경 -> 원본의 슬라이스에도 영향
	ps.Tags[0] = "mutated"
}

// --- 3. 고친 버전 함수들 ---

// 고친1: 변경 사항을 반환값으로 반환합니다.
// struct는 값 타입이므로, 수정하려면 반드시 새 값을 받아야 합니다.
func FixPoint(p Point) Point {
	p.X = 100
	p.Y = 200
	return p
}

// 고친2: Person 구조체 전체를 반환합니다.
func FixPersonName(p Person, name string) Person {
	p.Name = name
	return p
}

// 고친3: 슬라이스 내용을 깊은 복사(Deep Copy)로 처리합니다.
func FixSliceInStruct(ps Person) Person {
	// 새로운 슬라이스를 만들어 할당
	copyTags := make([]string, len(ps.Tags))
	copy(copyTags, ps.Tags)
	copyTags[0] = "mutated"
	ps.Tags = copyTags
	return ps
}

// --- 4. 주요 기능 시연 함수들 ---

// 구조체 리터럴과 익명 구조체 사용법을 보여줍니다.
func ShowLiteralsAndAnonymous() {
	// 필드 순서만 맞추면 이름을 생략할 수 있습니다.
	p := Point{3, 4}
	fmt.Printf("Point 리터럴(순서만): %#+v\n", p)

	// 필드 이름을 쓰면 순서가 바뀌어도 됩니다.
	color := Color{R: 255, G: 0, B: 0}
	fmt.Printf("Color 리터럴(이름 포함): %#+v\n", color)

	// 부분 초기화: 지정하지 않은 필드는 0이나 null로 채워집니다.
	empty := Person{Name: "Unknown"}
	fmt.Printf("Person 부분 초기화: %+v\n", empty)

	// 익명 구조체: 일회성 설정 값 등을 만들 때 유용합니다.
	// type 선언 없이 바로 리터럴로 만듭니다.
	config := struct {
		Debug   bool
		Version string
	}{
		Debug:   true,
		Version: "1.0",
	}
	fmt.Printf("익명 구조체: %+v\n", config)
}

// 구조체 비교(==)가 되는 조건과 안 되는 조건을 보여줍니다.
func ShowStructComparison() {
	p1 := Point{X: 1, Y: 2}
	p2 := Point{X: 1, Y: 2}
	p3 := Point{X: 3, Y: 4}

	fmt.Printf("p1 == p2: %v (같은 좌표)\n", p1 == p2)
	fmt.Printf("p1 == p3: %v (다른 좌표)\n", p1 == p3)

	// Person도 string과 int만 있어서 비교 가능합니다.
	per1 := Person{Name: "Alice", Age: 30}
	per2 := Person{Name: "Alice", Age: 30}
	fmt.Printf("Person 동일 비교: %v\n", per1 == per2)
}

// 공개(Exported)와 비공개(Unexported) 필드 규칙을 보여줍니다.
// 대문자로 시작하는 필드만 외부 패키지에서 접근 가능합니다.
func ShowVisibility() {
	alice := Person{Name: "Alice", Age: 25}

	// 대문자 필드 접근 성공
	fmt.Printf("공개 필드 접근: 이름=%s, 나이=%d\n", alice.Name, alice.Age)

	// 만약 여기에 alice.name = "Bob" 코드가 있다면 컴파일 오류가 납니다:
	// invalid method name name (cannot refer to unexported field or method name)
	fmt.Println("소문자 필드 접근 시도 -> 컴파일 오류 발생 (코드 주석 처리됨)")
}

// 슬라이스가 포함된 구조체가 == 비교에서 왜 실패하는지 보여줍니다.
func ShowUncomparableStruct() {
	// 슬라이스 필드가 있는 구조체는 비교할 수 없습니다.
	type WithSlice struct {
		Tag  string
		Tags []string
	}

	ws := WithSlice{Tag: "a", Tags: []string{"x", "y"}}
	_ = ws // 변수 사용(할당)은 되지만, == 연산은 컴파일 오류가 납니다.
	fmt.Println("슬라이스가 포함된 구조체는 == 비교 불가 (컴파일 오류)")
}

func main() {
	fmt.Println("=== 1. 구조체 리터럴 및 익명 구조체 ===")
	ShowLiteralsAndAnonymous()

	fmt.Println("\n=== 2. 구조체 비교 (==) ===")
	ShowStructComparison()

	fmt.Println("\n=== 3. 실수: 값 전달과 반환값 무시 ===")
	p := Point{X: 1, Y: 2}
	fmt.Printf("수정 전 Point: %#+v\n", p)

	// 실수: 반환값을 무시함 -> p는 그대로
	MutatePointByValue(p)
	fmt.Printf("MutatePointByValue 호출 후 (실수): %#+v\n", p)

	// 고친: 반환값을 받아야 함
	pFixed := FixPoint(p)
	fmt.Printf("FixPoint 호출 후 (고친): %#+v\n", pFixed)

	fmt.Println("\n=== 4. 실수: 슬라이스 포함 구조체의 얕은 복사 ===")
	ps := Person{
		Name: "Original",
		Tags: []string{"a", "b"},
	}
	fmt.Printf("수정 전 Tags: %v\n", ps.Tags)

	// 실수: 슬라이스를 공유하므로 원본 Tags도 변합니다.
	MutateSliceInStruct(ps)
	fmt.Printf("MutateSliceInStruct 후 Tags: %v (원본이 변함!)\n", ps.Tags)

	// 고친: 깊은 복사를 하면 원본이 변하지 않습니다.
	psFixed := FixSliceInStruct(ps)
	fmt.Printf("FixSliceInStruct 후 Tags: %v (원본 유지)\n", psFixed.Tags)

	fmt.Println("\n=== 5. 공개/비공개 필드 ===")
	ShowVisibility()

	fmt.Println("\n=== 6. 구조체 비교의 한계 ===")
	ShowUncomparableStruct()
}