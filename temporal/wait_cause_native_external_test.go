package temporal_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

// The embedded native context owns cancellation/cause. Done observation only
// synchronizes the test with Wait's acquired timer and select inputs.
type waitAcquiredContext struct {
	context.Context
	observed chan struct{}
	once     sync.Once
}

func (c *waitAcquiredContext) Done() <-chan struct{} {
	c.once.Do(func() { close(c.observed) })
	return c.Context.Done()
}

func TestWaitPreservesCauseAfterNativeTimerAcquisition(t *testing.T) {
	t.Parallel()
	parent, cancel := context.WithCancelCause(context.Background())
	ctx := &waitAcquiredContext{Context: parent, observed: make(chan struct{})}
	duration, err := temporal.DurationFromSeconds(60)
	if err != nil {
		cancel(nil)
		t.Fatalf("timer duration=%v, want nil", err)
	}
	done := make(chan error, 1)
	finished := false
	t.Cleanup(func() {
		cancel(nil)
		if !finished {
			select {
			case <-done:
			case <-time.After(10 * time.Second):
				t.Error("owned Wait goroutine did not exit after cleanup cancellation")
			}
		}
	})
	go func() { done <- temporal.Wait(temporal.WaitRequest{Context: ctx, Duration: duration}) }()
	select {
	case <-ctx.observed:
	case <-done:
		finished = true
		t.Fatal("Wait returned before native timer/select acquisition")
	case <-time.After(10 * time.Second):
		t.Fatal("Wait did not acquire its native timer/select inputs")
	}
	cancel(core.ErrContextObservation)
	select {
	case got := <-done:
		finished = true
		if !errors.Is(got, context.Canceled) || !errors.Is(got, core.ErrContextObservation) {
			t.Fatalf("acquired Wait=%v, want native cancellation and original cause", got)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("owned Wait did not exit after native cancellation")
	}
}
