package main

import (
	"reflect"
	"testing"
)

// 파이프라인이 입력 순서를 지키며 두 번 제곱하는지 검증합니다.
// 단계마다 고루틴이 하나뿐이면 순서가 보존된다는 사실도 함께 확인합니다.
func TestPipeline(t *testing.T) {
	tests := []struct {
		name string
		in   []int
		want []int
	}{
		{"빈 입력", nil, nil},
		{"하나", []int{2}, []int{16}},
		{"여럿", []int{1, 2, 3}, []int{1, 16, 81}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Pipeline(tc.in)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

// 작업자 수가 달라도 fan-out/fan-in 결과(정렬 후)가 같아야 함을 검증합니다.
// 도착 순서는 비결정적이므로 정렬된 결과만 비교합니다.
func TestFanOut(t *testing.T) {
	nums := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	want := []int{1, 4, 9, 16, 25, 36, 49, 64, 81, 100}
	for _, workers := range []int{1, 2, 3, 8} {
		t.Run("workers", func(t *testing.T) {
			got := FanOut(nums, workers)
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("workers=%d got %v, want %v", workers, got, want)
			}
		})
	}
}

// done 을 닫으면 일부만 읽어도 모든 고루틴이 종료되는지 검증합니다.
func TestSafeFirstNoLeak(t *testing.T) {
	v, active := SafeFirst([]int{3, 4, 5})
	if v != 9 {
		t.Fatalf("첫 값 got %d, want 9", v)
	}
	if active != 0 {
		t.Fatalf("살아 있는 고루틴 %d 개", active)
	}
}

// 입력이 없어도 merge 가 닫혀 교착 없이 끝나는지 검증합니다.
func TestFanOutEmpty(t *testing.T) {
	if got := FanOut(nil, 3); len(got) != 0 {
		t.Fatalf("got %v, want empty", got)
	}
}