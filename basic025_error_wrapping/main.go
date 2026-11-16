package main

import (
	"errors"
	"fmt"
	"os"
)

// openFileBuggy는 %v를 사용해 에러를 래핑합니다.
// 이 방법은 에러 메시지를 텍스트로 합칠 뿐, 에러 체인(wrap chain)을 형성하지 않습니다.
// 따라서 errors.Is로 내부 원인을 찾을 수 없습니다.
func openFileBuggy(path string) error {
	f, err := os.Open(path)
	if err != nil {
		// %v: 단순 문자열 결합. 에러 체인 미포함.
		return fmt.Errorf("파일 열기 실패 (%s): %v", path, err)
	}
	defer f.Close()
	_, err = os.ReadFile(path)
	if err != nil {
		// %v: 단순 문자열 결합. 에러 체인 미포함.
		return fmt.Errorf("파일 읽기 실패 (%s): %v", path, err)
	}
	return nil
}

// openFileFixed는 %w를 사용해 에러를 래핑합니다.
// 이 방법은 에러 객체를 내부에 보존하므로 errors.Is/As가 체인을 탐색할 수 있습니다.
func openFileFixed(path string) error {
	f, err := os.Open(path)
	if err != nil {
		// %w: 에러 체인 생성. errors.Is/As가 내부 원인을 찾는다.
		return fmt.Errorf("파일 열기 실패 (%s): %w", path, err)
	}
	defer f.Close()
	_, err = os.ReadFile(path)
	if err != nil {
		// %w: 에러 체인 생성. errors.Is/As가 내부 원인을 찾는다.
		return fmt.Errorf("파일 읽기 실패 (%s): %w", path, err)
	}
	return nil
}

// 센티널 에러: 프로그램 전반에서 재사용하는 미리 정의한 에러 변수
var ErrNotFound = errors.New("데이터를 찾지 못함")
var ErrPermissionDenied = errors.New("권한이 없음")

// findUser는 ID에 따라 센티널 에러를 반환합니다.
func findUser(id int) error {
	if id == 0 {
		return ErrPermissionDenied
	}
	if id == 999 {
		return ErrNotFound
	}
	return nil
}

// getUser는 findUser를 감싸서 호출합니다.
// %w를 사용했으므로 errors.Is로 내부 에러를 찾을 수 있습니다.
func getUser(id int) error {
	if err := findUser(id); err != nil {
		return fmt.Errorf("사용자 조회 실패 (ID=%d): %w", id, err)
	}
	return nil
}

// checkSentinelBuggy는 == 비교의 한계를 보여줍니다.
// 래핑된 에러는 원본 에러와 == 비교에서 항상 false를 반환합니다.
func checkSentinelBuggy(id int) {
	err := getUser(id)

	// == 비교: 래핑된 에러는 센티널 에러와 같지 않다.
	if err == ErrNotFound {
		fmt.Println("  [잘못된 비교] ErrNotFound 발견 (동등 연산자)")
	} else {
		fmt.Println("  [잘못된 비교] ErrNotFound 미발견 (동등 연산자)")
	}

	// errors.Is: 에러 체인을 탐색해 찾는다.
	if errors.Is(err, ErrNotFound) {
		fmt.Println("  [올바른 비교] errors.Is로 ErrNotFound 발견")
	} else {
		fmt.Println("  [올바른 비교] errors.Is로 ErrNotFound 미발견")
	}
}

// CustomNotFoundError는 사용자 정의 에러 타입입니다.
type CustomNotFoundError struct {
	ID  int
	Msg string
}

// Error 메서드를 구현해 error 인터페이스를 만족시킵니다.
func (e *CustomNotFoundError) Error() string {
	return e.Msg
}

// lookupUser는 음수 ID에 대해 사용자 정의 에러를 반환합니다.
func lookupUser(id int) error {
	if id < 0 {
		return &CustomNotFoundError{ID: id, Msg: fmt.Sprintf("ID %d: 존재하지 않음", id)}
	}
	return nil
}

// checkCustomError는 errors.As를 사용해 구체 에러 타입을 추출합니다.
func checkCustomError(id int) {
	err := lookupUser(id)
	if err == nil {
		fmt.Println("  [CustomError] 사용자 찾음")
		return
	}

	// errors.As: 에러가 *CustomNotFoundError 타입인가?
	var notFound *CustomNotFoundError
	if errors.As(err, &notFound) {
		fmt.Printf("  [CustomError] 발견! ID=%d, Msg=%q\n", notFound.ID, notFound.Msg)
	} else {
		fmt.Printf("  [CustomError] 다른 에러: %v\n", err)
	}
}

// safeAsCheck는 nil 에러에서 errors.As가 안전하게 동작함을 보여줍니다.
func safeAsCheck(id int) {
	err := lookupUser(10) // nil 반환

	var notFound *CustomNotFoundError
	if errors.As(err, &notFound) {
		fmt.Printf("  [Safe] CustomError 발견: %v\n", notFound)
	} else {
		fmt.Println("  [Safe] CustomError 아님 (nil 포함)")
	}
}

func main() {
	fmt.Println("=== 1. 에러 래핑: %v vs %w ===")
	// %v 버전: Is로 찾아지지 않는다
	errBuggy := openFileBuggy("nonexistent.txt")
	fmt.Printf("  [%s 래핑] ErrNotExist 직접 비교: %v\n", "v", errors.Is(errBuggy, os.ErrNotExist))
	fmt.Printf("  [%s 래핑] 에러 메시지: %v\n", "v", errBuggy)

	// %w 버전: Is로 찾을 수 있다
	errFixed := openFileFixed("nonexistent.txt")
	fmt.Printf("  [%s 래핑] ErrNotExist 직접 비교: %v\n", "w", errors.Is(errFixed, os.ErrNotExist))
	fmt.Printf("  [%s 래핑] 에러 메시지: %v\n", "w", errFixed)

	fmt.Println("\n=== 2. 센티널 에러: == vs errors.Is ===")
	checkSentinelBuggy(999)
	checkSentinelBuggy(0)

	fmt.Println("\n=== 3. errors.As: 타입 추출 ===")
	checkCustomError(-1)
	checkCustomError(10)

	fmt.Println("\n=== 4. nil 안전성 ===")
	safeAsCheck(10)
}