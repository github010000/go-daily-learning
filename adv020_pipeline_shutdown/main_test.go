package main

import (
	"context"
	"testing"
)

// TestProducerStopsWhenContextCanceled는 소비자보다 먼저 취소가 일어난 경우
// Produce가 값을 보내지 않고 channel을 닫는지를 확인한다.
// cancel을 Produce 호출 전에 호출하면 Produce는 goroutine을 만들지 않고
// 이미 닫힌 channel을 돌려줘야 한다. 이 단정은 타이머에 의존하지 않는다.
func TestProducerStopsWhenContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Produce가 시작되기 전에 취소

	out := Produce(ctx, 10, 20, 30)

	var got []int
	for v := range out {
		got = append(got, v)
	}
	if len(got) != 0 {
		t.Fatalf("producer sent values after cancel: %v", got)
	}
}

// TestFindFirstCancelsContext는 target을 찾았을 때 FindFirst가 cancel을
// 호출하는지, 그리고 cancel이 upstream으로 전파되어 merged channel이 닫히는지를
// 검증한다. cancel이 없으면 아래 for range merged가 영원히 끝나지 않아
// 테스트가 hang 한다. 따라서 여기서는 시간 제한이 아니라 close 여부를 본다.
func TestFindFirstCancelsContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel() // 테스트가 중간에 실패해도 goroutine leak를 막는 안전장치

	in := Produce(ctx, 1, 2, 3, 4, 5)
	workers := []<-chan int{
		Square(ctx, in),
		Square(ctx, in),
		Square(ctx, in),
	}
	merged := Merge(ctx, workers...)

	got, ok := FindFirst(ctx, cancel, merged, 9)
	if !ok || got != 9 {
		t.Fatalf("FindFirst = (%d, %v), want (9, true)", got, ok)
	}
	if ctx.Err() == nil {
		t.Fatalf("context should be canceled after target found")
	}

	// cancel이 제대로 전파되지 않았다면 이 for range는 merged가 닫히지 않아
	// 멈춘다. 닫힘 자체가 검증하려는 불변식이다.
	for range merged {
	}
}

// TestMergeFansInAllValues는 fan-out/fan-in이 입력마다 정확히 하나의 제곱값을
// 보존하는지 확인한다. worker가 세 개여도 각 입력은 무조건 한 번만 소비되어야
// 하고, 중복이나 유실이 없어야 한다.
func TestMergeFansInAllValues(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	in := Produce(ctx, 1, 2, 3, 4, 5)
	const workers = 3
	workerChans := make([]<-chan int, workers)
	for i := 0; i < workers; i++ {
		workerChans[i] = Square(ctx, in)
	}
	merged := Merge(ctx, workerChans...)

	got := make(map[int]int)
	for v := range merged {
		got[v]++
	}

	want := map[int]int{
		1:  1,
		4:  1,
		9:  1,
		16: 1,
		25: 1,
	}
	if len(got) != len(want) {
		t.Fatalf("got %d distinct squares, want %d: %v", len(got), len(want), got)
	}
	for k, wantCount := range want {
		if got[k] != wantCount {
			t.Fatalf("square %d count = %d, want %d", k, got[k], wantCount)
		}
	}
}