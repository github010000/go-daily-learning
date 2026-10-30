package main

import (
	"fmt"
)

// scoreToGrade — 숫자 점수를 문자열 등급으로 변환합니다.
// Go switch는 case를 실행한 뒤 자동 종료(break)하므로, return이 없어도 다음 case로 넘어가지 않습니다.
// 초보자는 "break를 빼먹었나?" 싶어서 불안해하는데, 이건 Go의 설계 의도입니다.
func scoreToGrade(score int) string {
	switch {
	case score >= 90:
		return "A"
	case score >= 80:
		return "B"
	case score >= 70:
		return "C"
	case score >= 60:
		return "D"
	default:
		return "F"
	}
}

// scoreToGradeWithBreak — 실수 버전: case 안에서 break를 쓰는 경우.
// Go에서 switch case 내부의 break는 "현재 case 종료"가 아니라 "바깥 루프 종료"로 동작합니다.
// 따라서 이 함수는 의도한 대로 작동하지 않습니다.
func scoreToGradeWithBreak(score int) string {
	var grade string
	switch {
	case score >= 90:
		grade = "A"
		break
	case score >= 80:
		grade = "B"
		break
	default:
		grade = "F"
	}
	return grade
}

// scoreToGradeWithFallthrough — fallthrough 사용 버전.
// case를 실행한 뒤 다음 case의 코드를 강제로 실행합니다.
// break 대신 fallthrough를 쓰는 이유는, "이 값이 이전 case에도 해당하니까" 같은 논리적 연쇄 때문입니다.
// 하지만 가독성이 떨어져서 실제 프로젝트에서는 거의 쓰이지 않습니다.
func scoreToGradeWithFallthrough(score int) string {
	var grade string
	switch {
	case score >= 90:
		grade = "A"
		fallthrough
	case score >= 80:
		grade = "B"
		fallthrough
	case score >= 70:
		grade = "C"
	case score >= 60:
		grade = "D"
	default:
		grade = "F"
	}
	return grade
}

// classifyNumber — 조건 없는 switch로 if-else 사슬을 대체합니다.
// switch 뒤에 조건식을 쓰지 않으면, 각 case가 bool 조건식이 됩니다.
// Go 1.22부터는 for 루프 변수도 case에서 쓸 수 있어서 더 유용해졌습니다.
func classifyNumber(n int) string {
	switch {
	case n < 0:
		return "음수"
	case n == 0:
		return "영"
	case n > 0 && n <= 10:
		return "1~10 사이"
	case n > 10 && n <= 100:
		return "11~100 사이"
	default:
		return "100 초과"
	}
}

// classifyNumberIfElse — 같은 로직을 if-else로 쓴 버전 (비교용).
func classifyNumberIfElse(n int) string {
	if n < 0 {
		return "음수"
	} else if n == 0 {
		return "영"
	} else if n > 0 && n <= 10 {
		return "1~10 사이"
	} else if n > 10 && n <= 100 {
		return "11~100 사이"
	} else {
		return "100 초과"
	}
}

// monthDays — 여러 값을 하나의 case에 묶어서 사용합니다.
// case 1, 3, 5: 는 쉼표로 나열하면 됩니다.
func monthDays(month int) int {
	switch month {
	case 1, 3, 5, 7, 8, 10, 12:
		return 31
	case 4, 6, 9, 11:
		return 30
	case 2:
		return 28
	default:
		return 0
	}
}

// describeWeekday — 요일 이름을 숫자로 변환합니다.
// case에 여러 값을 나열하는 예시입니다.
func describeWeekday(day int) string {
	switch day {
	case 1, 2, 3, 4, 5:
		return "평일"
	case 6, 7:
		return "주말"
	default:
		return "잘못된 요일"
	}
}

// demonstrateBreakMistake — break를 case 안에서 잘못 사용한 예시.
// Go에서 switch case 내부의 break는 루프를 빠져나가는 동작을 합니다.
// 따라서 이 함수는 의도한 대로 작동하지 않습니다.
func demonstrateBreakMistake() {
	fmt.Println("  [잘못된 버전] break를 case 안에서 사용")
	scores := []int{95, 82, 70}
	for _, s := range scores {
		grade := scoreToGradeWithBreak(s)
		fmt.Printf("    score=%2d → grade=%s (A/B/F 기대)\n", s, grade)
	}
	fmt.Println("    (break를 쓰면 루프가 즉시 종료됨 — 의도치 않은 동작)")
}

// demonstrateFallthroughMistake — fallthrough를 잘못 사용한 예시.
// case를 나눌 필요도 없이 fallthrough를 쓰면, 의도치 않은 case가 실행됩니다.
func demonstrateFallthroughMistake() {
	fmt.Println("  [잘못된 버전] 불필요한 fallthrough")
	score := 85
	var grade string
	switch {
	case score >= 90:
		grade = "A"
		fallthrough // 85 >= 90 은 false인데도 실행됨!
	case score >= 80:
		grade = "B"
	case score >= 70:
		grade = "C"
	}
	fmt.Printf("    score=%2d → grade=%s (A가 아닌데도 fallthrough로 B가 할당됨)\n", score, grade)
	fmt.Println("    (이건 버그 — case 조건이 false여도 fallthrough는 무조건 다음 case를 실행함)")
}

func main() {
	fmt.Println("=== 1. 기본 switch (자동 break) ===")
	fmt.Println("  case를 실행한 뒤 자동 종료됨. break를 안 써도 됨.")
	scores := []int{95, 82, 70, 55, 100}
	for _, s := range scores {
		fmt.Printf("  scoreToGrade(%2d) = %s\n", s, scoreToGrade(s))
	}

	fmt.Println("\n=== 2. break 오용 시연 (루프 종료) ===")
	demonstrateBreakMistake()

	fmt.Println("\n=== 3. fallthrough 오용 시연 ===")
	demonstrateFallthroughMistake()

	fmt.Println("\n=== 4. 조건 없는 switch (if-else 사슬 대체) ===")
	fmt.Println("  switch 뒤에 조건 없이 case에 bool 조건식 사용")
	numbers := []int{-5, 0, 5, 50, 150}
	for _, n := range numbers {
		fmt.Printf("  classifyNumber(%3d) = %s (if-else: %s)\n",
			n,
			classifyNumber(n),
			classifyNumberIfElse(n))
	}

	fmt.Println("\n=== 5. 여러 값을 하나의 case에 ===")
	fmt.Println("  case 1, 3, 5: 형식으로 쉼표 나열")
	for _, m := range []int{1, 2, 4, 6, 9, 12} {
		fmt.Printf("  monthDays(%2d) = %2d\n", m, monthDays(m))
	}
	fmt.Println("  요일:")
	for _, d := range []int{1, 3, 6, 7, 8} {
		fmt.Printf("  describeWeekday(%d) = %s\n", d, describeWeekday(d))
	}
}