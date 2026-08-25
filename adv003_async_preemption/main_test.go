package main

import (
	"testing"
)

// TestCooperativePreemptionWithYield: 협조적 선점 지점(Gosched)이 있으면
// GOMAXPROCS=1 에서도 다른 goroutine 이 양보 없이 진행되는지 확인한다.
// useYield=true 이므로 ok 가 true 여야 한다.
func TestCooperativePreemptionWithYield(t *testing.T) {
	ok, _ := runPreemptionDemo(true)
	if !ok {
		t.Fatalf("yield 모드에서는 다른 goroutine 이 진행되어야 하는데 ok=false")
	}
}

// TestAsyncPreemptionNoYield: 함수 호출 없는 tight loop 에서도
// 1.14+ 비동기 선점 덕분에 다른 goroutine 이 진행되는지 확인한다.
// 이 테스트는 GODEBUG=asyncpreemptoff=1 이면 실패하도록 작성해
// 옛날 협조적 선점 한계를 재현할 수 있게 한다.
func TestAsyncPreemptionNoYield(t *testing.T) {
	ok, _ := runPreemptionDemo(false)
	if !ok {
		t.Fatalf("no-yield 모드에서는 비동기 선점이 있어야 하는데 ok=false (혹시 GODEBUG=asyncpreemptoff=1 인가?)")
	}
}