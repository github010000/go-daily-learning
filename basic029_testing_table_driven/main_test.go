package main

import "testing"

// TestParsePositiveInt_TableDriven은 테이블 주도 테스트의 기본 패턴을 검증합니다.
// 케이스(데이터)와 검증 로직을 분리하여, 새로운 케이스 추가가 쉬워지는 것을 보여줍니다.
// t.Run()을 써서 각 케이스를 독립적인 서브테스트로 실행합니다.
func TestParsePositiveInt_TableDriven(t *testing.T) {
	// 테이블 주도 테스트: 케이스 데이터
	table := []struct {
		name     string
		input    string
		expected int
		wantErr  bool
	}{
		{"양수", "123", 123, false},
		{"영", "0", 0, false},
		{"음수", "-42", 0, true},
		{"문자", "abc", 0, true},
		{"혼합", "12a", 0, true},
		{"빈 문자열", "", 0, true},
	}

	// range로 반복하면서 각 케이스를 t.Run()으로 실행
	for _, tc := range table {
		t.Run(tc.name, func(t *testing.T) {
			result, err := parsePositiveIntFixed(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Errorf("expected error for %q, but got nil", tc.input)
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error for %q: %v", tc.input, err)
				return
			}
			if result != tc.expected {
				t.Errorf("expected %d, got %d", tc.expected, result)
			}
		})
	}
}

// TestParsePositiveInt_CommonMistake는 고친 버전의 기본 동작이 올바르게 동작하는지를 검증합니다.
// 실수 버전의 출력은 main.go 실행 결과로 확인하세요.
func TestParsePositiveInt_CommonMistake(t *testing.T) {
	t.Run("fixed_version", func(t *testing.T) {
		n, err := parsePositiveIntFixed("123")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if n != 123 {
			t.Errorf("expected 123, got %d", n)
		}
	})
}

// TestAssertEqual은 t.Helper()의 효과를 보여주는 테스트입니다.
// assertEqual 헬퍼를 사용해 기대값과 실제값이 일치하는지 검증합니다.
func TestAssertEqual(t *testing.T) {
	t.Run("matching_values", func(t *testing.T) {
		assertEqual(t, 5, 5)
		assertEqual(t, 0, 0)
		assertEqual(t, -10, -10)
	})
}