package main

import (
	"strings"
	"testing"
)

// TestVariadicSum은 가변 인자 함수 sumInts가 올바른 합을 반환하는지 검증합니다.
// - 기본 호출: 고정 인자 여러 개 전달
// - 슬라이스 펼치기: 슬라이스를 ... 연산자로 펼칠 때
// - 빈 인자: 인자가 없을 때
func TestVariadicSum(t *testing.T) {
	t.Run("기본 호출", func(t *testing.T) {
		result := sumInts(1, 2, 3, 4, 5)
		if result != 15 {
			t.Errorf("sumInts(1, 2, 3, 4, 5) = %d, want 15", result)
		}
	})
	t.Run("슬라이스 펼치기", func(t *testing.T) {
		numbers := []int{10, 20, 30}
		result := sumInts(numbers...)
		if result != 60 {
			t.Errorf("sumInts(numbers...) = %d, want 60", result)
		}
	})
	t.Run("빈 인자", func(t *testing.T) {
		result := sumInts()
		if result != 0 {
			t.Errorf("sumInts() = %d, want 0", result)
		}
	})
}

// TestMaxInts는 가변 인자 함수 maxInts가 올바른 최댓값을 반환하는지 검증합니다.
func TestMaxInts(t *testing.T) {
	t.Run("기본 호출", func(t *testing.T) {
		result := maxInts(3, 1, 4, 1, 5, 9, 2, 6)
		if result != 9 {
			t.Errorf("maxInts(3, 1, 4, 1, 5, 9, 2, 6) = %d, want 9", result)
		}
	})
	t.Run("슬라이스 펼치기", func(t *testing.T) {
		numbers := []int{10, 20, 30, 40, 50}
		result := maxInts(numbers...)
		if result != 50 {
			t.Errorf("maxInts(numbers...) = %d, want 50", result)
		}
	})
	t.Run("빈 인자", func(t *testing.T) {
		result := maxInts()
		if result != 0 {
			t.Errorf("maxInts() = %d, want 0", result)
		}
	})
}

// TestFunctionValueStrategy는 함수 값을 통한 전략 패턴이 올바르게 동작하는지 검증합니다.
// - joinStrategy: 기본 연결
// - upperJoinStrategy: 대문자 변환 후 연결
// - 익명 함수: 동적으로 정의된 전략
func TestFunctionValueStrategy(t *testing.T) {
	items := []string{"apple", "banana", "cherry"}

	t.Run("joinStrategy", func(t *testing.T) {
		result := processItems(items, joinStrategy, " | ")
		expected := "apple | banana | cherry"
		if result != expected {
			t.Errorf("joinStrategy result = %q, want %q", result, expected)
		}
	})
	t.Run("upperJoinStrategy", func(t *testing.T) {
		result := processItems(items, upperJoinStrategy, " - ")
		expected := "APPLE - BANANA - CHERRY"
		if result != expected {
			t.Errorf("upperJoinStrategy result = %q, want %q", result, expected)
		}
	})
	t.Run("익명 함수", func(t *testing.T) {
		customStrategy := func(items []string, sep string) string {
			return strings.Join(items, sep) + " (익명)"
		}
		result := processItems(items, customStrategy, " + ")
		expected := "apple + banana + cherry (익명)"
		if result != expected {
			t.Errorf("anonymous function result = %q, want %q", result, expected)
		}
	})
}