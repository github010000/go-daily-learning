package main

import (
	"fmt"
	"strings"
)

// ---------------------------------------------------------------------------
// 다중 반환값 (Value, Error) 패턴
// ---------------------------------------------------------------------------

// splitName은 이름을 이름(이름) 형태로 파싱합니다.
// 성공하면 firstName, lastName을, 아니면 err를 반환합니다.
func splitName(full string) (string, string, error) {
	parts := strings.Fields(full)
	if len(parts) < 2 {
		return "", "", fmt.Errorf("이름이 부족합니다: %q", full)
	}
	return parts[0], parts[1], nil
}

// splitNameWithBlank은 실패 시 빈 문자열을 반환하는 버전입니다.
// 에러를 호출자에게 던지지 않고 내부에서 처리하는 경우의 모습입니다.
func splitNameWithBlank(full string) (firstName, lastName string) {
	parts := strings.Fields(full)
	if len(parts) < 2 {
		// firstName, lastName 은 이미 zero value(빈 문자열)로 초기화됨
		return
	}
	firstName = parts[0]
	lastName = parts[1]
	return
}

// ---------------------------------------------------------------------------
// 이름 붙은 반환값 (Named Return Values)
// ---------------------------------------------------------------------------

// countWords는 문자열에서 단어 수와 최대 길이를 반환합니다.
// 이름 붙은 반환값을 사용하여 반환 전 변수를 미리 선언한 것처럼 보이게 합니다.
func countWords(text string) (count int, maxLen int) {
	for _, word := range strings.Fields(text) {
		count++
		if len(word) > maxLen {
			maxLen = len(word)
		}
	}
	// return; // naked return: count와 maxLen을 반환
	return count, maxLen
}

// ---------------------------------------------------------------------------
// 실수 버전: defer에서의 이름 붙은 반환값 함정
// ---------------------------------------------------------------------------

// deferInFunc은 defer 함수가 이름 붙은 반환값을 "후에" 덮어쓴다는 점을 보여줍니다.
// 호출자는 결과가 예상과 다르다고 느끼게 됩니다.
func deferInFunc(success bool) (result int, err error) {
	defer func() {
		if !success {
			// err를 덮어쓰는데, return 전에 실행되므로 영향이 있음
			err = fmt.Errorf("처리 실패: %v", result)
		}
	}()

	result = 42
	if !success {
		return 0, nil // 이 시점의 err는 nil이지만, defer가 err를 "처리 실패..."로 바꿈
	}
	return result, nil
}

// ---------------------------------------------------------------------------
// 고친 버전: defer를 명확히 사용하고 반환값을 직접 제어
// ---------------------------------------------------------------------------

// deferInFuncFixed는 defer가 반환값을 어떻게 건드리는지 명확히 합니다.
// 이름 붙은 반환값 대신 일반 반환값을 사용하고, defer 내부에서 직접 err를 반환하지 않습니다.
func deferInFuncFixed(success bool) (int, error) {
	var result int
	defer func() {
		if !success {
			// 여기서는 반환값을 건드리지 않고, 그냥 로깅이나 cleanup만 수행
			fmt.Println("[defer] cleanup: result was", result)
		}
	}()

	result = 42
	if !success {
		return 0, fmt.Errorf("처리 실패: %v", result)
	}
	return result, nil
}

// ---------------------------------------------------------------------------
// panic 시연 (defer + recover로 감싸서 안전하게 보여줌)
// ---------------------------------------------------------------------------

func panicExample() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("panic 감지: %v\n", r)
		}
	}()
	panic("의도적인 panic")
}

// ---------------------------------------------------------------------------
// main: 모든 시연을 orchestrate
// ---------------------------------------------------------------------------

func main() {
	fmt.Println("=== 1. 다중 반환값 (기본 패턴) ===")
	first, last, err := splitName("Kim Min-su")
	if err != nil {
		fmt.Println("오류:", err)
	} else {
		fmt.Printf("성공: firstName=%q, lastName=%q\n", first, last)
	}

	_, _, err = splitName("Min-su") // 이름 부족
	if err != nil {
		fmt.Printf("오류 (이름 부족): %v\n", err)
	}

	fmt.Println("\n=== 2. 이름 붙은 반환값 (기본 사용) ===")
	c, m := countWords("Go is fast and simple")
	fmt.Printf("단어 수: %d, 최대 길이: %d\n", c, m)

	fmt.Println("\n=== 3. 이름 붙은 반환값 + naked return (단순 루프) ===")
	// countWords는 이미 위에서 썼지만, 이름 붙은 반환값의 Naked Return 효과를 보여주기 위해
	// 동일한 로직을 naked return 버전으로 재현
	count, maxLen := countWordsWithNaked("a bb ccc")
	fmt.Printf("단어 수: %d, 최대 길이: %d\n", count, maxLen)

	fmt.Println("\n=== 4. 실수 버전: defer에서의 이름 붙은 반환값 함정 ===")
	fmt.Println("[deferInFunc] success=true:")
	res, err := deferInFunc(true)
	fmt.Printf("  반환값: result=%d, err=%v\n", res, err)

	fmt.Println("[deferInFunc] success=false:")
	res, err = deferInFunc(false)
	fmt.Printf("  반환값: result=%d, err=%v\n", res, err)
	fmt.Println("  -> defer가 err를 덮어썼기 때문에, 반환된 err가 nil이 아님에 주목")

	fmt.Println("\n=== 5. 고친 버전: defer에서의 명확한 제어 ===")
	fmt.Println("[deferInFuncFixed] success=true:")
	res, err = deferInFuncFixed(true)
	fmt.Printf("  반환값: result=%d, err=%v\n", res, err)

	fmt.Println("[deferInFuncFixed] success=false:")
	res, err = deferInFuncFixed(false)
	fmt.Printf("  반환값: result=%d, err=%v\n", res, err)
	fmt.Println("  -> defer는 cleanup만 하고, 실제 반환값은 명시적 return으로 결정")

	fmt.Println("\n=== 6. panic 시연 (recover로 감싸서) ===")
	panicExample()
	fmt.Println("panic 후에도 프로그램은 계속 실행됩니다.")
}

// countWordsWithNaked는 countWords와 동일하지만 naked return을 명시적으로 보여줍니다.
func countWordsWithNaked(text string) (count int, maxLen int) {
	for _, word := range strings.Fields(text) {
		count++
		if len(word) > maxLen {
			maxLen = len(word)
		}
	}
	return // count와 maxLen을 그대로 반환 (naked return)
}