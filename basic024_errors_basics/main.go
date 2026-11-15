package main

import (
	"errors"
	"fmt"
	"strings"
)

// --- 1. sentinel error 정의 ---
// errors.New로 만든 값은 프로그램 전체에서 "동일한 사인"입니다.
// 따라서 == 비교가 가능하며, 특정 에러인지 빠르게 판정할 수 있습니다.
// 관례: 이름은 Err로 시작하고, PascalCase로 씁니다.
var (
	ErrNameEmpty    = errors.New("이름이 비어 있습니다")
	ErrNameTooShort = errors.New("이름이 너무 짧습니다")
	ErrDBDown       = errors.New("데이터베이스에 연결할 수 없습니다")
)

// --- 2. 실수한 버전: 에러를 무시하거나 늦게 체크 ---
// 에러를 _ 에 버리거나, 이미 반환값을 쓴 뒤에 체크하는 패턴을 보여줍니다.
// 이러한 실수는 호출자가 에러 상황을 모른 채 진행되게 만듭니다.
func greetingBad(name string) (string, error) {
	if name == "" {
		return "", ErrNameEmpty
	}
	if len(name) < 2 {
		return "", ErrNameTooShort
	}
	return "안녕하세요, " + name + "님!", nil
}

// callGreetingBad은 에러를 무시하는 두 가지 실수를 보여줍니다.
func callGreetingBad(name string) {
	msg, err := greetingBad(name)

	// 실수 1: 에러를 체크하지 않고 msg를 바로 출력
	fmt.Printf("[실수 1] %q -> 결과: %q\n", name, msg)

	// 실수 2: 에러를 체크하지만, 이미 msg를 사용했다
	if err != nil {
		fmt.Printf("[실수 2] (후회) 에러 발생: %v\n", err)
	}
}

// --- 3. 고친 버전: 에러가 있으면 즉시 처리 ---
// 에러가 반환되면, 정상 로직을 실행하기 전에 먼저 처리합니다.
// 이를 통해 잘못된 데이터가 시스템 깊숙이 들어가지 않습니다.
func callGreetingGood(name string) {
	msg, err := greetingBad(name)
	if err != nil {
		// 에러가 있으면 즉시 return. 정상 로직은 실행되지 않음.
		fmt.Printf("[고친 버전] %q -> 에러: %v\n", name, err)
		return
	}
	fmt.Printf("[고친 버전] %q -> 성공: %s\n", name, msg)
}

// --- 4. 에러 메시지 관례 ---
// Go에서 에러 메시지 작성 관례를 보여줍니다.
// 관례: 소문자 시작, 마침표 없음, 'Error:' 접두사 없음.
func demonstrateMessageConventions() {
	fmt.Println("\n=== 에러 메시지 작성 관례 ===")

	// 올바른 에러 메시지
	fmt.Println("올바른 에러 메시지 (소문자 시작, 마침표 없음):")
	fmt.Printf("  - %q\n", ErrNameEmpty)
	fmt.Printf("  - %q\n", ErrNameTooShort)

	// 잘못된 에러 메시지 (관례 위반)
	fmt.Println("\n피해야 할 에러 메시지:")
	fmt.Printf("  - %q (대문자 시작, 마침표 있음)\n", "Error: 이름이 비어 있습니다.")
	fmt.Printf("  - %q ('Error:' 접두사 있음)\n", "에러 발생: 이름이 너무 짧습니다!")
}

// --- 5. 에러 비교 방법 ---
// Go에는 에러를 비교하는 세 가지 주요 방법이 있습니다.
func demonstrateErrorChecking() {
	fmt.Println("\n=== 에러 비교하기 ===")

	// [1] nil 체크: 가장 기본적이고 흔한 방법
	fmt.Println("\n[1] nil 체크 (가장 흔함)")
	name := ""
	msg, err := greetingBad(name)
	if err != nil {
		fmt.Printf("  에러 발생: %v\n", err)
	} else {
		fmt.Printf("  성공: %s\n", msg)
	}

	// [2] == 비교 (sentinel error): errors.New로 만든 에러만 가능
	fmt.Println("\n[2] == 비교 (sentinel 에러)")
	name = "김"
	_, err = greetingBad(name)
	if err == ErrNameTooShort {
		fmt.Printf("  에러 종류: %v (ErrNameTooShort와 동일)\n", err)
	}

	// [3] errors.Is: 에러 체인(wrapped error)에서 원본 에러 탐색
	fmt.Println("\n[3] errors.Is (에러 체인 검사)")
	err = wrapDBError("user123")
	if errors.Is(err, ErrDBDown) {
		fmt.Println("  에러 종류: 데이터베이스 오류 (errors.Is로 탐색)")
	}

	// [4] 에러 무시의 위험
	fmt.Println("\n[4] 에러 무시 (_ 사용)의 위험")
	name = "박고고"
	msg, _ = greetingBad(name)
	fmt.Printf("  무시하고 사용: %s (에러 정보 손실)\n", msg)
}

// --- 6. 에러 감싸기 (wrapping) ---
// fmt.Errorf의 %w 동사를 쓰면 에러에 컨텍스트를 추가하고,
// errors.Is로 원래 에러를 찾을 수 있는 체인을 만들 수 있습니다.
func wrapDBError(userID string) error {
	if userID == "" {
		return ErrNameEmpty
	}
	if userID == "user123" {
		// %w를 사용해 ErrDBDown을 감쌉니다.
		return fmt.Errorf("사용자 %s 처리 중 오류: %w", userID, ErrDBDown)
	}
	return nil
}

func main() {
	fmt.Println("=== 에러 처리 기초 ===")

	// 1. 실수한 버전 vs 고친 버전 비교
	fmt.Println("\n--- 1. 에러 무시 vs 에러 처리 ---")
	callGreetingBad("")
	callGreetingGood("")
	callGreetingBad("김")
	callGreetingGood("김")
	callGreetingBad("박고고")
	callGreetingGood("박고고")

	// 2. 에러 비교 방법 시연
	demonstrateErrorChecking()

	// 3. 에러 메시지 관례 시연
	demonstrateMessageConventions()

	fmt.Println("\n=== 완료 ===")
}