package main

import (
	"reflect"
	"testing"
)

// runWorkers 가 모든 고루틴의 결과를 빠짐없이, 올바른 인덱스에 채웠는지 검증합니다.
// 고루틴 실행 순서는 비결정적이어도 인덱스별 결과는 항상 같아야 합니다.
func TestRunWorkers(t *testing.T) {
	tests := []struct {
		name string
		n    int
		want []int
	}{
		{"빈 입력", 0, []int{}},
		{"하나", 1, []int{0}},
		{"다섯", 5, []int{0, 1, 4, 9, 16}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := runWorkers(tc.n)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("runWorkers(%d) = %v, want %v", tc.n, got, tc.want)
			}
		})
	}
}

// Mutex 로 보호한 카운터가 정확히 n*1000 이 되는지 검증합니다.
// -race 로 돌려도 통과해야 합니다.
func TestSumWithMutex(t *testing.T) {
	tests := []struct {
		n    int
		want int
	}{
		{1, 1000},
		{10, 10000},
		{50, 50000},
	}
	for _, tc := range tests {
		t.Run("", func(t *testing.T) {
			if got := sumWithMutex(tc.n); got != tc.want {
				t.Fatalf("sumWithMutex(%d) = %d, want %d", tc.n, got, tc.want)
			}
		})
	}
}

// Add 를 go 앞에서 부른 고친 버전은 Wait 직후 n 개가 모두 끝나 있어야 합니다.
func TestFixedAddBefore(t *testing.T) {
	for _, n := range []int{0, 1, 10, 100} {
		if got := fixedAddBefore(n); got != n {
			t.Fatalf("fixedAddBefore(%d) = %d, want %d", n, got, n)
		}
	}
}