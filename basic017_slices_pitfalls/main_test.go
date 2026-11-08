package main

import (
	"testing"
)

// TestAppendDoesNotMutateOriginal은 append가 새로운 슬라이스를 반환하며
// 원본 슬라이스의 len/cap은 변하지 않음을 검증합니다.
// Go의 append는 원본 슬라이스 구조체(포인터, len, cap)를 수정하지 않습니다.
func TestAppendDoesNotMutateOriginal(t *testing.T) {
	original := []int{1, 2}
	origLen := len(original)
	origCap := cap(original)

	_ = append(original, 3)

	if len(original) != origLen {
		t.Errorf("append 후 원본 len이 변했습니다: want %d, got %d", origLen, len(original))
	}
	if cap(original) != origCap {
		t.Errorf("append 후 원본 cap이 변했습니다: want %d, got %d", origCap, cap(original))
	}
}

// TestMutateSliceSafePreservesOriginal은 mutateSliceSafe가 callers의
// 슬라이스를 변조하지 않음을 검증합니다.
// 함수가 인자를 깊은 복사해야 side effect가 없음을 테스트합니다.
func TestMutateSliceSafePreservesOriginal(t *testing.T) {
	data := []int{100, 200, 300, 400}
	originalFirst := data[0]

	result := mutateSliceSafe(data)

	if data[0] != originalFirst {
		t.Errorf("mutateSliceSafe가 원본 슬라이스를 변조했습니다: want %d, got %d", originalFirst, data[0])
	}
	if result[0] != 0 {
		t.Errorf("함수 반환값의 첫 원소가 0이 아닙니다: want 0, got %d", result[0])
	}
}

// TestCopyReturnsCorrectLength는 copy가 min(len(dst), len(src))를
// 반환하는지를 검증합니다.
func TestCopyReturnsCorrectLength(t *testing.T) {
	tests := []struct {
		name     string
		src      []int
		dstLen   int
		wantN    int
		wantDst  []int
	}{
		{
			name:    "src가 dst보다 긴 경우",
			src:     []int{1, 2, 3, 4, 5},
			dstLen:  3,
			wantN:   3,
			wantDst: []int{1, 2, 3},
		},
		{
			name:    "dst가 src보다 긴 경우",
			src:     []int{1, 2},
			dstLen:  5,
			wantN:   2,
			wantDst: []int{1, 2, 0, 0, 0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dst := make([]int, tt.dstLen)
			n := copy(dst, tt.src)

			if n != tt.wantN {
				t.Errorf("copy 반환값: want %d, got %d", tt.wantN, n)
			}
			if len(dst) != len(tt.wantDst) {
				t.Errorf("dst 길이: want %d, got %d", len(tt.wantDst), len(dst))
			}
			for i := range tt.wantDst {
				if dst[i] != tt.wantDst[i] {
					t.Errorf("dst[%d]: want %d, got %d", i, tt.wantDst[i], dst[i])
				}
			}
		})
	}
}

// TestDeepCopyCreatesIndependentSlice은 깊은 복사가 원본과 독립적인
// 슬라이스를 생성하는지를 검증합니다.
func TestDeepCopyCreatesIndependentSlice(t *testing.T) {
	original := []int{10, 20, 30}
	copied := make([]int, len(original))
	copy(copied, original)

	// 원본을 수정해도 사본은 바뀌지 않아야 합니다.
	original[0] = 999

	if copied[0] != 10 {
		t.Errorf("깊은 복사가 독립적이지 않습니다: want %d, got %d", 10, copied[0])
	}
}