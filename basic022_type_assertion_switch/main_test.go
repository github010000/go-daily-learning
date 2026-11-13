package main

import (
	"strings"
	"testing"
)

// TestTypeAssertSuccess 는 올바른 타입에 대한 타입 단언이 성공하는지 검증합니다.
// comma-ok 패턴에서 ok=true 가 되고, 꺼낸 값의 내용도 맞아야 합니다.
func TestTypeAssertSuccess(t *testing.T) {
	t.Run("StringVal 단언 성공", func(t *testing.T) {
		var val interface{} = StringVal{V: "Hello"}
		sv, ok := val.(StringVal)
		if !ok {
			t.Fatal("StringVal 단언이 실패했습니다 (ok=false)")
		}
		if sv.V != "Hello" {
			t.Fatalf("내용 불일치: 기대 'Hello', 실제 %q", sv.V)
		}
	})

	t.Run("IntVal 단언 성공", func(t *testing.T) {
		var val interface{} = IntVal{V: 42}
		iv, ok := val.(IntVal)
		if !ok {
			t.Fatal("IntVal 단언이 실패했습니다 (ok=false)")
		}
		if iv.V != 42 {
			t.Fatalf("내용 불일치: 기대 42, 실제 %d", iv.V)
		}
	})
}

// TestTypeAssertFailure 는 잘못된 타입에 대한 타입 단언이 panic 없이 false 를 반환하는지 검증합니다.
// panic 을 일으키지 않고 안전하게 실패를 확인할 수 있어야 합니다.
func TestTypeAssertFailure(t *testing.T) {
	t.Run("StringVal 을 IntVal 로 단언 실패", func(t *testing.T) {
		var val interface{} = StringVal{V: "Hello"}
		_, ok := val.(IntVal)
		if ok {
			t.Fatal("StringVal 을 IntVal 로 단언하면 ok=false 여야 합니다.")
		}
	})

	t.Run("nil interface 단언 실패", func(t *testing.T) {
		var val interface{} = nil
		_, ok := val.(IntVal)
		if ok {
			t.Fatal("nil interface 를 단언하면 ok=false 여야 합니다.")
		}
	})
}

// TestTypeSwitchRoutes 는 타입 스위치가 올바른 케이스로 분기하는지 검증합니다.
// 테이블 주도 방식으로 각 타입별로 expected 분류값과 비교합니다.
func TestTypeSwitchRoutes(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected string
	}{
		{"StringVal 타입", StringVal{V: "Go"}, "StringVal"},
		{"IntVal 타입", IntVal{V: 123}, "IntVal"},
		{"float64 타입", 42.5, "float64"},
		{"string 타입", "plain string", "string"},
		{"알 수 없는 타입 (float32)", float32(1.1), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result string
			switch v := tt.input.(type) {
			case StringVal:
				result = "StringVal"
				_ = v.V
			case IntVal:
				result = "IntVal"
				_ = v.V
			case float64:
				result = "float64"
				_ = v
			case string:
				result = "string"
				_ = v
			default:
				result = "unknown"
				_ = v
			}

			if result != tt.expected {
				t.Fatalf("기대값 %s, 실제값 %s", tt.expected, result)
			}
		})
	}
}

// TestPanicOnWrongTypeAssert 는 잘못된 타입 단언 시 panic 이 발생하는지 검증합니다.
// defer/recover 로 panic 을 포착하여 실제로 터지는지 확인합니다.
func TestPanicOnWrongTypeAssert(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("잘못된 타입 단언 시 panic 이 발생하지 않았습니다.")
		} else {
			t.Logf("panic 발생 확인: %v", r)
		}
	}()

	// StringVal 을 IntVal 로 단언하면 panic 이 발생합니다.
	panicOnWrongTypeAssert(StringVal{V: "Not an Int"})
}

// TestSliceCopyIsolation 는 copySliceSafe 가 원본과 독립적인 슬라이스를 반환하는지 검증합니다.
// copySliceUnsafe 와 달리, 반환값을 수정해도 원본이 변하지 않아야 합니다.
func TestSliceCopyIsolation(t *testing.T) {
	original := []int{1, 2, 3}
	safe := copySliceSafe(original)

	// safe 의 첫 요소를 바꿉니다.
	safe[0] = 999

	// 원본은 여전히 처음 값이어야 합니다.
	if original[0] != 1 {
		t.Fatalf("copySliceSafe 가 원본을 보호하지 않았습니다: original[0]=%d, 기대 1", original[0])
	}
	// safe 는 바뀐 값이어야 합니다.
	if safe[0] != 999 {
		t.Fatalf("copySliceSafe 가 독립적인 사본을 만들지 않았습니다: safe[0]=%d, 기대 999", safe[0])
	}
}

// captureOutput 은 함수 실행 시 표준 출력을 가로채서 문자열로 반환합니다.
func captureOutput(f func()) (string, error) {
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	f()

	w.Close()
	os.Stdout = oldStdout

	out, _ := io.ReadAll(r)
	return string(out), nil
}

// TestTypeSwitchOutput 은 typeSwitchDemo 함수의 출력이 모든 예상 키워드를 포함하는지 검증합니다.
// 타입 스위치가 float64, string, StringVal, IntVal, default 모두 처리하는지 확인합니다.
func TestTypeSwitchOutput(t *testing.T) {
	expectedSubstrings := []string{
		"StringVal 타입",
		"IntVal 타입",
		"float64 타입",
		"string 타입",
		"알 수 없는 타입",
	}

	out, err := captureOutput(typeSwitchDemo)
	if err != nil {
		t.Fatalf("출력 캡처 중 오류 발생: %v", err)
	}

	for _, sub := range expectedSubstrings {
		if !strings.Contains(out, sub) {
			t.Fatalf("출력에 '%s' 가 포함되지 않았습니다.\n실제 출력:\n%s", sub, out)
		}
	}
}

// TestSliceCopyDemoOutput 는 sliceCopyDemo 의 출력이 "원본"과 "safe" 의 차이를 명확히 보여주는지 검증합니다.
// unsafe 버전은 원본이 바뀌지만, safe 버전은 원본이 보존됩니다.
func TestSliceCopyDemoOutput(t *testing.T) {
	expectedSubstrings := []string{
		"원본: [1 2 3]",
		"unsafe 수정 후 -> 원본: [999 2 3]",
		"safe 수정 후 -> 원본: [1 2 3]",
	}

	out, err := captureOutput(sliceCopyDemo)
	if err != nil {
		t.Fatalf("출력 캡처 중 오류 발생: %v", err)
	}

	for _, sub := range expectedSubstrings {
		if !strings.Contains(out, sub) {
			t.Fatalf("출력에 '%s' 가 포함되지 않았습니다.\n실제 출력:\n%s", sub, out)
		}
	}
}