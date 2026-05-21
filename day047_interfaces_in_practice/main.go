// Day 47: 인터페이스 실전 패턴 예제
package main

import (
	"fmt"
	"io"
	"os"
	"strings"
	"bytes"
)

// ReadString 은 io.Reader 인터페이스를 사용하여 데이터 읽기
func ReadString(reader io.Reader) (string, error) {
	buf := new(bytes.Buffer)
	_, err := buf.ReadFrom(reader)
	return buf.String(), err
}

// WriteString 은 io.Writer 인터페이스를 사용하여 데이터 쓰기
func WriteString(writer io.Writer, content string) error {
	_, err := writer.Write([]byte(content))
	return err
}

// CleanRoom 은 청소하는 행위를 추상화한 인터페이스
type CleanRoom interface {
	Clean()
}

// VacuumCleaner 는 진공청소기 구조체
type VacuumCleaner struct {
	Name string
}

func (v VacuumCleaner) Clean() {
	fmt.Printf("%s: 청소 중...\n", v.Name)
}

// Broom 는 빗자루 구조체
type Broom struct {
	Name string
}

func (b Broom) Clean() {
	fmt.Printf("%s: 청소 중...\n", b.Name)
}

// CleanHouse 는 청소 도구(Dependent)가 아닌 청소 행위(Abstract)를 받습니다.
func CleanHouse(tool CleanRoom) {
	tool.Clean()
}

func main() {
	fmt.Println("=== 1. io.Reader 패턴 ===")
	// strings.NewReader 는 io.Reader 인터페이스를 구현하고 있습니다.
	r := strings.NewReader("Hello, Go!")
	content, err := ReadString(r)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	fmt.Printf("Read Result: %s\n", content)

	fmt.Println("\n=== 2. io.Writer 패턴 ===")
	// os.Stdout 은 io.Writer 인터페이스를 구현하고 있습니다.
	w := os.Stdout
	err = WriteString(w, "Wrote to stdout!\n")
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	fmt.Println("Write Status: Completed")

	fmt.Println("\n=== 3. ISP & Accept interface return struct ===")
	// 인터페이스를 인자로 받아서 추상화된 행위를 수행
	vacuum := VacuumCleaner{Name: "Dyson V12"}
	broom := Broom{Name: "Hand Broom"}

	fmt.Printf("Cleaner 1: ")
	CleanHouse(vacuum) // VacuumCleaner 구조체를 CleanRoom 인터페이스로 받음
	fmt.Printf("Cleaner 2: ")
	CleanHouse(broom) // Broom 구조체를 CleanRoom 인터페이스로 받음
}