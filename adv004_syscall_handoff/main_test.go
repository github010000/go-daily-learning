package main

import (
	"runtime"
	"sync/atomic"
	"testing"
	"time"
)

// TestGoodBlockingSyscallReturns 는 goodBlockingSyscall 이 정상적으로 반환하는지 확인한다.
// syscall.Select 경로가 P handoff 과 관계없이 제어를 반환하는지 검증한다.
func TestGoodBlockingSyscallReturns(t *testing.T) {
	goodBlockingSyscall(1 * time.Millisecond)
}

// TestBadBlockingSyscallReturns 는 badBlockingSyscall 이 반환하는지 확인한다.
// RawSyscall6 경로가 프로세스를 멈추지 않도록 회귀 테스트한다.
func TestBadBlockingSyscallReturns(t *testing.T) {
	badBlockingSyscall(1 * time.Millisecond)
}

// TestSpinCounterIncrementsAndStops 는 spinCounter 가 count 를 올리고 stop 채널을 받으면
// 정상 종료하는지 검증한다. 시간 단정 대신 count 가 0 인 동안 runtime.Gosched 로
// 스케줄러를 양보해 실제 실행을 기다린다.
func TestSpinCounterIncrementsAndStops(t *testing.T) {
	var count int64
	stop, done := startSpinWorker(&count)

	for atomic.LoadInt64(&count) == 0 {
		runtime.Gosched()
	}
	close(stop)
	<-done

	if atomic.LoadInt64(&count) == 0 {
		t.Fatalf("spinCounter did not increment")
	}
}

// BenchmarkGoodBlockingSyscall 은 P handoff 가 일어나는 syscall.Select 의 반복 비용을 잰다.
// entersyscall/exitsyscall 전환 오버헤드를 포함한 syscall 비용을 관찰하기 위한 벤치마크다.
func BenchmarkGoodBlockingSyscall(b *testing.B) {
	for i := 0; i < b.N; i++ {
		goodBlockingSyscall(1 * time.Microsecond)
	}
}

// BenchmarkBadBlockingSyscall 은 RawSyscall6 인 select 의 반복 비용을 잰다.
// handoff 없이 M 이 블로킹되는 비용을 비교할 수 있다.
func BenchmarkBadBlockingSyscall(b *testing.B) {
	for i := 0; i < b.N; i++ {
		badBlockingSyscall(1 * time.Microsecond)
	}
}