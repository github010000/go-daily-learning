package main

import (
	"testing"
)

// TestScoreToGrade — 기본 switch가 올바른 등급을 반환하는지 검증합니다.
// Go switch는 case 실행 후 자동 종료되므로, return만 있으면 됩니다.
func TestScoreToGrade(t *testing.T) {
	tests := []struct {
		name     string
		score    int
		expected string
	}{
		{"A score", 95, "A"},
		{"A boundary", 90, "A"},
		{"B score", 82, "B"},
		{"B boundary", 80, "B"},
		{"C score", 70, "C"},
		{"D score", 55, "D"},
		{"F score", 40, "F"},
		{"zero", 0, "F"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := scoreToGrade(tt.score)
			if result != tt.expected {
				t.Errorf("scoreToGrade(%d) = %q, want %q", tt.score, result, tt.expected)
			}
		})
	}
}

// TestClassifyNumber — 조건 없는 switch가 if-else와 같은 결과를 반환하는지 검증합니다.
// switch 뒤 조건을 생략한 버전과 if-else 버전을 비교합니다.
func TestClassifyNumber(t *testing.T) {
	numbers := []int{-5, 0, 5, 50, 150}

	for _, n := range numbers {
		t.Run("classifyNumber", func(t *testing.T) {
			switchResult := classifyNumber(n)
			ifElseResult := classifyNumberIfElse(n)
			if switchResult != ifElseResult {
				t.Errorf("classifyNumber(%d) = %q, but if-else gives %q", n, switchResult, ifElseResult)
			}
		})
	}
}

// TestMonthDays — 여러 달의 일수가 올바른지 검증합니다.
// case에 여러 값을 나열한 버전입니다.
func TestMonthDays(t *testing.T) {
	tests := []struct {
		month    int
		expected int
	}{
		{1, 31},  // 1월
		{2, 28},  // 2월
		{3, 31},  // 3월
		{4, 30},  // 4월
		{6, 30},  // 6월
		{9, 30},  // 9월
		{12, 31}, // 12월
		{0, 0},   // 잘못된 월
		{13, 0},  // 잘못된 월
	}

	for _, tt := range tests {
		t.Run("month", func(t *testing.T) {
			result := monthDays(tt.month)
			if result != tt.expected {
				t.Errorf("monthDays(%d) = %d, want %d", tt.month, result, tt.expected)
			}
		})
	}
}

// TestDescribeWeekday — 요일 분류가 올바른지 검증합니다.
// case에 여러 값을 나열한 버전입니다.
func TestDescribeWeekday(t *testing.T) {
	tests := []struct {
		day      int
		expected string
	}{
		{1, "평일"},
		{3, "평일"},
		{5, "평일"},
		{6, "주말"},
		{7, "주말"},
		{8, "잘못된 요일"},
		{0, "잘못된 요일"},
	}

	for _, tt := range tests {
		t.Run("weekday", func(t *testing.T) {
			result := describeWeekday(tt.day)
			if result != tt.expected {
				t.Errorf("describeWeekday(%d) = %q, want %q", tt.day, result, tt.expected)
			}
		})
	}
}

// TestFallthroughBehavior — fallthrough가 강제로 다음 case를 실행하는지 검증합니다.
// fallthrough가 있으면 상위 case의 조건이 false여도 다음 case를 실행합니다.
func TestFallthroughBehavior(t *testing.T) {
	// score=95: A case 실행 후 fallthrough로 B case 실행 → "B" 반환
	result := scoreToGradeWithFallthrough(95)
	if result != "B" {
		t.Errorf("scoreToGradeWithFallthrough(95) = %q, want B (fallthrough로 B case 실행)", result)
	}

	// score=82: B case 실행 후 fallthrough로 C case 실행 → "C" 반환
	result = scoreToGradeWithFallthrough(82)
	if result != "C" {
		t.Errorf("scoreToGradeWithFallthrough(82) = %q, want C (fallthrough로 C case 실행)", result)
	}

	// score=70: C case 실행 (fallthrough 없음) → "C" 반환
	result = scoreToGradeWithFallthrough(70)
	if result != "C" {
		t.Errorf("scoreToGradeWithFallthrough(70) = %q, want C (fallthrough 없으므로 C 유지)", result)
	}

	// 기본 switch와 비교: fallthrough 없는 버전은 올바르게 동작함
	baseResult := scoreToGrade(95)
	if baseResult != "A" {
		t.Errorf("scoreToGrade(95) = %q, want A (fallthrough 없음)", baseResult)
	}
}