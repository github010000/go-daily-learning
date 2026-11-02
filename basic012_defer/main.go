package main

import (
	"fmt"
	"os"
)

// 실수 버전: defer 인수 평가 시점 오해.
// path 변수를 바꾸어도 cleanup에는 변경 전 값이 기록됩니다.
// defer는 "실행 시점"을 예약하는 것이지 "인수를 지연 평가"하지 않습니다.
func demoDeferArgEvalError() {
	path := "/tmp/foo.txt"
	fmt.Println("[실수] path를 먼저 '/tmp/foo.txt'로 설정")

	// 이 줄에서 path의 값이 즉시 복사되어 defer 스택에 저장됩니다.
	defer fmt.Println("cleanup:", path)

	path = "/tmp/bar.txt"
	fmt.Println("working path:", path)
}

// 고친 버전: 함수 리터럴을 defer하여 path 읽기를 실행 시점까지 미룹니다.
// path를 즉시 읽지 않으므로 cleanup 시점의 최신 값을 볼 수 있습니다.
func demoDeferArgEvalFixed() {
	path := "/tmp/foo.txt"
	fmt.Println("[고친] path를 먼저 '/tmp/foo.txt'로 설정")

	// 함수 리터럴 내부에서 path를 읽으므로, cleanup이 실행될 때
	// path가 이미 '/tmp/bar.txt'로 바뀌어 있어도 그 값을 가져옵니다.
	defer func() { fmt.Println("cleanup:", path) }()

	path = "/tmp/bar.txt"
	fmt.Println("working path:", path)
}

// 실수 버전: 루프 안에서 defer로 파일 닫기.
// long-running 서비스에서는 파일 디스크립터가 계속 쌓여 고갈될 수 있습니다.
func demoDeferInLoopError() {
	fmt.Println("[실수] 루프 안에서 defer로 파일 닫기")

	for i := 0; i < 3; i++ {
		f, err := os.CreateTemp("", "defer-loop-*.txt")
		if err != nil {
			fmt.Println("create error:", err)
			return
		}
		fmt.Fprintf(f, "hello %d\n", i)

		// 루프 iteration마다 defer가 함수(main) 종료 시까지 보류됩니다.
		// 3개 파일 모두 main() 끝에서 한꺼번에 닫힙니다.
		defer f.Close()
		fmt.Printf("step %d: created %s\n", i, f.Name())
	}
	fmt.Println("loop finished — defer are pending until here")
}

// 고친 버전: defer를 즉시 실행되는 익명 함수 안에서 defer합니다.
// 익명 함수가 끝나면 defer가 즉시 실행되므로 루프마다 자원이 정리됩니다.
func demoDeferInLoopFixed() {
	fmt.Println("[고친] 루프 안에서 즉시 실행되는 함수로 defer 감싸기")

	for i := 0; i < 3; i++ {
		func() {
			f, err := os.CreateTemp("", "defer-loop-fixed-*.txt")
			if err != nil {
				fmt.Println("create error:", err)
				return
			}
			defer f.Close() // 이 함수가 끝나면 바로 실행됩니다

			fmt.Fprintf(f, "hello %d\n", i)
			fmt.Printf("step %d: created and closed %s\n", i, f.Name())
		}()
	}
	fmt.Println("loop finished — all files are already closed")
}

// LIFO 실행 순서와 named return value를 보여줍니다.
// named return은 defer가 실행될 때 직접 수정 가능합니다.
// defer가 등록된 순서와 반대(LIFO)로 실행됩니다.
func demoLIFOAndNamedReturn() (result int) {
	result = 1
	defer func() {
		result++ // defer가 named return 값을 직접 수정할 수 있습니다
	}()
	defer func() {
		result *= 10
	}()
	return result // 1을 반환하되, defer가 그 값을 바꿉니다
}

func main() {
	fmt.Println("=== defer 기초 시연 ===")

	fmt.Println(">>> 인수가 평가되는 시점")
	demoDeferArgEvalError()
	fmt.Println()
	demoDeferArgEvalFixed()
	fmt.Println()

	fmt.Println(">>> LIFO 순서와 named return")
	fmt.Println("LIFO demo 반환값:", demoLIFOAndNamedReturn())
	fmt.Println("(두 defer 중 먼저 실행된 쪽이 *10, 이후 +1)")
	fmt.Println()

	fmt.Println(">>> 루프 안 defer — 자원 누수 시연")
	demoDeferInLoopError()
	fmt.Println()
	demoDeferInLoopFixed()
	fmt.Println()

	fmt.Println("모두 정상 종료 (exit code 0)")
}