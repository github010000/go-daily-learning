package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
)

// TestGroupCancelsContextOnFirstError checks that the first non-nil error cancels
// the derived context immediately, while the parent context remains active.
func TestGroupCancelsContextOnFirstError(t *testing.T) {
	parent := context.Background()
	g := NewGroup(parent)

	g.Go(func(ctx context.Context) error {
		return errors.New("first failure")
	})

	g.Go(func(ctx context.Context) error {
		<-ctx.Done()
		return fmt.Errorf("saw cancellation: %w", ctx.Err())
	})

	err := g.Wait()
	if err == nil {
		t.Fatalf("Wait() returned nil, want first failure")
	}
	if !strings.Contains(err.Error(), "first failure") {
		t.Fatalf("Wait() = %v, want it to include 'first failure'", err)
	}

	if got := context.Cause(g.ctx); got == nil || got.Error() != "first failure" {
		t.Fatalf("context cause = %v, want first failure", got)
	}
	if got := g.ctx.Err(); got != context.Canceled {
		t.Fatalf("derived ctx.Err() = %v, want context.Canceled", got)
	}
	if got := parent.Err(); got != nil {
		t.Fatalf("parent context should not be canceled, got %v", got)
	}
}

// TestGroupSetLimitKeepsMaxActive starts more tasks than the limit and verifies
// that active count never exceeds the configured limit. Channels are used
// instead of time sleeps to keep the test deterministic and fast.
func TestGroupSetLimitKeepsMaxActive(t *testing.T) {
	const (
		limit = 2
		total = 5
	)

	g := NewGroup(context.Background())
	g.SetLimit(limit)

	var active, maxActive atomic.Int32
	entered := make(chan struct{}, total)
	release := make(chan struct{})
	loopDone := make(chan struct{})

	go func() {
		defer close(loopDone)
		for i := 0; i < total; i++ {
			g.Go(func(ctx context.Context) error {
				cur := active.Add(1)
				updateMax(&maxActive, cur)
				entered <- struct{}{}
				<-release
				active.Add(-1)
				return nil
			})
		}
	}()

	// The first limit tasks must enter. The third Go call is blocked on the
	// semaphore because the first two goroutines are still holding tokens.
	for i := 0; i < limit; i++ {
		<-entered
	}

	if got := active.Load(); got != limit {
		t.Fatalf("active after %d entries = %d, want %d", limit, got, limit)
	}
	if got := maxActive.Load(); got != limit {
		t.Fatalf("maxActive after %d entries = %d, want %d", limit, got, limit)
	}

	close(release)
	<-loopDone
	if err := g.Wait(); err != nil {
		t.Fatalf("Wait() returned %v, want nil", err)
	}

	if got := maxActive.Load(); got != limit {
		t.Fatalf("final maxActive = %d, want %d", got, limit)
	}
}

// TestGroupWaitCancelsEvenWithoutError shows that Wait cancels the derived context
// even when all tasks succeed. This is a common gotcha when reusing the context.
func TestGroupWaitCancelsEvenWithoutError(t *testing.T) {
	g := NewGroup(context.Background())
	g.Go(func(ctx context.Context) error {
		return nil
	})
	if err := g.Wait(); err != nil {
		t.Fatalf("Wait() = %v, want nil", err)
	}
	if got := g.ctx.Err(); got != context.Canceled {
		t.Fatalf("derived ctx.Err() after successful Wait = %v, want context.Canceled", got)
	}
}