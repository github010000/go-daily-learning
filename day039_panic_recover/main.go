package main

import "fmt"

// unsafeOperation: 의도적으로 panic을 발생시키고 recover로 처리하는 예제
func unsafeOperation() {
	defer func() {
		// recover는 defer 블록 내부에서만 panic 값을 받을 수 있습니다.
		if err := recover(); err != nil {
			fmt.Println("[recover] panic 감지:", err)
		}
	}()

	// nil 슬라이스 인덱스 접근은 런타임 panic을 유발합니다.
	var slice []int
	_ = slice[0]
}

// libraryFunction: 라이브러리 경계에서의 안전한 recover 패턴
// 외부에서 호출되는 함수는 panic을 error로 변환하여 반환해야 합니다.
func libraryFunction(input int) (result int, err error) {
	defer func() {
		if r := recover(); r != nil {
			// recover는 함수 경계에서 panic을 error 타입으로 감싸서 반환합니다.
			err = fmt.Errorf("library panic caught: %v", r)
		}
	}()

	// 비즈니스 로직에서 검증 실패 시 panic 발생
	if input < 0 {
		panic("input must be non-negative")
	}
	return input * 2, nil
}

func main() {
	fmt.Println("=== 1. 기본 panic 및 recover 예제 ===")
	unsafeOperation()

	fmt.Println("\n=== 2. 라이브러리 경계에서의 안전한 recover 패턴 ===")
	res, err := libraryFunction(5)
	fmt.Printf("정상 입력: result=%d, err=%v\n", res, err)

	_, err = libraryFunction(-3)
	fmt.Printf("오류 입력: err=%v\n", err)

	fmt.Println("\n=== 3. recover 후 실행 흐름 확인 ===")
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("[recover] main.defer에서 panic 처리:", r)
		}
	}()

	fmt.Println("panic 발생 전 실행되는 코드")
	panic("의도적 테스트 패닉")
	// 이 줄은 실행되지 않습니다.
	fmt.Println("이 줄은 절대 실행되지 않습니다.")
}