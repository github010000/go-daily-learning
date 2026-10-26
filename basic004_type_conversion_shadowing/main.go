package main

import (
	"fmt"
	"strconv"
)

// demonstrateExplicitConversion 은 Go 가 Python/JS 와 달리
// 암묵적 타입 변환을 허용하지 않는 이유와 그 해결책을 보여줍니다.
func demonstrateExplicitConversion() {
	var age int = 30
	var price float64 = 19.9

	// 실수 버전: age + price
	// Go 컴파일러는 "mismatched types int and float64" 오류를 냅니다.
	// 암묵적 변환이 없기 때문에 이 줄은 컴파일되지 않습니다.
	fmt.Println("실수 버전: age + price -> 컴파일 에러")

	// 고친 버전: float64(age) 로 명시적 변환
	fixedSum := price + float64(age)
	fmt.Printf("고친 버전: %d + %.2f = %.2f\n", age, price, fixedSum)

	// 문자 코드 변환: string(65) 는 유니코드 코드포인트 65 ('A') 를 반환합니다.
	// 주의: string(65) != string("65"). 전자는 문자 'A', 후자는 문자열 "65".
	fmt.Printf("string(65) -> '%s' (유니코드 65번 = 'A')\n", string(65))

	// 숫자 문자열 -> int: strconv.Atoi
	numStr := "42"
	numVal, err := strconv.Atoi(numStr)
	if err != nil {
		fmt.Printf("parse 실패: %v\n", err)
		return
	}
	fmt.Printf("strconv.Atoi('%s') -> %d\n", numStr, numVal)
}

// demonstrateShadowing 는 := 의 함정인 섀도잉을 보여줍니다.
// if 블록 안에서 err := ... 를 쓰면 바깥 err 를 가립니다.
func demonstrateShadowing() {
	// 고친 버전: 변수를 먼저 선언하고 = 로 할당
	var err error
	if err = doWork(); err != nil {
		fmt.Printf("고친 버전: 에러 발생 - %v\n", err)
	} else {
		fmt.Println("고친 버전: 작업 성공")
	}
	fmt.Printf("고친 버전 후: err = %v\n", err)

	// 실수 버전: if 안에서 := 을 쓰면 섀도잉 발생
	fmt.Println("섀도잉 실수 버전: if 안에서 := 사용")
	showShadowingDemo()
}

// showShadowingDemo 는 if 안에서 err := 를 써서
// 바깥 err 가 갱신되지 않는 섀도잉 현상을 보여줍니다.
func showShadowingDemo() {
	err := fmt.Errorf("바깥 에러")
	fmt.Printf("시뮬레이션 시작: 바깥 err = %v\n", err)

	// if 안에서 err := doWork() — 이 err 는 if 블록 내 로컬 변수입니다!
	if err := doWork(); err != nil {
		fmt.Printf("시뮬레이션: if 블록 안 err = %v\n", err)
	}
	// if 블록 밖 err 는 여전히 "바깥 에러"입니다.
	fmt.Printf("시뮬레이션 끝: 바깥 err = %v (섀도잉으로 인해 변경 없음!)\n", err)
}

// doWork 는 항상 nil 을 반환하는 더미 작업 함수입니다.
func doWork() error {
	return nil
}

func main() {
	fmt.Println("=== 타입 변환 ===")
	demonstrateExplicitConversion()
	fmt.Println()
	fmt.Println("=== 섀도잉 ===")
	demonstrateShadowing()
}