package main

import (
	"fmt"
	"os"
)

// SampleUser 구조체를 정의하여 다양한 포맷 동사 예시 제공
type SampleUser struct {
	Name string
	Age  int
}

func main() {
	fmt.Println("=== Day 28: fmt 패키지 심화 학습 ===")
	fmt.Println()

	// 1. Sprintf (문자열 포맷팅)
	// fmt.Sprintf는 포맷팅된 문자열을 반환합니다.
	name := "고라니"
	age := 5
	price := 15000

	// %s: 문자열, %d: 정수, %v: 기본값
	message := fmt.Sprintf("이름: %s, 나이: %d세, 가격: %v원", name, age, price)
	fmt.Println("1. Sprintf 예제:")
	fmt.Println(message)

	// %05d: 5자리 정수 (왼쪽 0 패딩), %x: 16진수
	hexPrice := fmt.Sprintf("가격(16진수): %x", price)
	fmt.Println(hexPrice)

	// 2. Fprintf (파일/표준출력 포맷팅)
	// fmt.Fprintf는 지정된 io.Writer로 데이터를 출력합니다.
	fmt.Println("\n2. Fprintf 예제 (표준출력):")
	fmt.Fprintf(os.Stdout, "[Fprintf] Sprintf와 동일하게 표준출력으로 출력합니다.\n")

	// 3. Errorf (오류 메시지 생성)
	// fmt.Errorf는 오류 메시지를 반환합니다.
	fmt.Println("\n3. Errorf 예제:")
	err := fmt.Errorf("사용자 %s의 나이가 유효하지 않습니다 (나이: %d)", name, age)
	fmt.Println(err)

	// 4. 다양한 포맷 동사 (Format Verbs)
	fmt.Println("\n4. 포맷 동사 (Format Verbs) 예제:")
	user := SampleUser{Name: "고라니", Age: 5}

	// %v: 기본 값
	fmt.Printf("기본 값 (%v): %+v\n", user, user)
	// %+v: 구조체의 필드 이름 포함
	fmt.Printf("필드 이름 포함 (%+v): %+v\n", user, user)
	// %#v: Go 문법 표현 (전체)
	fmt.Printf("Go 문법 표현 (%#v): %#v\n", user, user)
	// %T: 데이터 타입
	fmt.Printf("데이터 타입 (%T): %T\n", user, user)
	// %q: 큰따옴표 감싸기 (quoting)
	fmt.Printf("큰따옴표 감싸기 (%q): %q\n", "Hello, World!", "Hello, World!")
	// %x: 16진수
	fmt.Printf("16진수 (%x): %x\n", 255)
	// %s: 문자열
	fmt.Printf("문자열 (%s): %s\n", "Hello", "Hello")
	// %d: 정수
	fmt.Printf("정수 (%d): %d\n", 10, 10)

	// 5. Scanf로 입력 처리
	fmt.Println("\n5. Scanf로 입력 처리:")
	var inputName string
	var inputAge int

	fmt.Print("이름과 나이를 공백으로 구분하여 입력하세요 (예: 고라니 5): ")
	// fmt.Scanf는 표준입력에서 데이터를 읽고 변수에 저장합니다.
	// 주의: 실제 개발 환경이나 테스트 시 입력이 없는 경우 에러가 발생할 수 있으므로 주의하세요.
	_, err := fmt.Scanf("%s %d", &inputName, &inputAge)
	if err != nil {
		fmt.Println("입력 처리 중 오류 발생 (입력값이 없거나 형식이 맞지 않음)")
	} else {
		fmt.Printf("Sscanf로 읽은 데이터 -> 이름: %s, 나이: %d\n", inputName, inputAge)
	}
}