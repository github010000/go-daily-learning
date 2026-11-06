package main

import (
	"testing"
)

// 배열은 길이까지 타입의 일부이다. 길이가 다르면 컴파일 오류이므로,
// 이 테스트는 같은 길이·값의 배열이 같음을 검증합니다.
func TestArrayLengthIsPartOfType(t *testing.T) {
	var a [3]int = [3]int{1, 2, 3}
	var b [3]int = [3]int{1, 2, 3}
	if a != b {
		t.Errorf("같은 길이·값의 배열은 같아야 한다: %v vs %v", a, b)
	}
}

// 배열은 값 전달이라 함수 호출 후 원본이 바뀌지 않는다.
func TestArrayIsPassedByValue(t *testing.T) {
	before := [3]int{10, 20, 30}
	after := mutateArray(before)

	if before[1] != 20 {
		t.Errorf("배열은 값 전달이므로 원본은 바뀌면 안 된다: before[1]=%d", before[1])
	}
	if after[1] != 99 {
		t.Errorf("배열 복사본은 바뀌어도 된다: after[1]=%d", after[1])
	}
}

// 슬라이스는 레퍼런스라 원본이 바뀐다.
func TestSliceMutatesOriginal(t *testing.T) {
	before := []int{10, 20, 30}
	_ = mutateSlice(before)

	if before[1] != 99 {
		t.Errorf("슬라이스는 원본 배열을 공유하므로 바뀌어야 한다: before[1]=%d", before[1])
	}
}