package main

import (
	"runtime"
	"sync"
	"testing"
	"time"
)

// TestDeepRecursionPointerIntegrity 는 깊은 재귀로 stack 성장과 복사를
// 여러 번 유도한 뒤에도 heap 객체에 대한 포인터가 깨지지 않는지 검증합니다.
// -race 로 실행해도 통과해야 하며, 시간에 의존하지 않습니다.
func TestDeepRecursionPointerIntegrity(t *testing.T) {
	target := &pointerTarget{value: 0}

	for _, depth := range []int{128, 256, 512, 1024, 2048} {
		ok := fillStack(depth, target)
		if !ok {
			t.Fatalf("depth %d: fillStack 가 false 를 반환했습니다.", depth)
		}
		if target.value != 42 {
			t.Fatalf("depth %d 후 target.value=%d, 42 기대. stack 복사가 포인터를 깨뜨렸습니다.",
				depth, target.value)
		}
		target.value = 0
	}
}

// TestTraceLenGrowsWithDepth 는 재귀 깊이가 늘어나면 runtime.Stack 이
// 돌려주는 trace 길이가 늘어나는지 확인합니다.
// 이 값은 stack 메모리 크기가 아니라 프레임 수에 비례하는 값이므로
// 깊은 호출이 프레임을 늘린다는 사실을 안정적으로 검증합니다.
func TestTraceLenGrowsWithDepth(t *testing.T) {
	shallow := traceLenAtDepth(0)
	deep := traceLenAtDepth(200)

	if deep <= shallow {
		t.Fatalf("재귀 깊이가 늘었는데 trace 길이가 늘지 않았습니다: shallow=%d, deep=%d",
			shallow, deep)
	}
}

// TestManyGoroutinesMemoryUsage 는 2KB 시작 stack 정책 덕분에
// 수천 개 goroutine 의 stack 총 사용량이 OS thread 1MB 기준보다
// 훨씬 적은 수준인지 확인합니다.
// 정확한 수치 대신 합리적인 상한을 두어, goroutine 하나당 1MB 씩
// 썼다면 통과할 수 없는 값으로 설정합니다.
func TestManyGoroutinesMemoryUsage(t *testing.T) {
	const goroutineCount = 5_000
	const maxExpectedBytes = 1 << 30 // 1GB. 실패해야 할 경우엔 5GB 가 됩니다.

	var wg sync.WaitGroup
	wg.Add(goroutineCount)

	for i := 0; i < goroutineCount; i++ {
		go func() {
			defer wg.Done()
			var x int
			x = 1
			_ = x
			time.Sleep(1 * time.Millisecond)
		}()
	}

	wg.Wait()

	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	if m.StackInuse > maxExpectedBytes {
		t.Fatalf("goroutine %d개의 stack 사용량이 너무 큽니다: %d bytes",
			goroutineCount, m.StackInuse)
	}
}

// BenchmarkFillStack 은 깊은 재귀로 stack 성장과 복사를 반복할 때
// 드는 비용을 측정합니다. b.ResetTimer 를 반복 시작 직전에 호출해
// 준비 비용을 측정에서 제외합니다.
func BenchmarkFillStack(b *testing.B) {
	target := &pointerTarget{}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = fillStack(256, target)
	}
}

// BenchmarkSimulateBoundaryCrossing 은 연속 stack 에서 아주 작은 함수
// 호출이 얼마나 빠른지 측정합니다. segment stack 시절에는 이 수치가
// 경계를 넘을 때마다 나빠졌습니다.
func BenchmarkSimulateBoundaryCrossing(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = simulateBoundaryCrossing(i)
	}
}