package main

import (
	"fmt"
)

// MyError는 간단한 에러 타입입니다.
// Error() 메서드를 가지고 있으므로 error 인터페이스를 구현합니다.
type MyError struct {
	Msg string
}

// Error는 error 인터페이스를 구현합니다.
func (e *MyError) Error() string {
	return e.Msg
}

// okFunc는 성공적으로 실행되면 nil을 반환합니다.
// nil 인터페이스를 반환하는 올바른 패턴입니다.
func okFunc() error {
	return nil
}

// badFunc는 실수로 nil 포인터를 반환합니다.
// 타입은 *MyError(비어있지 않음), 값은 nil입니다.
// 이는 nil이 아닌 인터페이스 값을 반환하므로 함정에 빠집니다.
func badFunc() error {
	var e *MyError // 타입은 *MyError, 값은 nil
	return e       // 반환값이 nil이 아니다! (타입이 있으므로)
}

// fixedFunc는 nil 포인터 대신 nil 인터페이스를 반환합니다.
// nil이 아닌 인터페이스 값을 만들지 않으려 노력합니다.
func fixedFunc() error {
	return nil // nil 인터페이스를 반환합니다.
}

// checkNil은 인터페이스 값이 nil인지 출력합니다.
func checkNil(name string, err error) {
	if err == nil {
		fmt.Printf("%s: nil입니다.\n", name)
	} else {
		fmt.Printf("%s: nil이 아닙니다. 에러 메시지: %v\n", name, err)
	}
}

// handleError는 에러를 처리하는 함수입니다.
// nil 체크를 통해 에러 발생 여부를 판단합니다.
func handleError(label string, err error) {
	fmt.Printf("%s: ", label)
	if err != nil {
		fmt.Printf("에러 발생: %v\n", err)
	} else {
		fmt.Println("성공")
	}
}

// nil 인터페이스 내부 구조를 설명합니다.
// 인터페이스는 (타입, 값) 두 칸으로 이루어져 있습니다.
func explainNilInterface() {
	fmt.Println("--- nil 인터페이스 내부 구조 설명 ---")
	fmt.Println("인터페이스 = (타입, 값) 두 칸으로 이루어져 있습니다.")
	fmt.Println("nil 인터페이스: (nil, nil)")
	fmt.Println("nil이 아닌 인터페이스: ( *MyError, nil )")
	fmt.Println()
}

// main은 실수 버전과 고친 버전의 차이를 출력으로 보여줍니다.
func main() {
	// 1. okFunc: 정상적인 nil 반환
	fmt.Println("--- 1. 정상적인 nil 반환 (okFunc) ---")
	checkNil("okFunc", okFunc())

	// 2. badFunc: 타입은 있지만 값이 nil인 에러 반환 (실수 버전)
	fmt.Println("\n--- 2. 타입은 있지만 값이 nil인 에러 반환 (badFunc - 실수) ---")
	checkNil("badFunc", badFunc())

	// 3. nil 인터페이스 내부 구조 설명
	explainNilInterface()

	// 4. 고친 버전: nil 포인터 대신 nil 인터페이스 반환
	fmt.Println("\n--- 3. 고친 버전: nil 포인터 대신 nil 인터페이스 반환 (fixedFunc) ---")
	checkNil("fixedFunc", fixedFunc())

	// 5. 에러 처리 시 nil 체크가 중요한 예시
	fmt.Println("\n--- 4. 에러 처리 시 nil 체크 예시 ---")
	handleError("okFunc 결과", okFunc())
	handleError("badFunc 결과 (실수)", badFunc())
	handleError("fixedFunc 결과", fixedFunc())
}