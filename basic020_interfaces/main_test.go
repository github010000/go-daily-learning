package main

import (
	"io"
	"strings"
	"testing"
)

// TestSmallReader는 smallReader가 io.Reader를 올바르게 구현하는지 검증합니다.
// 인터페이스의 암묵적 구현과 기본 읽기 동작을 확인합니다.
func TestSmallReader(t *testing.T) {
	tests := []struct {
		name     string
		data     string
		expected string
	}{
		{"일반 데이터", "Hello, Go!", "Hello, Go!"},
		{"빈 데이터", "", ""},
		{"특수 문자", "한글🇰🇷Emoji🚀", "한글🇰🇷Emoji🚀"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &smallReader{data: tt.data}
			content, err := io.ReadAll(r)
			if err != nil && err != io.EOF {
				t.Fatalf("예상치 못한 오류: %v", err)
			}
			if got := string(content); got != tt.expected {
				t.Errorf("읽은 내용 = %q, 원하는 값 = %q", got, tt.expected)
			}
		})
	}
}

// TestWriteCounter는 writeCounter가 io.Writer를 올바르게 구현하는지 검증합니다.
func TestWriteCounter(t *testing.T) {
	w := &writeCounter{}
	n, err := w.Write([]byte("Test "))
	if err != nil {
		t.Fatalf("Write 오류: %v", err)
	}
	if n != 5 {
		t.Errorf("작성된 바이트 수 = %d, 원하는 값 = 5", n)
	}

	_, err = w.Write([]byte("Writer"))
	if err != nil {
		t.Fatalf("Write 오류: %v", err)
	}

	expected := "Test Writer"
	if got := w.buf.String(); got != expected {
		t.Errorf("버퍼 내용 = %q, 원하는 값 = %q", got, expected)
	}
}

// TestPrintReaderContent는 인터페이스를 통한 읽기 동작이 panic 없이 실행되는지 검증합니다.
func TestPrintReaderContent(t *testing.T) {
	r := &smallReader{data: "Test content"}
	// 함수가 panic 없이 실행되어야 합니다
	printReaderContent(r)
}

// TestEmptyInterfaceTypeSwitch는 any 타입에서의 타입 스위칭이 올바르게 동작하는지 검증합니다.
func TestEmptyInterfaceTypeSwitch(t *testing.T) {
	tests := []struct {
		name     string
		input    any
		expected string
	}{
		{"문자열 처리", "hello", "문자열"},
		{"정수 처리", 42, "정수"},
		{"부동소수점 처리", 3.14, "부동소수점"},
		{"불리언 처리", true, "불리언"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result string
			switch v := tt.input.(type) {
			case string:
				result = "문자열"
			case int:
				result = "정수"
			case float64:
				result = "부동소수점"
			case bool:
				result = "불리언"
			default:
				result = "기타"
			}

			if result != tt.expected {
				t.Errorf("타입 = %T, 결과 = %q, 원하는 값 = %q", tt.input, result, tt.expected)
			}
		})
	}
}

// TestMultipleInterfaces는 하나의 타입이 여러 인터페이스를 구현하는지 검증합니다.
func TestMultipleInterfaces(t *testing.T) {
	rw := &readerWriter{data: "multi"}

	// io.Reader로 사용 가능
	reader := io.Reader(rw)
	content, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("Reader 사용 오류: %v", err)
	}
	if string(content) != "multi" {
		t.Errorf("Reader로 읽은 내용 = %q, 원하는 값 = %q", string(content), "multi")
	}

	// io.Writer로 사용 가능
	writer := io.Writer(rw)
	n, err := writer.Write([]byte(" interface"))
	if err != nil {
		t.Fatalf("Writer 사용 오류: %v", err)
	}
	if n != 9 {
		t.Errorf("Writer로 작성된 바이트 수 = %d, 원하는 값 = 9", n)
	}

	expected := "multi interface"
	if got := rw.buf.String(); got != expected {
		t.Errorf("최종 내용 = %q, 원하는 값 = %q", got, expected)
	}
}

// TestAnySlice은 any 슬라이스에 다양한 타입을 저장하고 처리하는 것을 검증합니다.
func TestAnySlice(t *testing.T) {
	var items []any
	items = append(items, "string")
	items = append(items, 123)
	items = append(items, 45.67)

	if len(items) != 3 {
		t.Fatalf("items 길이 = %d, 원하는 값 = 3", len(items))
	}

	if s, ok := items[0].(string); !ok || s != "string" {
		t.Errorf("첫 번째 항목 처리 오류")
	}

	if i, ok := items[1].(int); !ok || i != 123 {
		t.Errorf("두 번째 항목 처리 오류")
	}

	if f, ok := items[2].(float64); !ok || f != 45.67 {
		t.Errorf("세 번째 항목 처리 오류")
	}
}

// TestStringReader는 strings.Reader가 io.Reader를 자동으로 구현하는지 검증합니다.
func TestStringReader(t *testing.T) {
	s := strings.NewReader("strings.Reader test")
	content, err := io.ReadAll(s)
	if err != nil {
		t.Fatalf("strings.Reader 읽기 오류: %v", err)
	}

	expected := "strings.Reader test"
	if got := string(content); got != expected {
		t.Errorf("strings.Reader 내용 = %q, 원하는 값 = %q", got, expected)
	}
}

// TestNilInterface는 nil 인터페이스와 nil concrete 인터페이스의 차이를 검증합니다.
func TestNilInterface(t *testing.T) {
	// nil 인터페이스
	var nilReader io.Reader
	if nilReader != nil {
		t.Errorf("nil 인터페이스는 nil이어야 합니다")
	}

	// nil concrete 포인터를 인터페이스에 할당
	var ptr *smallReader = nil
	var ifaceReader io.Reader = ptr
	// 이 인터페이스는 nil이 아닙니다
	if ifaceReader == nil {
		t.Errorf("nil concrete 포인터를 인터페이스에 할당하면 nil이 아닙니다")
	}
}

// TestInterfaceSatisfaction는 다양한 타입이 io.Reader를 어떻게 구현하는지 검증합니다.
func TestInterfaceSatisfaction(t *testing.T) {
	tests := []struct {
		name     string
		reader   io.Reader
		expected string
	}{
		{"strings.Reader", strings.NewReader("문자열"), "문자열"},
		{"smallReader", &smallReader{data: "작은 리더"}, "작은 리더"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content, err := io.ReadAll(tt.reader)
			if err != nil && err != io.EOF {
				t.Fatalf("예상치 못한 오류: %v", err)
			}
			if got := string(content); got != tt.expected {
				t.Errorf("읽은 내용 = %q, 원하는 값 = %q", got, tt.expected)
			}
		})
	}
}