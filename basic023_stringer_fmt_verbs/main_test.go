package main

import (
	"fmt"
	"strings"
	"testing"
)

// TestVerbDifference는 %v·%+v·%#v·%T·%q 포맷터가 기대하는 형태로 출력되는지 검증합니다.
// Stringer가 구현된 타입은 String() 결과를, 아닌 타입은 구조체 값을 출력해야 합니다.
func TestVerbDifference(t *testing.T) {
	p := Person{Name: "테스트", Age: 10}
	pt := Point{X: 5, Y: 15}
	plain := Plain{Name: "기본"}

	tests := []struct {
		name     string
		format   string
		value    interface{}
		expected string
	}{
		{
			name:     "Person %v는 Stringer 결과",
			format:   "%v",
			value:    p,
			expected: "테스트 (10세)",
		},
		{
			name:     "Point %v는 Stringer 결과",
			format:   "%v",
			value:    pt,
			expected: "(5,15)",
		},
		{
			name:     "Person %+v는 Stringer 결과",
			format:   "%+v",
			value:    p,
			expected: "테스트 (10세)",
		},
		{
			name:     "Point %+v는 Stringer 결과",
			format:   "%+v",
			value:    pt,
			expected: "(5,15)",
		},
		{
			name:     "Person %#v는 구조체 구문 (Stringer 무시)",
			format:   "%#v",
			value:    p,
			expected: "main.Person{Name:\"테스트\", Age:10}",
		},
		{
			name:     "Point %#v는 구조체 구문 (Stringer 무시)",
			format:   "%#v",
			value:    pt,
			expected: "main.Point{X:5, Y:15}",
		},
		{
			name:     "Person %T는 타입 이름",
			format:   "%T",
			value:    p,
			expected: "main.Person",
		},
		{
			name:     "Point %T는 타입 이름",
			format:   "%T",
			value:    pt,
			expected: "main.Point",
		},
		{
			name:     "문자열 %q는 쌍따옴표 포함",
			format:   "%q",
			value:    "안녕",
			expected: `"안녕"`,
		},
		{
			name:     "숫자 %q는 백틱 포함",
			format:   "%q",
			value:    999,
			expected: "`999`",
		},
		{
			name:     "rune %q는 유니코드 문자",
			format:   "%q",
			value:    '가',
			expected: "'가'",
		},
		{
			name:     "Plain %v는 Stringer 없음 (구조체 값 출력)",
			format:   "%v",
			value:    plain,
			expected: "{기본}",
		},
		{
			name:     "Plain %+v는 필드 이름 포함",
			format:   "%+v",
			value:    plain,
			expected: "{Name:기본}",
		},
		{
			name:     "Plain %#v는 구조체 구문",
			format:   "%#v",
			value:    plain,
			expected: "main.Plain{Name:\"기본\"}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := fmt.Sprintf(tt.format, tt.value)
			if result != tt.expected {
				t.Errorf("fmt.Sprintf(%q, %v) = %q, want %q", tt.format, tt.value, result, tt.expected)
			}
		})
	}
}

// TestStringerInterface는 Stringer 인터페이스로 저장된 값들이
// 올바르게 String()을 호출하는지 검증합니다.
func TestStringerInterface(t *testing.T) {
	var items []fmt.Stringer
	items = append(items, Person{Name: "이순신", Age: 40})
	items = append(items, Point{X: 1, Y: 2})
	items = append(items, SafeStringer{Value: 99})

	expected := []string{
		"이순신 (40세)",
		"(1,2)",
		"값: 99",
	}

	for i, item := range items {
		result := item.String()
		if result != expected[i] {
			t.Errorf("items[%d].String() = %q, want %q", i, result, expected[i])
		}
	}
}

// TestSafeStringerNoRecursion은 SafeStringer가 재귀 없이 정상 작동하는지 검증합니다.
// SafeStringer는 내부적으로 %d를 써서 재귀를 피합니다.
func TestSafeStringerNoRecursion(t *testing.T) {
	s := SafeStringer{Value: 77}
	result := s.String()

	expected := "값: 77"
	if result != expected {
		t.Errorf("SafeStringer.String() = %q, want %q", result, expected)
	}
}

// TestLogFormatOutput은 로깅 출력 형식이 올바른지 검증합니다.
// %v는 Stringer 결과를, %+v는 Stringer가 있으면 %v와 동일합니다.
func TestLogFormatOutput(t *testing.T) {
	p := Person{Name: "홍길동", Age: 28}
	plain := Plain{Name: "기본"}

	// %v는 Stringer 결과를 사용합니다.
	s := fmt.Sprintf("%v", p)
	if s != "홍길동 (28세)" {
		t.Errorf("fmt.Sprintf(\"%%v\", p) = %q, want %q", s, "홍길동 (28세)")
	}

	// %+v도 Stringer가 있으면 String() 결과를 사용합니다.
	s = fmt.Sprintf("%+v", p)
	if s != "홍길동 (28세)" {
		t.Errorf("fmt.Sprintf(\"%%+v\", p) = %q, want %q", s, "홍길동 (28세)")
	}

	// Stringer가 없는 Plain은 %+v가 필드 이름을 포함합니다.
	s = fmt.Sprintf("%+v", plain)
	if s != "{Name:기본}" {
		t.Errorf("fmt.Sprintf(\"%%+v\", plain) = %q, want %q", s, "{Name:기본}")
	}
}

// TestNonStringerVerbDiff는 Stringer를 구현하지 않은 타입에서
// %v와 %+v가 어떻게 다른지 검증합니다.
func TestNonStringerVerbDiff(t *testing.T) {
	p := Plain{Name: "기본"}

	// %v는 구조체 값을 보여줍니다.
	v := fmt.Sprintf("%v", p)
	if !strings.Contains(v, "기본") {
		t.Errorf("fmt.Sprintf(\"%%v\", p) = %q, '기본'을 포함해야 합니다", v)
	}

	// %+v는 필드 이름을 포함합니다.
	vv := fmt.Sprintf("%+v", p)
	if !strings.Contains(vv, "Name") {
		t.Errorf("fmt.Sprintf(\"%%+v\", p) = %q, 'Name'을 포함해야 합니다", vv)
	}
}