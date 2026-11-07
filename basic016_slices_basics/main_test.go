package main

import (
	"reflect"
	"testing"
)

// TestSliceByVal_NotModifyOriginal: sliceByVal 함수가 반환값은 올바르게 주되,
// 원본 슬라이스는 append로 인한 재할당 시에도 변하지 않아야 함을 검증합니다.
func TestSliceByVal_NotModifyOriginal(t *testing.T) {
	original := []int{1, 2, 3}
	result := sliceByVal(original, 4)

	// 원본 슬라이스는 그대로여야 합니다.
	if !reflect.DeepEqual(original, []int{1, 2, 3}) {
		t.Errorf("원본 슬라이스가 변경되었습니다: got %v, want %v", original, []int{1, 2, 3})
	}

	// 반환값은 원본에 새 요소를 추가한 결과여야 합니다.
	expected := []int{1, 2, 3, 4}
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("반환값이 올바르지 않습니다: got %v, want %v", result, expected)
	}
}

// TestModifyInPlace: modifyInPlace 함수가 특정 인덱스의 값을 올바르게 변경하는지 검증합니다.
func TestModifyInPlace(t *testing.T) {
	nums := []int{10, 20, 30}
	modifyInPlace(nums, 1, 99)

	expected := []int{10, 99, 30}
	if !reflect.DeepEqual(nums, expected) {
		t.Errorf("값 변경이 올바르지 않습니다: got %v, want %v", nums, expected)
	}
}

// TestNilVsEmptySlice_Append: nil 슬라이스와 빈 슬라이스에 append를 했을 때
// 반환되는 슬라이스의 값이 동일한지 검증합니다.
func TestNilVsEmptySlice_Append(t *testing.T) {
	var nilSl []int
	emptySl := []int{}

	resultNil := append(nilSl, 1, 2)
	resultEmpty := append(emptySl, 1, 2)

	expected := []int{1, 2}
	if !reflect.DeepEqual(resultNil, expected) {
		t.Errorf("nil 슬라이스 append 결과 불일치: got %v, want %v", resultNil, expected)
	}
	if !reflect.DeepEqual(resultEmpty, expected) {
		t.Errorf("빈 슬라이스 append 결과 불일치: got %v, want %v", resultEmpty, expected)
	}
}