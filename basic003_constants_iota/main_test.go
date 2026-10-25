package main

import (
	"testing"
)

// TestDayOfWeekIotaValues 는 요일 열거형의 iota 값이 0~6임을 검증합니다.
// 0=Sunday, 1=Monday, ..., 6=Saturday 순서입니다.
func TestDayOfWeekIotaValues(t *testing.T) {
	t.Run("DayOfWeek iota values", func(t *testing.T) {
		table := []struct {
			name     string
			day      DayOfWeek
			expected int
		}{
			{"Sunday", Sunday, 0},
			{"Monday", Monday, 1},
			{"Tuesday", Tuesday, 2},
			{"Wednesday", Wednesday, 3},
			{"Thursday", Thursday, 4},
			{"Friday", Friday, 5},
			{"Saturday", Saturday, 6},
		}

		for _, tc := range table {
			t.Run(tc.name, func(t *testing.T) {
				if int(tc.day) != tc.expected {
					t.Errorf("%s = %d, want %d", tc.name, tc.day, tc.expected)
				}
			})
		}
	})
}

// TestDayOfWeekZeroValueIsSafe 는 var DayOfWeek{} 가 0(알 수 없음)이 되는 것을 검증합니다.
// 0 을 '미정의'로 비워두면 new() 나 var 로 선언해도 안전한 시작점을 갖습니다.
func TestDayOfWeekZeroValueIsSafe(t *testing.T) {
	var d DayOfWeek
	if d != 0 {
		t.Errorf("zero value DayOfWeek = %d, want 0 (알 수 없음)", d)
	}
}

// TestDayOfWeekStringMethod 는 요일의 String() 메서드가 올바른 한국어 이름을 반환하는지 검증합니다.
func TestDayOfWeekStringMethod(t *testing.T) {
	if Sunday.String() != "일" {
		t.Errorf("Sunday.String() = %q, want '일'", Sunday.String())
	}
	if Monday.String() != "월" {
		t.Errorf("Monday.String() = %q, want '월'", Monday.String())
	}
	if Saturday.String() != "토" {
		t.Errorf("Saturday.String() = %q, want '토'", Saturday.String())
	}
	if DayOfWeek(99).String() != "알 수 없음" {
		t.Errorf("DayOfWeek(99).String() = %q, want '알 수 없음'", DayOfWeek(99).String())
	}
}

// TestUntypedConstantTypeResolution 은 숫자 리터럴이 타입 없는 상수임을 검증합니다.
// 같은 리터럴 '1'을 byte/int/float64 변수에 할당해도 각각 올바른 타입으로 저장됩니다.
func TestUntypedConstantTypeResolution(t *testing.T) {
	t.Run("same literal, different variable types", func(t *testing.T) {
		var smallByte byte = 1
		var bigInt int = 1
		var bigFloat float64 = 1.0

		if smallByte != 1 {
			t.Errorf("smallByte = %d, want 1", smallByte)
		}
		if bigInt != 1 {
			t.Errorf("bigInt = %d, want 1", bigInt)
		}
		if bigFloat != 1.0 {
			t.Errorf("bigFloat = %f, want 1.0", bigFloat)
		}
	})

	t.Run("untyped constant fits multiple types", func(t *testing.T) {
		const limit = 100

		var b byte = byte(limit)
		var i int = limit
		var u uint = uint(limit)

		if b != 100 {
			t.Errorf("byte limit = %d, want 100", b)
		}
		if i != 100 {
			t.Errorf("int limit = %d, want 100", i)
		}
		if u != 100 {
			t.Errorf("uint limit = %d, want 100", u)
		}
	})
}

// TestIotaResetsPerBlock 은 iota 가 const 블록이 열릴 때마다 0 으로 리셋된다는 것을 검증합니다.
// iota 는 전역 카운터가 아님을 확인합니다.
func TestIotaResetsPerBlock(t *testing.T) {
	const (
		a = iota // 0
		b        // 1
	)
	const (
		c = iota // 0 — 리셋!
		d        // 1
	)

	if a != 0 {
		t.Errorf("a = %d, want 0", a)
	}
	if b != 1 {
		t.Errorf("b = %d, want 1", b)
	}
	if c != 0 {
		t.Errorf("c = %d, want 0 (reset)", c)
	}
	if d != 1 {
		t.Errorf("d = %d, want 1", d)
	}
}

// TestBitFlagsCombine 는 비트 플래그가 OR 조합으로 동작하고,
// AND 로 개별 비트를 확인할 수 있음을 검증합니다.
func TestBitFlagsCombine(t *testing.T) {
	type Color int
	const (
		Red   Color = 1 << iota // 1
		Green                   // 2
		Blue                    // 4
	)

	combined := Red | Green
	if combined != 3 { // 0b001 | 0b010 = 0b011 = 3
		t.Errorf("Red|Green = %d, want 3", combined)
	}
	if combined&Red == 0 {
		t.Error("combined should have Red bit set")
	}
	if combined&Green == 0 {
		t.Error("combined should have Green bit set")
	}
	if combined&Blue != 0 {
		t.Error("combined should NOT have Blue bit set")
	}
}

// TestEnumStringConversion 은 열거형 값을 문자열로 변환하는 방식이 올바르게 동작하는지 검증합니다.
func TestEnumStringConversion(t *testing.T) {
	if Sunday.String() != "일" {
		t.Errorf("Sunday.String() = %q, want '일'", Sunday.String())
	}
	if Monday.String() != "월" {
		t.Errorf("Monday.String() = %q, want '월'", Monday.String())
	}
}