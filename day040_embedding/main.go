package main

import (
	"fmt"
)

// --- 1. 구조체 임베딩 (Struct Embedding) ---

// Base는 기본 구조체입니다.
type Base struct {
	ID int
}

// Base의 메서드 (메서드 프로모션 대상)
func (b Base) Display() {
	fmt.Printf("Base ID: %d\n", b.ID)
}

// Derived는 Base를 임베딩합니다.
// Go에서는 상속이 아니라 컴포지션(Composition)을 사용합니다.
type Derived struct {
	Base // 임베딩: Base의 필드(ID)와 메서드(Display)를 '가집니다'
	Name string
}

// Derived만의 메서드
func (d Derived) ShowName() {
	fmt.Printf("Name: %s\n", d.Name)
}

// --- 2. 인터페이스 임베딩 (Interface Embedding) ---

// Reader 인터페이스
type Reader interface {
	Read() string
}

// Writer 인터페이스
type Writer interface {
	Write() string
}

// ReadWriter는 Reader와 Writer 인터페이스를 임베딩하여 조합합니다.
// Go 1.14부터 인터페이스 임베딩을 지원합니다.
type ReadWriter interface {
	Reader
	Writer
}

// --- 3. 임베딩 활용 예시 ---

// MyReaderWriter는 ReadWriter 인터페이스를 구현합니다.
type MyReaderWriter struct {
	Data string
}

func (m MyReaderWriter) Read() string {
	return fmt.Sprintf("Reading: %s", m.Data)
}

func (m MyReaderWriter) Write() string {
	return fmt.Sprintf("Writing: %s", m.Data)
}

func main() {
	fmt.Println("=== Day 40: 임베딩(Embedding) ===")

	// 1. 구조체 임베딩 테스트
	fmt.Println("\n--- 구조체 임베딩 ---")
	d := Derived{
		Base: Base{ID: 101},
		Name: "GoLang",
	}

	// promoted field 접근
	fmt.Printf("d.ID: %d\n", d.ID) 

	// promoted method 접근
	d.Display() 
	d.ShowName()

	// 2. 인터페이스 임베딩 테스트
	fmt.Println("\n--- 인터페이스 임베딩 ---")
	
	var rw ReadWriter = MyReaderWriter{Data: "Hello Embedding"}
	
	// ReadWriter는 Reader와 Writer를 임베딩했으므로, 해당 메서드를 직접 호출할 수 있습니다.
	fmt.Println(rw.Read())
	fmt.Println(rw.Write())

	// 3. 임베딩 vs 상속
	fmt.Println("\n--- 임베딩 vs 상속 ---")
	fmt.Println("Go는 클래스 기반 상속을 지원하지 않습니다.")
	fmt.Println("대신 임베딩을 통해 'has-a' 관계를 구현합니다.")
	fmt.Println("예: Derived는 Base를 '가지고 있습니다'.")
	fmt.Println("이는 더 유연한 설계와 낮은 결합도를 가능하게 합니다.")
}