package main

import (
	"fmt"
	"strings"
)

// =============================================================================
// 1. recover는 defer 안에서만 동작합니다.
// =============================================================================

// safeDivide는 분모가 0인 panic을 recover하여 0을 반환합니다.
// recover는 defer 블록 안에서만 panic을 잡을 수 있습니다.
func safeDivide(a, b int) int {
	defer func() {
		// recover()는 현재 실행 중인 goroutine의 panic 체인을 검사합니다.
		// panic이 defer가 실행되는 시점(스택이 내려오는 중)에 발생했어야 합니다.
		if r := recover(); r != nil {
			fmt.Printf("  [safeDivide] panic 복구: %v → 0 반환\n", r)
		}
	}()
	if b == 0 {
		// b가 0이면panic이 발생합니다.
		// 이 panic은 위에서 등록한 defer가 LIFO 순서로 실행되면서 recover됩니다.
		panic("0으로 나눌 수 없습니다")
	}
	return a / b
}

// recoverInWrongPlace는 defer 밖에서 recover를 호출하면 panic을 잡지 못함을 보여줍니다.
func recoverInWrongPlace() {
	defer fmt.Println("  [defer 1] panic 전 등록된 defer")
	defer fmt.Println("  [defer 2] panic 후 실행되는 defer")

	// recover를 defer가 아닌, 직접 호출합니다.
	// recover는 현재panic 체인이 없으므로 nil을 반환합니다.
	// 즉, 아무것도 잡지 못하고 계속 진행됩니다.
	result := recover()
	fmt.Printf("  recover() 호출 결과: %v (panic이 없으므로 nil)\n", result)

	panic("잡히지 않는panic")
}

// =============================================================================
// 2. goroutine의 panic은 다른 goroutine이 못 잡습니다.
// =============================================================================

// unguardedGoroutinePanics는 recover가 없으면 프로그램을 종료시킵니다.
func unguardedGoroutinePanics() {
	go func() {
		// 이 goroutine 안에 defer/recover가 없으므로
		// panic이 전파되어 프로그램 전체가 종료됩니다.
		panic("고루틴 내부 미처리panic")
	}()
}

// guardedGoroutinePanics는 고루틴 내부에서 recover하여 안전하게 처리합니다.
func guardedGoroutinePanics() {
	done := make(chan string, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				// 고루틴 내부에서 recover하면, 이 고루틴만 종료되고
				// 메인 고루틴은 계속 실행됩니다.
				done <- fmt.Sprintf("고루틴panic 복구: %v", r)
			}
		}()
		panic("고루틴 내부panic")
	}()
	// 고루틴이 종료될 때까지 기다립니다(결과를 채널로 받습니다).
	fmt.Println(<-done)
}

// =============================================================================
// 3. panic을 써도 되는 경우: 초기화 실패, 불변조건 위반
// =============================================================================

// newConfig는 필수 설정이 없으면panic을 일으킵니다.
// 이 값이 없으면 애플리케이션이 정상적으로 동작할 수 없으므로 panic이 적절합니다.
func newConfig(host string, port int) map[string]interface{} {
	if host == "" {
		panic("Config: host는 필수입니다")
	}
	if port <= 0 || port > 65535 {
		panic("Config: port는 1~65535 사이여야 합니다")
	}
	return map[string]interface{}{
		"host": host,
		"port": port,
	}
}

// =============================================================================
// 4. panic을 써서는 안 되는 경우: 일반적인 에러
// =============================================================================

// readFile은 파일이 없으면 error를 반환합니다.
// panic을 쓰면 호출자가 에러를 처리할 수 없고 프로그램이 죽을 수 있습니다.
func readFile(path string) (string, error) {
	// 실제 파일 읽기 대신, 존재하는 경우만 시뮬레이션합니다.
	if strings.Contains(path, "missing") {
		// error를 반환하는 것이 Go의 관용적 패턴입니다.
		return "", fmt.Errorf("파일을 찾을 수 없습니다: %s", path)
	}
	return "파일 내용", nil
}

// =============================================================================
// 5. panic과 error의 혼동: 실수 예시
// =============================================================================

// divideErrorVersion는 에러를 리턴해야 할 곳에panic을 쓰는 실수를 보여줍니다.
// (이 함수는 직접 호출하지 않고, main에서 시연용으로만 사용합니다)
func divideErrorVersion(a, b int) int {
	// 에러 리턴을 써야 할 곳에 panic을 씁니다.
	if b == 0 {
		panic("나눗셈 실패")
	}
	return a / b
}

// =============================================================================
// main: 모든 시연을 orchestrate합니다.
// =============================================================================

func main() {
	// --- 1. recover는 defer 안에서만 동작합니다 ---
	fmt.Println("=== 1. recover는 defer 안에서만 동작합니다 ===")
	fmt.Println()

	fmt.Println("[고친 버전] safeDivide (defer 안에서 recover)")
	fmt.Printf("  10 / 2 = %d\n", safeDivide(10, 2))
	fmt.Printf("  10 / 0 = %d\n", safeDivide(10, 0))
	fmt.Println()

	fmt.Println("[실수 버전] recover를 defer 밖에서 호출")
	func() {
		defer func() {
			if r := recover(); r != nil {
				// main의 defer가 이panic을 잡습니다.
				fmt.Printf("  [main recover] panic 복구: %v\n", r)
			}
		}()
		recoverInWrongPlace()
	}()
	fmt.Println()

	// --- 2. goroutine의 panic ---
	fmt.Println("=== 2. goroutine의 panic ===")
	fmt.Println()

	fmt.Println("[실수 버전] unguardedGoroutinePanics (프로그램 종료)")
	fmt.Println("  이 라인은 고루틴panic으로 인해 출력되지 않습니다.")
	// NOTE: 이 함수를 호출하면 프로그램이 종료됩니다.
	// 시연용으로 보여주지 않고, 설명으로만 남겨두거나,
	// defer/recover로 감싸서 보여줄 수 있습니다.
	// 여기서는 "이런 코드가 있으면 프로그램이 죽는다"는 점을 설명합니다.
	fmt.Println("  (위 함수를 호출하면 프로그램이 종료됩니다)")
	fmt.Println()

	fmt.Println("[고친 버전] guardedGoroutinePanics (고루틴 내부에서 recover)")
	guardedGoroutinePanics()
	fmt.Println("  [메인] 고루틴panic이 복구되었으므로 계속 실행됩니다.")
	fmt.Println()

	// --- 3. panic을 써도 되는 경우 ---
	fmt.Println("=== 3. panic을 써도 되는 경우: 초기화/불변조건 ---")
	fmt.Println()

	fmt.Println("[고친 버전] newConfig (필수 설정 없으면panic)")
	config := newConfig("localhost", 8080)
	fmt.Printf("  설정 로드 성공: host=%v, port=%v\n", config["host"], config["port"])
	func() {
		defer func() {
			if r := recover(); r != nil {
				fmt.Printf("  [recover] panic 복구: %v\n", r)
			}
		}()
		newConfig("", 8080)
	}()
	fmt.Println()

	// --- 4. panic을 써서는 안 되는 경우 ---
	fmt.Println("=== 4. panic을 써서는 안 되는 경우: 일반적인 에러 ---")
	fmt.Println()

	fmt.Println("[고친 버전] readFile (error 리턴 사용)")
	content, err := readFile("data.txt")
	fmt.Printf("  파일 읽기 성공: %q\n", content)
	content, err = readFile("missing.txt")
	if err != nil {
		fmt.Printf("  파일 읽기 에러(정상 처리): %v\n", err)
	}
	fmt.Println()

	fmt.Println("[실수 버전] divideErrorVersion (일반 에러에panic)")
	fmt.Println("  호출자가 panic을 recover하지 않으면 프로그램이 죽습니다.")
	func() {
		defer func() {
			if r := recover(); r != nil {
				fmt.Printf("  [recover] panic 복구: %v\n", r)
			}
		}()
		divideErrorVersion(10, 0)
	}()
	fmt.Println()

	fmt.Println("=== 시연 완료 ===")
}