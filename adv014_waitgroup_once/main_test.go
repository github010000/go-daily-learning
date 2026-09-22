package main

import (
	"strings"
	"testing"
)

// TestRunCorrectWaitGroup verifies that Add-before-go makes Wait wait for every worker.
// It checks the completion count invariant, not a wall-clock deadline.
func TestRunCorrectWaitGroup(t *testing.T) {
	const n = 16
	got := runCorrectWaitGroup(n)
	if got != n {
		t.Fatalf("runCorrectWaitGroup(%d) = %d, want %d", n, got, n)
	}
}

// TestRunWrongAddInGoroutine verifies that Add inside the child goroutine makes Wait return before completion.
// The function uses a channel fence, so returnedBefore must be 0 even though all workers eventually finish.
func TestRunWrongAddInGoroutine(t *testing.T) {
	const n = 16
	returned, final := runWrongAddInGoroutine(n)
	if final != n {
		t.Fatalf("final completed = %d, want %d", final, n)
	}
	if returned != 0 {
		t.Fatalf("Wait should have returned before any worker registered; returnedBefore = %d, want 0", returned)
	}
}

// TestRunNegativeAddPanic verifies the exact negative counter panic symptom.
// It prevents false confidence from a custom error message.
func TestRunNegativeAddPanic(t *testing.T) {
	msg := runNegativeAddPanic()
	if !strings.Contains(msg, "negative WaitGroup counter") {
		t.Fatalf("panic message = %q, want substring %q", msg, "negative WaitGroup counter")
	}
}

// TestRunOnceExactlyOnce verifies sync.Once calls f exactly once under concurrent Do calls.
// Running with -race also proves no data race in the demonstration path.
func TestRunOnceExactlyOnce(t *testing.T) {
	const n = 64
	got := runOnce(n)
	if got != 1 {
		t.Fatalf("runOnce(%d) f calls = %d, want 1", n, got)
	}
}

// TestRunCountingOnceAfterFirst verifies the double-checked locking shape:
// after the first slow-path call, every later call should hit the lock-free fast path.
// This is an invariant of the implementation, not a time-dependent assertion.
func TestRunCountingOnceAfterFirst(t *testing.T) {
	const n = 64
	fast, slow := runCountingOnceAfterFirst(n)
	if fast != int64(n) {
		t.Fatalf("fast path count = %d, want %d", fast, n)
	}
	if slow != 1 {
		t.Fatalf("slow path count = %d, want 1", slow)
	}
}