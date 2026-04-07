package main

import (
	"fmt"
)

func main() {
	// 1. 기본 if 문: 조건이 참일 때만 실행
	age := 18
	if age >= 18 {
		fmt.Println("투표 가능합니다.")
	}

	// 2. if / else 조합: 조건이 거짓일 때 else 블록 실행
	if age >= 20 {
		fmt.Println("술 마실 수 있는 나이입니다.")
	} else {
		fmt.Println("술 마시기 전에 기다려야 합니다.")
	}

	// 3. if / else if / else 체인: 다중 조건 처리
	score := 85
	if score >= 90 {
		fmt.Println("A 학점")
	} else if score >= 80 {
		fmt.Println("B 학점")
	} else if score >= 70 {
		fmt.Println("C 학점")
	} else {
		fmt.Println("D 학점")
	}

	// 4. if 초기화 구문 (중요!): 조건식과 동시에 변수 초기화 및 사용
	// 변수 'result'를 초기화하고, 그 값을 조건식에서 바로 사용
	result := 10
	if x := result * 2; x > 15 {
		fmt.Printf("계산 결과: %d (15보다 큽니다!)\n", x)
	}

	// 5. else 블록도 동일한 스코프를 공유 (Go의 경우, if/else if/else는 같은 블록 내에 있음)
	// 이 예제에서는 if 초기화 구문의 결과를 else 블록에서도 사용 가능
	if y := result - 3; y > 0 {
		fmt.Printf("y는 %d이며 0보다 큽니다.\n", y)
	} else {
		fmt.Printf("y는 %d이며 0 이하입니다.\n", y)
	}

	// 6. if 초기화 구문 + else 사용 예시: 오류 체크 패턴과 유사
	// 이 예시는 실제 오류는 아니지만, 'init value'와 'error status'를 흉내 낸 것
	// Go의 실제 오류 체크는 func() (value, error) 형태로 주로 쓰임
	dbStatus := "connected"
	if status := dbStatus; status == "connected" {
		fmt.Println("데이터베이스 연결 OK")
	} else {
		fmt.Println("데이터베이스 연결 실패!")
	}

	// 7. 여러 조건을 한 줄로 결합할 수 없음 (Go는 &&, || 등으로만 가능)
	// 단순 조건 표현 패턴: 변수를 사용하기 전에 조건 검사
	value := 0
	if value > 0 {
		fmt.Println("양수입니다")
	} else if value < 0 {
		fmt.Println("음수입니다")
	} else {
		fmt.Println("0입니다")
	}
}
