package main

import (
	"fmt"
	"io"
	"os"
	"strings"
)

// smallReader는 io.Reader 인터페이스를 암묵적으로 구현합니다.
// Go에서는 "implements" 선언 없이 Read 메서드만 있으면 인터페이스를 만족합니다.
type smallReader struct {
	data string
	pos  int
}

// Read는 p에 데이터를 채워 넣고 채운 바이트 수와 에러를 반환합니다.
func (r *smallReader) Read(p []byte) (n int, err error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	n = copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}

// writeCounter는 io.Writer 인터페이스를 암묵적으로 구현합니다.
type writeCounter struct {
	buf strings.Builder
}

func (w *writeCounter) Write(p []byte) (n int, err error) {
	return w.buf.Write(p)
}

// mistakeVersion은 인터페이스의 흔한 오해와 실수를 보여줍니다.
// 1. concrete type을 인터페이스에 할당하는 것처럼 착각
// 2. 타입 어설션에서 ok 체크 없이 사용해서 panic 발생
func mistakeVersion() {
	fmt.Println("--- 실수 버전 (주의: panic 시연 포함) ---")

	// 실수 1: 인터페이스를 concrete type으로 다루려 함
	var r io.Reader = &smallReader{data: "안녕"}
	// r.data 접근 불가: 인터페이스는 메서드만 허용, 필드 접근 불가
	// r.data는 컴파일 오류가 납니다. 아래 주석 처리한 것이 맞습니다.
	// fmt.Println(r.data) // 컴파일 오류: r.data undefined

	// 실수 2: 타입 어설션에서 ok 체크 없이 사용 -> panic
	panicDemo()

	// nil 인터페이스에 메서드를 호출하면 panic이 발생합니다.
	var nilReader io.Reader
	defer func() {
		if recovered := recover(); recovered != nil {
			fmt.Printf("nil 인터페이스 호출 panic: %v\n", recovered)
		}
	}()
	fmt.Println("nil 인터페이스 호출 시도...")
	nilReader.Read([]byte{0}) // panic 발생
	fmt.Println("이 줄은 실행되지 않습니다.")
}

// panicDemo는 타입 어설션 실수로 인한 panic을 시연합니다.
func panicDemo() {
	defer func() {
		if recovered := recover(); recovered != nil {
			fmt.Printf("패닉 발생 (예상됨): %v\n", recovered)
		}
	}()

	var i any = 123
	// int를 string으로 무작정 변환 시도 -> panic
	_ = i.(string)
}

// correctVersion은 안전한 타입 사용법을 보여줍니다.
func correctVersion() {
	fmt.Println("--- 고친 버전 ---")

	// any로부터 안전하게 값 가져오기
	var val any = "world"
	if s, ok := val.(string); ok {
		fmt.Printf("고친 버전에서 얻은 문자열: %s\n", s)
	} else {
		fmt.Println("string이 아닙니다.")
	}

	// io.Reader 구현체를 인터페이스로 전달
	r := &smallReader{data: "Go is great"}
	printReaderContent(r)

	// io.Writer 구현체를 인터페이스로 전달
	w := &writeCounter{}
	io.WriteString(w, "Hello, ")
	io.WriteString(w, "Interface!")
	fmt.Printf("작성된 내용: %s\n", w.buf.String())

	// any를 활용해 다양한 타입 처리
	showInterfaceType(42)
	showInterfaceType(3.14)
	showInterfaceType(true)
	showInterfaceType([]int{1, 2, 3})
}

// showInterfaceType는 어떤 타입이 들어오든 인터페이스가 어떻게 동작하는지 보여줍니다.
func showInterfaceType(val any) {
	fmt.Printf("값: %v (동적 타입: %T)\n", val, val)
}

// emptyInterfaceExample는 any(빈 인터페이스)의 특성을 보여줍니다.
func emptyInterfaceExample() {
	fmt.Println("--- 빈 인터페이스 (any) 예시 ---")

	var items []any
	items = append(items, "문자열")
	items = append(items, 100)
	items = append(items, true)

	for i, v := range items {
		switch x := v.(type) {
		case string:
			fmt.Printf("[%d] 문자열: %s (길이: %d)\n", i, x, len(x))
		case int:
			fmt.Printf("[%d] 정수: %d (2배: %d)\n", i, x, x*2)
		case bool:
			fmt.Printf("[%d] 불리언: %v\n", i, x)
		default:
			fmt.Printf("[%d] 알 수 없는 타입: %T\n", i, x)
		}
	}
}

// multipleInterfacesExample은 하나의 타입이 여러 인터페이스를 구현하는 경우를 보여줍니다.
func multipleInterfacesExample() {
	fmt.Println("--- 다중 인터페이스 구현 예시 ---")

	rw := &readerWriter{data: "test"}

	var reader io.Reader = rw
	var writer io.Writer = rw

	content, _ := io.ReadAll(reader)
	fmt.Printf("Reader로 읽은 내용: %s\n", string(content))

	n, err := writer.Write([]byte(" appended"))
	if err != nil {
		fmt.Printf("Write 오류: %v\n", err)
		return
	}
	fmt.Printf("Writer로 %d바이트 작성\n", n)
	fmt.Printf("최종 내용: %s\n", rw.buf.String())
}

// readerWriter는 Read와 Write를 모두 구현합니다.
type readerWriter struct {
	data string
	buf  strings.Builder
	pos  int
}

func (rw *readerWriter) Read(p []byte) (n int, err error) {
	if rw.pos >= len(rw.data) {
		return 0, io.EOF
	}
	n = copy(p, rw.data[rw.pos:])
	rw.pos += n
	return n, nil
}

func (rw *readerWriter) Write(p []byte) (n int, err error) {
	return rw.buf.Write(p)
}

// ioReaderExample는 io.Reader 인터페이스를 실제 코드에서 어떻게 활용하는지 보여줍니다.
func ioReaderExample() {
	fmt.Println("--- io.Reader 실전 예시 ---")

	// strings.Reader는 이미 io.Reader를 구현하고 있습니다.
	s := strings.NewReader("Hello, Go!")
	buf := make([]byte, 3)

	for {
		n, err := s.Read(buf)
		if n > 0 {
			fmt.Printf("읽은 데이터: %s\n", string(buf[:n]))
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			fmt.Printf("오류: %v\n", err)
			break
		}
	}
}

// osReaderExample는 os.File이 io.Reader를 자동으로 구현하는 것을 보여줍니다.
func osReaderExample() {
	fmt.Println("--- os.File (io.Reader) 실전 예시 ---")

	// 임시 파일 생성
	tmpFile, err := os.CreateTemp("", "reader-example-*.txt")
	if err != nil {
		fmt.Printf("임시 파일 생성 오류: %v\n", err)
		return
	}
	defer os.Remove(tmpFile.Name())

	// 파일에 쓰기
	_, err = tmpFile.WriteString("임시 파일 내용")
	if err != nil {
		fmt.Printf("쓰기 오류: %v\n", err)
		return
	}

	// 파일 위치를 처음으로 되돌리기
	_, err = tmpFile.Seek(0, io.SeekStart)
	if err != nil {
		fmt.Printf("seek 오류: %v\n", err)
		return
	}

	// io.Reader 인터페이스로 읽기
	content, err := io.ReadAll(tmpFile)
	if err != nil {
		fmt.Printf("읽기 오류: %v\n", err)
		return
	}
	fmt.Printf("파일 내용: %s\n", string(content))
}

// interfaceNilExample은 nil 인터페이스의 특성을 보여줍니다.
func interfaceNilExample() {
	fmt.Println("--- nil 인터페이스 예시 ---")

	// nil 인터페이스: 타입과 값 모두 nil
	var nilReader io.Reader
	fmt.Printf("nilReader는 nil: %v\n", nilReader == nil)

	// concrete pointer가 nil이더라도 인터페이스는 nil이 아닙니다!
	var ptr *smallReader = nil
	var ifaceReader io.Reader = ptr
	fmt.Printf("nil 포인터를 인터페이스에 할당: %v (nil이 아닙니다!)\n", ifaceReader)
	fmt.Printf("타입: %T, 값: %v\n", ifaceReader, ifaceReader)

	// 이 인터페이스를 사용하면 panic!
	defer func() {
		if recovered := recover(); recovered != nil {
			fmt.Printf("nil concrete 인터페이스 panic: %v\n", recovered)
		}
	}()

	fmt.Println("nil concrete 인터페이스로 Read 호출 시도...")
	ifaceReader.Read([]byte{0}) // panic
	fmt.Println("이 줄은 실행되지 않습니다.")
}

func main() {
	mistakeVersion()
	fmt.Println()
	correctVersion()
	fmt.Println()
	emptyInterfaceExample()
	fmt.Println()
	multipleInterfacesExample()
	fmt.Println()
	ioReaderExample()
	fmt.Println()
	osReaderExample()
	fmt.Println()
	interfaceNilExample()
}