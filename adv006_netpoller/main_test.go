package main

import (
	"runtime"
	"testing"
)

// TestBlockOnNetpollDoesNotLeakGoroutines 는 netpoller 경로로 블로킹된
// goroutine 들이 연결을 닫았을 때 모두 깨어나고, 함수 반환 후 goroutine 수가
// 원래 수준으로 돌아오는지 검증한다. settle 을 0 으로 두어 시간에 의존하지 않는다.
func TestBlockOnNetpollDoesNotLeakGoroutines(t *testing.T) {
	before := runtime.NumGoroutine()
	blockOnNetpoll(10, 0)
	runtime.Gosched()
	after := runtime.NumGoroutine()
	if after > before+2 {
		t.Fatalf("netpoll 이후 goroutine 누수: before=%d after=%d", before, after)
	}
}

// TestBlockOnRawSyscallPipeDoesNotLeakGoroutines 는 netpoller 를 타지 않는
// raw syscall.Read 경로도 쓰기로 해제하면 goroutine 이 모두 반환되는지 검증한다.
// 여기서도 시간 단정은 사용하지 않고 WaitGroup 수거 후 개수만 본다.
func TestBlockOnRawSyscallPipeDoesNotLeakGoroutines(t *testing.T) {
	before := runtime.NumGoroutine()
	blockOnRawSyscallPipe(10, 0)
	runtime.Gosched()
	after := runtime.NumGoroutine()
	if after > before+2 {
		t.Fatalf("raw syscall 이후 goroutine 누수: before=%d after=%d", before, after)
	}
}

// TestCurrentThreadsReadsProcStatus 는 /proc/self/status 가 있는 리눅스 환경에서
// 스레드 수를 0 보다 크게 읽어야 한다는 것을 검증한다. /proc 이 없으면 건너뛴다.
func TestCurrentThreadsReadsProcStatus(t *testing.T) {
	n := currentThreads()
	if n == 0 {
		t.Fatal("스레드 수가 0으로 읽힘")
	}
	if n == -1 {
		t.Skip("/proc/self/status 를 읽을 수 없어 스킵")
	}
}

// BenchmarkBlockOnNetpoll 은 netpoller 경로의 setup/teardown 전체를 상대 비교용으로 측정한다.
func BenchmarkBlockOnNetpoll(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		blockOnNetpoll(10, 0)
	}
}

// BenchmarkBlockOnRawSyscallPipe 는 raw syscall 경로의 setup/teardown 전체를 측정한다.
func BenchmarkBlockOnRawSyscallPipe(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		blockOnRawSyscallPipe(10, 0)
	}
}