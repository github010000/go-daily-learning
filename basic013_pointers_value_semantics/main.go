package main

import "fmt"

// Person은 충분히 커서 값 복사가 부담되는 구조체라고 가정합니다.
type Person struct {
	Name    string
	Age     int
	Email   string
	Address string
	Tags    []string
}

// ---------------------------------------------------------------------------
// 1. 값 복사 vs 포인터 전달: swap
// ---------------------------------------------------------------------------

// swapVal은 두 int를 값으로 받아들이지만, Go는 인자를 값으로 복사하므로
// 호출자의 a, b는 바뀌지 않습니다.
func swapVal(a, b int) {
	a, b = b, a // 복사본끼리 바꾸기만 할 뿐
}

// swapPtr은 두 int의 주소를 받아 역참조(*)로 원본 값을 바꿉니다.
func swapPtr(a, b *int) {
	*a, *b = *b, *a
}

// ---------------------------------------------------------------------------
// 2. nil 포인터 역참조 panic
// ---------------------------------------------------------------------------

// nilDeref는 nil 포인터를 역참조하면 panic이 발생하는 것을 보여줍니다.
func nilDeref() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("nilDeref panicked: %v\n", r)
		}
	}()

	var p *int // nil 포인터
	fmt.Println(*p) // panic!
}

// ---------------------------------------------------------------------------
// 3. 포인터를 써야 할 때: 슬라이스 헤더(head) 자체를 바꾸고 싶을 때
// ---------------------------------------------------------------------------

// badAppend는 슬라이스를 값으로 받습니다. append는 슬라이스 헤더(포인터, 길이, 용량)를
// 반환하므로, 반환값을 무시하면 원본 슬라이스는 아무런 변화가 없습니다.
func badAppend(src []int) {
	src = append(src, 99) // src는 복사본, 이 변경은 main으로 돌아오지 않음
}

// goodAppend는 append 결과를 반환합니다. 호출자가 반환값을 받아야 합니다.
func goodAppend(src []int) []int {
	return append(src, 99)
}

// ---------------------------------------------------------------------------
// 4. 포인터를 쓰지 않아도 되는 경우: 슬라이스 요소 자체는 값으로 넘겨도 됨
// ---------------------------------------------------------------------------

// setFirst는 슬라이스 헤더를 바꾸지 않고 요소 값만 수정합니다.
// 슬라이스 자체는 "값 타입이지만 내부에 포인터를 담은 구조체"이므로,
// 헤더(포인터/len/cap)는 복사되어 전달되지만, 그 포인터가 가리키는 underlying 배열은 공유됩니다.
func setFirst(src []int, v int) {
	src[0] = v // underlying 배열을 직접 수정 — 원본에 반영됨
}

// ---------------------------------------------------------------------------
// 5. 구조체 포인터: 큰 구조체를 복사하지 않고 포인터로 넘길 때
// ---------------------------------------------------------------------------

// copyPerson은 Person을 값으로 복사합니다.
func copyPerson(p Person) Person {
	p.Age = 99
	return p
}

// ptrPerson은 Person 포인터를 받아 원본을 수정합니다.
func ptrPerson(p *Person) {
	p.Age = 99
}

func main() {
	fmt.Println("=== 1. 값 복사 vs 포인터 전달 (swap) ===")

	a, b := 10, 20
	fmt.Printf("swapVal 전: a=%d, b=%d\n", a, b)
	swapVal(a, b)
	fmt.Printf("swapVal 후: a=%d, b=%d (바뀌지 않음)\n", a, b)

	c, d := 10, 20
	fmt.Printf("swapPtr 전: c=%d, d=%d\n", c, d)
	swapPtr(&c, &d)
	fmt.Printf("swapPtr 후: c=%d, d=%d (바뀜!)\n", c, d)

	fmt.Println()
	fmt.Println("=== 2. nil 포인터 역참조 panic ===")
	nilDeref()

	fmt.Println()
	fmt.Println("=== 3. 슬라이스: append 시 헤더 교체 vs 요소 수정 ===")

	// badAppend: 헤더를 바꾸므로 원본에 영향 없음
	s1 := []int{1, 2, 3}
	fmt.Printf("badAppend 전: s1=%v (len=%d, cap=%d)\n", s1, len(s1), cap(s1))
	badAppend(s1)
	fmt.Printf("badAppend 후: s1=%v (len=%d, cap=%d) — unchanged!\n", s1, len(s1), cap(s1))

	// goodAppend: 반환값을 받아야 함
	s2 := []int{1, 2, 3}
	fmt.Printf("goodAppend 전: s2=%v (len=%d, cap=%d)\n", s2, len(s2), cap(s2))
	s2 = goodAppend(s2)
	fmt.Printf("goodAppend 후: s2=%v (len=%d, cap=%d) — appended!\n", s2, len(s2), cap(s2))

	// setFirst: 헤더는 안 바뀌지만 underlying 배열은 공유
	s3 := []int{1, 2, 3}
	fmt.Printf("setFirst 전: s3=%v\n", s3)
	setFirst(s3, 99)
	fmt.Printf("setFirst 후: s3=%v (underlying 배열 공유로 반영됨)\n", s3)

	fmt.Println()
	fmt.Println("=== 4. 구조체: 값 복사 vs 포인터 전달 ===")

	p1 := Person{Name: "Alice", Age: 30}
	fmt.Printf("copyPerson 전: p1.Age=%d\n", p1.Age)
	p1 = copyPerson(p1)
	fmt.Printf("copyPerson 후: p1.Age=%d (복사본만 바뀜)\n", p1.Age)

	p2 := Person{Name: "Bob", Age: 25}
	fmt.Printf("ptrPerson 전: p2.Age=%d\n", p2.Age)
	ptrPerson(&p2)
	fmt.Printf("ptrPerson 후: p2.Age=%d (원본이 바뀜!)\n", p2.Age)
}