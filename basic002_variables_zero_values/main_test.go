package main

import "testing"

// TestVarAndAssign: var 선언과 := 단축 할당이 올바른 타입과 값을 가지는지 검증한다
func TestVarAndAssign(t *testing.T) {
	// var 로 선언한 변수는 명시적 타입을 유지하고, := 은 우변 타입을 추론한다.
	// 제로값도 올바르게 들어간다.
	t.Run("var vs short declaration", func(t *testing.T) {
		var i int = 10
		j := 20

		if i != 10 {
			t.Errorf("var int = 10 기대, got %d", i)
		}
		if j != 20 {
			t.Errorf("short decl int = 20 기대, got %d", j)
		}
	})
}

// TestZeroValues: 초기화하지 않은 변수의 제로값이 맞는지 검증한다
func TestZeroValues(t *testing.T) {
	// Go 는 undefined 변수를 허용하지 않고, 모두 제로값으로 초기화한다.
	// 이는 다른 언어(Java의 null pointer, Python의 NameError 등)와 다른.Go의 설계 결정이다.
	t.Run("zero values", func(t *testing.T) {
		var i int
		var b bool
		var s string
		var f float64

		if i != 0 {
			t.Errorf("int zero value 기대 0, got %d", i)
		}
		if b != false {
			t.Errorf("bool zero value 기대 false, got %v", b)
		}
		if s != "" {
			t.Errorf("string zero value 기대 \"\", got %q", s)
		}
		if f != 0.0 {
			t.Errorf("float64 zero value 기대 0.0, got %f", f)
		}
	})
}

// TestStringImmutable: string 이 불변이고, []byte 로 변환해야 수정 가능한지 검증한다
func TestStringImmutable(t *testing.T) {
	// string 은 읽기 전용 바이트 시퀀스다. 인덱스로 대입할 수 없다.
	// []byte 로 변환하면 writable 이 된다.
	t.Run("string is immutable", func(t *testing.T) {
		s := "hello"

		// 읽기는 가능하다
		if s[0] != 'h' {
			t.Errorf("s[0] 기대 'h', got %c", s[0])
		}

		// 대입은 불가능하다(컴파일 단계에서 오류).
		// 대신 []byte 로 변환하면 가능하다.
		b := []byte(s)
		b[0] = 'H'
		newS := string(b)

		if newS != "Hello" {
			t.Errorf("변환 후 string 기대 \"Hello\", got %q", newS)
		}
		// 원본은 여전히 "hello"
		if s != "hello" {
			t.Errorf("원본 string 은 변하지 않아야 함, got %q", s)
		}
	})
}