package main

import (
	"strings"
	"testing"
)

// TestSumConcurrentlySafeKeepsTotal은 atomic 연산이 여러 goroutine에서
// 동시에 수행되어도 최종 합계가 정확히 유지되는지 검증한다.
// 만약 한 번이라도 갱신이 유실되면 실패해야 한다.
func TestSumConcurrentlySafeKeepsTotal(t *testing.T) {
	const goroutines = 10
	const perGoroutine = 1000
	got := sumConcurrentlySafe(goroutines)
	want := int64(goroutines * perGoroutine)
	if got != want {
		t.Fatalf("sumConcurrentlySafe(%d) = %d, want %d", goroutines, got, want)
	}
}

// TestVectorClockDemoShowsRaceAndSync는 vectorClockDemo 출력이
// 비동기 쓰기는 race? true로, 채널 동기화 쓰기는 race? false로
// 올바르게 표시하는지 확인한다. 이 함수는 race detector와 무관하게
// 결정적 출력을 검증하므로 -race에서도 안전하다.
func TestVectorClockDemoShowsRaceAndSync(t *testing.T) {
	out := vectorClockDemo()
	if !strings.Contains(out, "race? true") {
		t.Fatalf("expected unsynchronized writes to be labeled race? true, got:\n%s", out)
	}
	if !strings.Contains(out, "race? false") {
		t.Fatalf("expected synchronized writes to be labeled race? false, got:\n%s", out)
	}
}

// BenchmarkMemoryAccess는 작은 메모리 접근 부하를 측정한다.
// go test -bench=. -benchmem 와 go test -bench=. -benchmem -race 를
// 번갈아 실행하면 race detector의 실행 시간 오버헤드를 확인할 수 있다.
// b.ResetTimer는 슬라이스 할당 비용을 측정에서 제외하기 위해 사용한다.
func BenchmarkMemoryAccess(b *testing.B) {
	data := make([]int, 1)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		data[0]++
	}
}