package main

import (
	"testing"
)

// TestBasicFor: 기본 for 루프가 1부터 5까지 출력하는지 검증
// - C/Java 스타일 for 루프의 기본 동작 확인
func TestBasicFor(t *testing.T) {
	result := []int{}
	for i := 1; i <= 5; i++ {
		result = append(result, i)
	}

	expected := []int{1, 2, 3, 4, 5}
	if len(result) != len(expected) {
		t.Fatalf("결과 길이 다름: got %d, want %d", len(result), len(expected))
	}

	for i := range expected {
		if result[i] != expected[i] {
			t.Errorf("위치 %d: got %d, want %d", i, result[i], expected[i])
		}
	}
}

// TestWhileStyleFor: 조건만 있는 for 루프가 올바르게 동작하는지 검증
// - Go에는 while이 없으므로 for로 작성
func TestWhileStyleFor(t *testing.T) {
	count := 0
	for count <= 2 {
		count++
	}

	if count != 3 {
		t.Errorf("카운트: got %d, want 3", count)
	}
}

// TestRangeSlice: range 루프가 인덱스와 값을 올바르게 반환하는지 검증
// - 슬라이스 range 루프의 기본 동작 확인
func TestRangeSlice(t *testing.T) {
	names := []string{"홍길동", "김영희", "이순신"}

	type expected struct {
		index int
		name  string
	}

	tests := []struct {
		name     string
		expected []expected
	}{
		{
			name:     "슬라이스 range 루프",
			expected: []expected{{0, "홍길동"}, {1, "김영희"}, {2, "이순신"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := []expected{}
			for i, name := range names {
				results = append(results, expected{i, name})
			}

			if len(results) != len(tt.expected) {
				t.Fatalf("결과 길이 다름: got %d, want %d", len(results), len(tt.expected))
			}

			for i := range tt.expected {
				if results[i] != tt.expected[i] {
					t.Errorf("위치 %d: got %+v, want %+v", i, results[i], tt.expected[i])
				}
			}
		})
	}
}

// TestRangeString: 문자열 range 루프가 UTF-8 코드포인트를 올바르게 순회하는지 검증
// - byte 단위가 아닌 rune 단위로 순회해야 함
func TestRangeString(t *testing.T) {
	text := "안녕👋"

	// range로 순회
	var runes []rune
	for _, r := range text {
		runes = append(runes, r)
	}

	// 기대값: "안", "녕", "👋" (3개)
	if len(runes) != 3 {
		t.Errorf("문자열 '%s'의 rune 수: got %d, want 3", text, len(runes))
	}

	if string(runes[0]) != "안" {
		t.Errorf("첫 번째 rune: got %q, want '안'", runes[0])
	}

	if string(runes[1]) != "녕" {
		t.Errorf("두 번째 rune: got %q, want '녕'", runes[1])
	}

	if string(runes[2]) != "👋" {
		t.Errorf("세 번째 rune: got %q, want '👋'", runes[2])
	}

	// byte로 순회하면 9바이트 (한국어 2글자 = 6바이트, 이모지 = 4바이트)
	byteCount := len([]byte(text))
	if byteCount != 10 {
		t.Errorf("문자열 '%s'의 바이트 수: got %d, want 10", text, byteCount)
	}
}

// TestLabelBreak: 레이블 break가 중첩 루프 전체에서 탈출하는지 검증
func TestLabelBreak(t *testing.T) {
	matrix := [][]int{
		{1, 2, 3},
		{4, 5, 6},
		{7, 8, 9},
	}

	// 레이블 break 사용
	found := false
	foundAt := struct{ i, j int }{}

searchLoop:
	for i, row := range matrix {
		for j, val := range row {
			if val == 5 {
				found = true
				foundAt = struct{ i, j int }{i, j}
				break searchLoop
			}
		}
	}

	if !found {
		t.Error("5를 찾지 못함")
	}

	if foundAt.i != 1 || foundAt.j != 1 {
		t.Errorf("5의 위치: got (%d,%d), want (1,1)", foundAt.i, foundAt.j)
	}
}

// TestLabelContinue: 레이블 continue가 외부 루프를 건너뛰는지 검증
func TestLabelContinue(t *testing.T) {
	matrix := [][]int{
		{1, 2, 3},
		{4, 5, 6},
		{7, 8, 9},
	}

	// 행의 첫 번째 원소가 홀수인 경우 건너뛰기
	var results [][]int

outerLoop:
	for _, row := range matrix {
		if len(row) > 0 && row[0]%2 != 0 {
			continue outerLoop
		}
		results = append(results, row)
	}

	// 첫 번째 원소가 짝수인 행만 있어야 함: {4,5,6}
	if len(results) != 1 {
		t.Errorf("결과 행 수: got %d, want 1", len(results))
	}

	if len(results) > 0 && len(results[0]) != 3 {
		t.Errorf("결과 행 길이: got %d, want 3", len(results[0]))
	}
}

// TestRangeNoVar: 변수 선언 없이 range 사용이 올바른지 검증
func TestRangeNoVar(t *testing.T) {
	values := []int{10, 20, 30}

	// 값만 필요
	var sum int
	for _, v := range values {
		sum += v
	}

	if sum != 60 {
		t.Errorf("합계: got %d, want 60", sum)
	}

	// 인덱스만 필요
	var indices []int
	for i := range values {
		indices = append(indices, i)
	}

	expected := []int{0, 1, 2}
	if len(indices) != len(expected) {
		t.Errorf("인덱스 수: got %d, want %d", len(indices), len(expected))
	}

	for i := range expected {
		if indices[i] != expected[i] {
			t.Errorf("인덱스 %d: got %d, want %d", i, indices[i], expected[i])
		}
	}
}

// TestInfiniteLoop: 무한 루프가 break로 올바르게 탈출하는지 검증
func TestInfiniteLoop(t *testing.T) {
	count := 0
	for {
		if count >= 3 {
			break
		}
		count++
	}

	if count != 3 {
		t.Errorf("카운트: got %d, want 3", count)
	}
}