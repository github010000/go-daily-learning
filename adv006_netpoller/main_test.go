package main

import (
    "runtime"
    "testing"
)

// TestParkNetworkReadsParksGoroutines는 network read에서 기다리는 goroutine이
// 실제로 netpoller에 park되는지 스택 트레이스로 검증한다.
// sleep 같은 시간 기반 대기를 쓰지 않아 부하가 높은 CI에서도 안정적이다.
func TestParkNetworkReadsParksGoroutines(t *testing.T) {
    old := runtime.GOMAXPROCS(1)
    defer runtime.GOMAXPROCS(old)

    demo, err := parkNetworkReads(3)
    if err != nil {
        t.Fatalf("parkNetworkReads failed: %v", err)
    }
    defer demo.close()

    got := countStackNeedle("internal/poll.(*FD).Read")
    if got < 3 {
        t.Errorf("expected at least 3 goroutines parked in internal/poll.(*FD).Read, got %d", got)
    }
}

// TestCPUMakesProgressWhileNetworkReadsAreParked는 net.Conn.Read가 goroutine만
// 재우고 M을 살려두는지 확인한다. GOMAXPROCS=1로 고정한 뒤 CPU worker가 목표까지
// 도달하는지 검증한다. 시간 단정 없이 조건 충족까지 runtime.Gosched로 양보한다.
func TestCPUMakesProgressWhileNetworkReadsAreParked(t *testing.T) {
    old := runtime.GOMAXPROCS(1)
    defer runtime.GOMAXPROCS(old)

    demo, err := parkNetworkReads(3)
    if err != nil {
        t.Fatalf("parkNetworkReads failed: %v", err)
    }
    defer demo.close()

    got := runCPUWorkUntilTarget(1000)
    if got < 1000 {
        t.Errorf("expected CPU work target 1000, got %d", got)
    }
}