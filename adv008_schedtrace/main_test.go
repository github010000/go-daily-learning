package main

import (
	"context"
	"testing"
)

// schedtrace 한 줄의 실제 필드 형식이 parseSchedtraceLine 에서
// 사람이 읽는 SchedStats 로 올바르게 변환되는지 검증한다.
func TestParseSchedtraceLine(t *testing.T) {
	line := `SCHED 1000ms: gomaxprocs=8 idleprocs=4 threads=7 spinningthreads=0 needspinning=1 idlethreads=3 runqueue=2 [0 1 0 0 0 0 0 0]`
	got, err := parseSchedtraceLine(line)
	if err != nil {
		t.Fatalf("parseSchedtraceLine(%q) returned error: %v", line, err)
	}

	if got.Gomaxprocs != 8 {
		t.Errorf("Gomaxprocs = %d, want 8", got.Gomaxprocs)
	}
	if got.Idleprocs != 4 {
		t.Errorf("Idleprocs = %d, want 4", got.Idleprocs)
	}
	if got.Threads != 7 {
		t.Errorf("Threads = %d, want 7", got.Threads)
	}
	if got.Spinningthreads != 0 {
		t.Errorf("Spinningthreads = %d, want 0", got.Spinningthreads)
	}
	if got.Needspinning != 1 {
		t.Errorf("Needspinning = %d, want 1", got.Needspinning)
	}
	if got.Idlethreads != 3 {
		t.Errorf("Idlethreads = %d, want 3", got.Idlethreads)
	}
	if got.Runqueue != 2 {
		t.Errorf("Runqueue = %d, want 2", got.Runqueue)
	}
	if len(got.PQueues) != 8 {
		t.Fatalf("len(PQueues) = %d, want 8", len(got.PQueues))
	}
	if got.PQueues[1] != 1 {
		t.Errorf("PQueues[1] = %d, want 1", got.PQueues[1])
	}
}

// schedtrace 가 아닌 문자열을 넣었을 때 error 를 반환하는지 검증한다.
// 이 테스트는 잘못된 입력을 조용히 넘기고 0 값을 주는 일을 막는다.
func TestParseSchedtraceLineInvalid(t *testing.T) {
	_, err := parseSchedtraceLine("not a sched line")
	if err == nil {
		t.Fatal("expected error for invalid line, got nil")
	}
}

// worker pool 의 모든 task 가 정상적으로 완료되는지 완료 개수로 검증한다.
// 시간에 의존하지 않고, 반환된 Completed 가 tasks 와 같은지만 확인한다.
func TestRunSchedulerDemoCompletesAllTasks(t *testing.T) {
	ctx := context.Background()
	stats := runSchedulerDemo(ctx, 4, 100)

	if stats.Started != 4 {
		t.Errorf("Started = %d, want 4", stats.Started)
	}
	if stats.Completed != 100 {
		t.Errorf("Completed = %d, want 100", stats.Completed)
	}
}