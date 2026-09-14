package main

import (
	"runtime"
	"testing"
)

// TestTightLoopSum은 tightLoop가 0부터 n-1까지의 합을 정확히 계산하는지 검증한다.
// tightLoop는 함수 호출이 없는 순수 루프여야 하므로 작은 n으로 빠르게 확인한다.
func TestTightLoopSum(t *testing.T) {
	const n = 1000
	got := tightLoop(n)
	want := uint64(n-1) * uint64(n) / 2 // 0 + 1 + ... + (n-1)
	if got != want {
		t.Fatalf("tightLoop(%d) = %d, want %d", n, got, want)
	}
}

// TestRunTightLoopDemoReturnsSum은 runTightLoopDemo가 실제 고루틴을 띄워
// 합계를 반환하고 시간 불변식이 깨지지 않는지 검증한다. 시간에 의존하는
// 단정은 피하고 결과값과 불변식만 확인한다.
func TestRunTightLoopDemoReturnsSum(t *testing.T) {
	old := runtime.GOMAXPROCS(1)
	defer runtime.GOMAXPROCS(old)

	const n = 10000
	result := runTightLoopDemo(n)
	want := uint64(n-1) * uint64(n) / 2
	if result.sum != want {
		t.Fatalf("runTightLoopDemo(%d) sum = %d, want %d", n, result.sum, want)
	}
	if result.resume < 0 {
		t.Fatalf("resume duration negative: %v", result.resume)
	}
	if result.total < result.resume {
		t.Fatalf("total(%v) < resume(%v)", result.total, result.resume)
	}
}

// BenchmarkTightLoop는 tightLoop의 연산 성능을 측정한다.
// b.ResetTimer로 준비 시간을 제외하고 실제 루프만 측정한다.
func BenchmarkTightLoop(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = tightLoop(1000)
	}
}