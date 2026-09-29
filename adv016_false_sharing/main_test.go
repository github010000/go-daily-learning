package main

import (
	"sync/atomic"
	"testing"
)

// TestPaddedCountersSeparateLines 는 패딩이 실제로 b 를 다음 캐시 라인으로
// 보냈는지 검증한다. false sharing 수정이 구조적으로 유효한지 확인하는 테스트다.
func TestPaddedCountersSeparateLines(t *testing.T) {
	if got := falseSharingOffsetB(); got >= uintptr(cacheLineSize) {
		t.Fatalf("FalseSharingCounters.b offset=%d, want < %d to be in same line", got, cacheLineSize)
	}
	if got := paddedOffsetB(); got < uintptr(cacheLineSize) {
		t.Fatalf("PaddedCounters.b offset=%d, want >= %d to be in different line", got, cacheLineSize)
	}

	// PaddedCounters.b 는 캐시 라인 경계인 64바이트 지점부터 시작해야
	// a 와 같은 라인에 들어갈 수 없다. 이 검증은 unsafe 없이 offset 값으로
	// 구조적 불변식을 확인한다.
}

// TestRunCounts 는 false sharing 이 정확성 문제가 아니라 성능 문제라는
// 것을 확인한다. 어느 배치든 각 goroutine 은 정확히 n 번 증가시켜야 하고,
// 동시성 상황에서도 카운터가 새거나 사라지면 안 된다.
func TestRunCounts(t *testing.T) {
	const n = 10_000

	a, b := runFalseSharing(n)
	if a != n || b != n {
		t.Fatalf("runFalseSharing(%d) = (%d, %d), want (%d, %d)", n, a, b, n, n)
	}

	a, b = runPadded(n)
	if a != n || b != n {
		t.Fatalf("runPadded(%d) = (%d, %d), want (%d, %d)", n, a, b, n, n)
	}
}

// BenchmarkFalseSharing 는 잘못된 배치에서 두 goroutine 이 b.N 번씩 증가할 때
// 걸리는 시간을 잰다. b.ResetTimer 는 goroutine 생성 후, start 채널을 닫기
// 직전에 호출해 측정 구간을 실제 증가 작업으로 한정한다.
func BenchmarkFalseSharing(b *testing.B) {
	var c FalseSharingCounters
	n := b.N
	start := make(chan struct{})
	done := make(chan struct{}, 2)

	go func() {
		<-start
		for i := 0; i < n; i++ {
			atomic.AddInt64(&c.a, 1)
		}
		done <- struct{}{}
	}()
	go func() {
		<-start
		for i := 0; i < n; i++ {
			atomic.AddInt64(&c.b, 1)
		}
		done <- struct{}{}
	}()

	b.ResetTimer()
	close(start)
	<-done
	<-done
}

// BenchmarkPadded 는 패딩으로 라인을 분리한 배치에서 같은 증가 작업을 잰다.
// BenchmarkFalseSharing 보다 ns/op 가 낮아야 false sharing 이 성능에 실제로
// 영향을 주고 있음을 알 수 있다.
func BenchmarkPadded(b *testing.B) {
	var c PaddedCounters
	n := b.N
	start := make(chan struct{})
	done := make(chan struct{}, 2)

	go func() {
		<-start
		for i := 0; i < n; i++ {
			atomic.AddInt64(&c.a, 1)
		}
		done <- struct{}{}
	}()
	go func() {
		<-start
		for i := 0; i < n; i++ {
			atomic.AddInt64(&c.b, 1)
		}
		done <- struct{}{}
	}()

	b.ResetTimer()
	close(start)
	<-done
	<-done
}