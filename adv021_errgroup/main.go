package main

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// Group mimics the core of golang.org/x/sync/errgroup using only the standard library.
// The real package also uses context.WithCancelCause, semaphore-based SetLimit, and
// sync.Once to record exactly one first error.
type Group struct {
	ctx     context.Context
	cancel  context.CancelCauseFunc
	wg      sync.WaitGroup
	errOnce sync.Once
	err     error
	sem     chan struct{}
}

// NewGroup derives a cancellable child context from ctx.
// The parent ctx is never canceled when the child is canceled.
func NewGroup(ctx context.Context) *Group {
	derived, cancel := context.WithCancelCause(ctx)
	return &Group{ctx: derived, cancel: cancel}
}

// SetLimit limits the number of concurrently running goroutines.
// The parameter is int. A limit of 0 removes the limit.
func (g *Group) SetLimit(n int) {
	if n < 0 {
		panic("errgroup: negative limit")
	}
	if n == 0 {
		g.sem = nil
		return
	}
	if g.sem != nil {
		panic("errgroup: limit already set")
	}
	g.sem = make(chan struct{}, n)
}

// Go starts f in a new goroutine unless the context is already canceled.
// The semaphore token is acquired before wg.Add so SetLimit also limits
// the rate at which goroutines are launched, not just their execution.
func (g *Group) Go(f func(ctx context.Context) error) {
	if g.sem != nil {
		select {
		case g.sem <- struct{}{}:
		case <-g.ctx.Done():
			return
		}
	}

	g.wg.Add(1)
	go func() {
		defer g.wg.Done()
		if g.sem != nil {
			defer func() { <-g.sem }()
		}

		if err := f(g.ctx); err != nil {
			g.errOnce.Do(func() {
				g.err = err
				if g.cancel != nil {
					g.cancel(err)
				}
			})
		}
	}()
}

// Wait blocks until all goroutines started by Go have finished.
// It cancels the derived context even on success, matching x/sync/errgroup.
func (g *Group) Wait() error {
	g.wg.Wait()
	if g.cancel != nil {
		g.cancel(g.err)
	}
	return g.err
}

func updateMax(max *atomic.Int32, cur int32) {
	for {
		old := max.Load()
		if cur <= old || max.CompareAndSwap(old, cur) {
			return
		}
	}
}

// runCancellationDemo shows that the first non-nil error cancels the derived
// context immediately. The second goroutine sees ctx.Done() and can stop early.
func runCancellationDemo() {
	fmt.Println("=== cancellation semantics ===")
	parent := context.Background()
	g := NewGroup(parent)

	g.Go(func(ctx context.Context) error {
		return errors.New("worker one failed")
	})

	g.Go(func(ctx context.Context) error {
		<-ctx.Done()
		return fmt.Errorf("worker two stopped: %w, cause: %v", ctx.Err(), context.Cause(ctx))
	})

	err := g.Wait()
	fmt.Printf("first error from group: %v\n", err)
	fmt.Printf("derived ctx after wait: %v, cause: %v\n", g.ctx.Err(), context.Cause(g.ctx))
	fmt.Printf("parent ctx after wait: %v\n", parent.Err())
}

// runLimitDemo shows that SetLimit(2) keeps the maximum observed active
// goroutine count at 2, even when total tasks are 6.
func runLimitDemo() {
	fmt.Println("=== SetLimit demo ===")
	const (
		limit = 2
		total = 6
	)
	g := NewGroup(context.Background())
	g.SetLimit(limit)

	var active, maxActive atomic.Int32
	for i := 0; i < total; i++ {
		g.Go(func(ctx context.Context) error {
			cur := active.Add(1)
			updateMax(&maxActive, cur)

			// Sleep simulates real work that holds a slot long enough to observe.
			// This is only for demonstration; tests use channels instead of sleep.
			time.Sleep(40 * time.Millisecond)

			active.Add(-1)
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		fmt.Printf("limit demo error: %v\n", err)
		return
	}
	fmt.Printf("total tasks: %d, limit: %d, observed maxActive: %d\n", total, limit, maxActive.Load())
}

// runWaitGroupManualDemo shows the manual WaitGroup + error channel pattern.
// The channel must be buffered. If it were unbuffered, the first goroutine
// sending to errCh would block before main reads it, and wg.Wait() would
// never finish because that goroutine would not call wg.Done().
func runWaitGroupManualDemo() {
	fmt.Println("=== WaitGroup + error channel (manual) ===")
	var wg sync.WaitGroup
	errCh := make(chan error, 3)

	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			if id == 2 {
				errCh <- errors.New("manual task failed")
				return
			}
		}(i)
	}

	wg.Wait()
	close(errCh)
	for err := range errCh {
		fmt.Printf("collected error: %v\n", err)
	}
}

func main() {
	runCancellationDemo()
	fmt.Println()
	runLimitDemo()
	fmt.Println()
	runWaitGroupManualDemo()
}