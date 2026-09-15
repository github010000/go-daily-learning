package main

import (
	"sync"
	"testing"
)

// TestReadMutexStateUnlocked 는 zero-value sync.Mutex 의 내부 상태가
// 모두 false/0 이어야 함을 검증한다. fast path 이전의 초기 상태를 확인한다.
func TestReadMutexStateUnlocked(t *testing.T) {
	var mu sync.Mutex
	locked, woken, starving, waiters := readMutexState(&mu)

	if locked || woken || starving || waiters != 0 {
		t.Fatalf(
			"unlocked mutex should have locked=false, woken=false, starving=false, waiters=0; got l=%v w=%v s=%v n=%d",
			locked, woken, starving, waiters,
		)
	}
}

// TestReadMutexStateLocked 는 Lock() 이후 locked 비트만 켜지고
// woken/starving/waiters 는 그대로 0 이어야 함을 검증한다.
func TestReadMutexStateLocked(t *testing.T) {
	var mu sync.Mutex
	mu.Lock()
	defer mu.Unlock()

	locked, woken, starving, waiters := readMutexState(&mu)

	if !locked {
		t.Fatalf("locked mutex should have locked=true, got locked=%v", locked)
	}
	if woken || starving || waiters != 0 {
		t.Fatalf(
			"locked mutex should not have woken/starving/waiters; got woken=%v starving=%v waiters=%d",
			woken, starving, waiters,
		)
	}
}

// TestSafeCounterRace 는 sync.Mutex 로 보호된 카운터를 여러 goroutine 이
// 동시에 증가시켰을 때 최종 값이 정확해야 함을 검증한다.
// `go test -race ./...` 로 실행해도 데이터 레이스가 없어야 한다.
func TestSafeCounterRace(t *testing.T) {
	c := &safeCounter{}

	const goroutines = 50
	const increments = 1000
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < increments; j++ {
				c.Inc()
			}
		}()
	}
	wg.Wait()

	got := c.Value()
	want := goroutines * increments
	if got != want {
		t.Fatalf("safeCounter.Value() = %d, want %d", got, want)
	}
}

// BenchmarkMutexUncontended 는 contention 이 없을 때 sync.Mutex 한 쌍의
// Lock/Unlock 비용이 어느 정도인지 측정한다. b.ResetTimer 는 준비 작업이
// 끝난 뒤 호출되어야 한다.
func BenchmarkMutexUncontended(b *testing.B) {
	var mu sync.Mutex

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mu.Lock()
		mu.Unlock()
	}
}