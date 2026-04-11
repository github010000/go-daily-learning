package main

import (
	"fmt"
	"strings"
)

// Day 8: switch 문의 다양한 사용법을 보여주는 예제
func main() {
	fmt.Println("======================================")
	fmt.Println("Day 8: Go switch 문 학습 시작")
	fmt.Println("======================================")


	// 1. 기본 case 분기 (자동 break)
	fmt.Println("
--- 1. 기본 Case 분기 (자동 break) ---")
	score := 85
	grade := "미지정"
	
	switch {
	case score >= 90:
		grade = "A"
	case score >= 80 && score < 90:
		grade = "B"
	case score >= 70 && score < 80:
		grade = "C"
	case score >= 60 && score < 70:
		grade = "D"
	default:
		grade = "F"
	}
	fmt.Printf("점수 %d점의 학점은 %s 입니다. (자동으로 해당 case를 빠져나옵니다)\n", score, grade)


	// 2. Fallthrough 명시 및 활용 (명시적 break 필요)
	fmt.Println("
--- 2. Fallthrough 명시 및 활용 ---")	
	dayOfWeek := 3 // 3: 수요일
	dayName := ""

	switch dayOfWeek {
	case 1:
		dayName = "월요일"
	case 2:
		dayName = "화요일"
	case 3:
		dayName = "수요일"
	fallthrough // 🚨 주의: 다음 case로 넘어감
	case 4:
		dayName += "/수요일 (삼일절)"
	case 5:
		dayName = "금요일"
	default:
		dayName = "알 수 없는 요일"
	}
	fmt.Printf("요일 %d에 해당하는 이름: %s (Fallthrough가 발생하여 값이 누적되었습니다)\n", dayOfWeek, dayName)


	// 3. 타입 기반 switch (Type Switch)
	fmt.Println("
--- 3. 타입 기반 switch (Type Switch) ---")
	var data interface{} = "Hello Go"
	
	switch v := data.(type) {
	case string:
		fmt.Printf("타입이 string 입니다. 값의 길이: %d
", len(v))
	case int:
		fmt.Printf("타입이 int 입니다. 값: %d
", v)
	case float64:
		fmt.Printf("타입이 float64 입니다. 값: %.2f
", v)
	default:
		fmt.Printf("알 수 없는 타입입니다. 타입 정보: %T
", v)
	}


	// 4. 조건식 switch (Boolean Switch - 조건만 사용)
	fmt.Println("
--- 4. 조건식 switch (Boolean Switch) ---")	
	isEven := true
	isPositive := true
	
	switch true {
	case isEven == true, isPositive == true: // 두 조건 모두 참일 때
		fmt.Println("✅ 짝수이고 양수입니다.")
	case isEven == true: // 짝수이고 양수가 아닐 때 (isPositive == false)
		fmt.Println("✅ 짝수이지만 양수가 아닙니다 (0 또는 음수).")
	case isPositive == true: // 홀수이고 양수일 때 (isEven == false)
		fmt.Println("✅ 홀수이고 양수입니다.")	
	default:
		fmt.Println("❌ 짝수도 아니고 양수도 아닙니다.")
	}

	fmt.Println("======================================")
	fmt.Println("모든 switch 문 테스트 완료!")
