package main

import (
	"testing"
)

// TestSafeDivide는 safeDivide가 0으로 나누는panic을 정상적으로recover하여
// 0을 반환하는지를 검증합니다.panic이 누출되면 테스트 자체가 실패합니다.
func TestSafeDivide(t *testing.T) {
	// 정상 케이스: 10/2 = 5
	if got := safeDivide(10, 2); got != 5 {
		t.Fatalf("safeDivide(10, 2) = %d, want 5", got)
	}
	//panic 케이스: 10/0 = 0 (recover가 처리)
	if got := safeDivide(10, 0); got != 0 {
		t.Fatalf("safeDivide(10, 0) = %d, want 0", got)
	}
}

// TestRecoverInWrongPlace는 recover를 defer 밖에서 호출하면panic이 전파됨을
// 검증합니다. 이panic은 테스트 함수의 defer recover가 잡아야 합니다.
func TestRecoverInWrongPlace(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("recoverInWrongPlace의panic이 recover되지 않았습니다.")
		} else {
			t.Logf("panic 복구 확인: %v", r)
		}
	}()
	recoverInWrongPlace()
}

// TestGuardedGoroutinePanics는 고루틴 내부에서recover하면 메인 고루틴이
// 계속 실행됨을 검증합니다.
func TestGuardedGoroutinePanics(t *testing.T) {
	done := make(chan string, 1)
	go func() {
		guardedGoroutinePanics()
		close(done)
	}()
	<-done
	// 고루틴이 정상 종료되면 테스트 통과
}

// TestNewConfig는 newConfig의 정상 동작과panic 발생 케이스를 검증합니다.
func TestNewConfig(t *testing.T) {
	t.Run("정상 케이스", func(t *testing.T) {
		config := newConfig("localhost", 8080)
		if config["host"] != "localhost" {
			t.Fatalf("host = %v, want localhost", config["host"])
		}
		if config["port"] != 8080 {
			t.Fatalf("port = %v, want 8080", config["port"])
		}
	})
	t.Run("host 비어있으면panic", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Fatal("newConfig(\"\", 8080)이panic을 일으키지 않았습니다.")
			}
		}()
		newConfig("", 8080)
	})
	t.Run("port 범위 초과면panic", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Fatal("newConfig(\"localhost\", 0)이panic을 일으키지 않았습니다.")
			}
		}()
		newConfig("localhost", 0)
	})
}

// TestReadFile은 readFile이 존재하지 않는 파일에 대해error를 반환하는지를
// 검증합니다. (panic을 일으키지 않아야 합니다.)
func TestReadFile(t *testing.T) {
	t.Run("정상 파일", func(t *testing.T) {
		content, err := readFile("data.txt")
		if err != nil {
			t.Fatalf("readFile(\"data.txt\") 에러: %v", err)
		}
		if content != "파일 내용" {
			t.Fatalf("content = %q, want \"파일 내용\"", content)
		}
	})
	t.Run("없는 파일", func(t *testing.T) {
		content, err := readFile("missing.txt")
		if err == nil {
			t.Fatal("readFile(\"missing.txt\")가 에러를 반환하지 않았습니다.")
		}
		if content != "" {
			t.Fatalf("content = %q, want \"\"", content)
		}
	})
}

// TestDivideErrorVersion는 일반적인 에러에panic을 쓰는 것이 잘못된 패턴임을
// 보여줍니다. 이panic은 테스트의 defer recover가 잡아야 합니다.
func TestDivideErrorVersion(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("divideErrorVersion(10, 0)이panic을 일으키지 않았습니다.")
		} else {
			t.Logf("panic 복구 확인: %v", r)
		}
	}()
	divideErrorVersion(10, 0)
}