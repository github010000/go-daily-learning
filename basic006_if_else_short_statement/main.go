package main

import (
	"errors"
	"fmt"
	"os"
)

// errMock는 항상 에러를 반환하는 가짜 함수입니다.
func errMock() (int, error) {
	return 0, errors.New("무조건 실패")
}

// okMock은 항상 성공을 반환하는 가짜 함수입니다.
func okMock() (int, error) {
	return 42, nil
}

// negativeMock은 0 이하 값을 반환하는 가짜 함수입니다.
func negativeMock() (int, error) {
	return -10, nil
}

// 실수 버전: if 내부에서 선언한 변수를 밖으로 끌어내는 실수
func demonstrateVariableScope() {
	fmt.Println("=== 변수 범위 실수 ===")

	// [실수 버전]
	// if 문 안에서 v를 선언했지만, 밖에서 접근하려 하면 컴파일 오류가 납니다.
	// 만약 컴파일러가 이를 허용한다면, v의 수명이 if 블록 안에서만
	// 한정되어야 한다는 직관을 무너뜨립니다.
	//
	// if v, err := errMock(); err != nil {
	//     fmt.Println("에러:", err)
	// }
	// fmt.Println("v는 여기서 접근 불가:", v) // 컴파일 에러: undefined: v

	// [고친 버전]
	// 변수를 밖에서 먼저 선언하고, if 안에서 := 대신 = 를 사용합니다.
	// 이 경우 v는 if 블록을 벗어나도 유효합니다.
	var v int
	if v, err := errMock(); err != nil {
		fmt.Println("[실수 버전 시연] 에러 발생:", err)
	}
	fmt.Printf("[고친 버전] v = %d (에러가 없으면 갱신됨)\n", v)

	// [고친 버전 2]
	// if 조건 내부의 변수는 if 블록 내부에서만 살아 있습니다.
	// 이 '범위 좁히기'가 Go의 핵심 관용구입니다.
	if _, err := errMock(); err != nil {
		fmt.Println("[고친 버전 2] 에러는 잡았으니 무시합니다.")
	}
	// fmt.Println(err) // 컴파일 에러: undefined: err
}

// 실수 버전: else를 남용해 코드를 깊게 만드는 실수
func demonstrateElseStyle() {
	fmt.Println("\n=== else 스타일 실수 ===")

	// [실수 버전: 중첩 심화]
	// if -> else if -> else if -> else ... 구조는 코드를 트리에 가깝게 만듭니다.
	// Go에서는 early return을 선호합니다.
	fmt.Println("[실수 버전] else 체이닝:")
	if result, err := errMock(); err != nil {
		fmt.Println("  에러 처리:", err)
	} else if result <= 0 {
		fmt.Println("  0 이하 처리")
	} else {
		fmt.Println("  정상 처리:", result)
	}

	// [고친 버전: Early Return]
	// 각 실패 케이스를 즉시 반환하여, 정상 로직이 깊어지지 않도록 합니다.
	fmt.Println("[고친 버전] Early Return:")
	handleWithEarlyReturn()
}

// early return 스타일의 실제 구현 예시
func handleWithEarlyReturn() {
	// 1. 에러 체크 후 즉시 반환
	v, err := okMock()
	if err != nil {
		fmt.Println("  에러 발생:", err)
		return
	}

	// 2. 비즈니스 로직 실패 체크 후 즉시 반환
	if v <= 0 {
		fmt.Println("  v가 0 이하입니다")
		return
	}

	// 3. 추가 처리 (가상의 함수 g)
	if gErr := mockG(v); gErr != nil {
		fmt.Println("  g 실행 실패:", gErr)
		return
	}

	// 4. 정상 로직 (들여쓰기가 최소화된 상태)
	fmt.Println("  정상 완료! 결과:", v)
}

// 가상의 추가 처리 함수
func mockG(v int) error {
	if v > 100 {
		return errors.New("v가 너무 큼")
	}
	return nil
}

// 파일 존재 체크 예시
func demonstrateFileCheck() {
	fmt.Println("\n=== 파일 존재 체크 예시 ===")

	// 존재하지 않는 파일 경로
	nonExistentPath := "/tmp/non_existent_file_go_tour.txt"

	// 실수 버전: err가 nil일 때만 동작하는 코드를 if 내부에 넣으면
	// else 블록이 생기거나 코드가 복잡해집니다.
	fmt.Println("[실수 버전] if-else 구조:")
	if info, err := os.Stat(nonExistentPath); err == nil {
		fmt.Println("  파일 있음:", info.Name())
	} else {
		fmt.Println("  파일 없음 또는 에러:", err)
	}

	// 고친 버전: 에러가 있으면 반환 (early return)
	fmt.Println("[고친 버전] Early Return 구조:")
	handleFileCheck(nonExistentPath)
	handleFileCheck("/etc/hostname") // 존재하는 파일 (대부분의 유닉스 시스템)
}

func handleFileCheck(path string) {
	info, err := os.Stat(path)
	if err != nil {
		// 파일이 없을 때는 에러 메시지를 출력하고 종료
		if os.IsNotExist(err) {
			fmt.Printf("  '%s' 파일이 존재하지 않습니다.\n", path)
		} else {
			fmt.Printf("  '%s' 접근 중 에러: %v\n", path, err)
		}
		return
	}
	fmt.Printf("  '%s' 파일 정보: 이름=%s, 크기=%d bytes\n", path, info.Name(), info.Size())
}

// 변수 재할당 실수 시연
func demonstrateReassignment() {
	fmt.Println("\n=== 변수 재할당 실수 ===")

	x := 10

	// [실수 버전]
	// := 은 변수를 새로 선언합니다. 만약 x가 이미 선언되어 있었다면,
	// 새로운 변수를 만드는 것이 아니라 x를 재할당할 것이라 오해하기 쉽습니다.
	// 하지만 다른 변수도 함께 선언해야 재할당이 됩니다.
	// x만 단독으로 있다면? 컴파일 에러가 납니다!
	// x, y := 20, 30 -> x는 재할당, y는 새로 선언
	// x := 20 -> 컴파일 에러: no new variables on left side of :=
	
	// [고친 버전]
	// 재할당할 때는 = 를, 새로 선언할 때는 := 를 명확히 구분합니다.
	x = 20
	fmt.Printf("[고친 버전] x 재할당 후: %d\n", x)

	// [고친 버전 2]
	// 여러 변수를 한 번에 처리할 때 := 가 유용합니다.
	x, y := 30, 40
	fmt.Printf("[고친 버전 2] x=%d, y=%d\n", x, y)
}

func main() {
	fmt.Println("주제: if/else와 짧은 문장 (Short Variable Declaration)")
	fmt.Println("=========================================")

	// 1. 변수 범위 실수 및 고친 버전
	demonstrateVariableScope()

	// 2. else 스타일 실수 및 고친 버전 (Early Return)
	demonstrateElseStyle()

	// 3. 파일 존재 체크 실전 예시
	demonstrateFileCheck()

	// 4. 변수 재할당 실수 및 고친 버전
	demonstrateReassignment()

	fmt.Println("\n모든 시연이 완료되었습니다.")
}