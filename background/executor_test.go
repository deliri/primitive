package background

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

func backgroundFixture(t *testing.T, concurrent, waiting uint16) *Executor {
	t.Helper()
	duration, err := temporal.DurationFromSeconds(600)
	if err != nil {
		t.Fatal(err)
	}
	e, err := Open(OpenRequest{Parent: t.Context(), Policy: Policy{concurrent, waiting, duration}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := e.Stop(ctx); err != nil {
			t.Errorf("cleanup Stop=%v,want joined nil", err)
		}
	})
	return e
}

func TestBackgroundResidentAdmissionCancellationLayerTriad(t *testing.T) {
	t.Parallel()
	e := backgroundFixture(t, 4, 3)
	started := make(chan struct{}, 4)
	completed := make(chan error, 7)
	var actions atomic.Uint32
	for range 4 {
		if err := e.Submit(Work{Run: func(ctx context.Context) error { actions.Add(1); started <- struct{}{}; <-ctx.Done(); return ctx.Err() }, Complete: func(err error) { completed <- err }}); err != nil {
			t.Fatal(err)
		}
		select {
		case <-started:
		case <-t.Context().Done():
			t.Fatal("worker start unavailable")
		}
	}
	for range 3 {
		if err := e.Submit(Work{Run: func(context.Context) error { actions.Add(1); return nil }, Complete: func(err error) { completed <- err }}); err != nil {
			t.Fatal(err)
		}
	}
	var refusedCalls atomic.Uint32
	if err := e.Submit(Work{Run: func(context.Context) error { refusedCalls.Add(1); return nil }, Complete: func(error) { refusedCalls.Add(1) }}); !errors.Is(err, core.ErrBackgroundCapacity) {
		t.Fatalf("full resident admission=%v,want capacity refusal", err)
	}
	if err := e.Stop(t.Context()); err != nil {
		t.Fatalf("cancel+join=%v,want nil", err)
	}
	for range 7 {
		if err := <-completed; !errors.Is(err, context.Canceled) {
			t.Fatalf("accepted completion=%v,want native cancellation", err)
		}
	}
	if got := actions.Load(); got != 4 {
		t.Fatalf("executed callbacks=%d,want only four started workers", got)
	}
	if got := refusedCalls.Load(); got != 0 {
		t.Fatalf("refused callbacks=%d,want zero effects", got)
	}
	if err := e.Submit(Work{Run: func(context.Context) error { return nil }, Complete: func(error) {}}); !errors.Is(err, core.ErrBackgroundStopped) || !errors.Is(err, context.Canceled) {
		t.Fatalf("sealed Submit=%v,want stopped and native cancellation", err)
	}
}

func TestBackgroundStopCannotCertifyBlockedCompletion(t *testing.T) {
	t.Parallel()
	e := backgroundFixture(t, 1, 1)
	started := make(chan struct{})
	inCompletion := make(chan struct{})
	release := make(chan struct{})
	t.Cleanup(func() {
		select {
		case <-release:
		default:
			close(release)
		}
	})
	if err := e.Submit(Work{Run: func(ctx context.Context) error { close(started); <-ctx.Done(); return ctx.Err() }, Complete: func(error) { close(inCompletion); <-release }}); err != nil {
		t.Fatal(err)
	}
	<-started
	terminal, cancel := context.WithCancel(t.Context())
	cancel()
	if err := e.Stop(terminal); !errors.Is(err, core.ErrBackgroundStopped) || !errors.Is(err, context.Canceled) {
		t.Fatalf("incomplete Stop=%v,want stopped/native canceled", err)
	}
	<-inCompletion
	select {
	case <-e.done:
		t.Fatal("joined notification closed before completion returned")
	default:
	}
	close(release)
	if err := e.Stop(t.Context()); err != nil {
		t.Fatalf("later completed Stop=%v,want joined nil", err)
	}
}

func TestBackgroundParentCancellationRetiresAcceptedWork(t *testing.T) {
	t.Parallel()
	parent, cancel := context.WithCancelCause(t.Context())
	duration, err := temporal.DurationFromSeconds(600)
	if err != nil {
		t.Fatal(err)
	}
	e, err := Open(OpenRequest{Parent: parent, Policy: Policy{1, 1, duration}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cancel(context.Canceled)
		ctx, stop := context.WithTimeout(context.Background(), 5*time.Second)
		defer stop()
		if err := e.Stop(ctx); err != nil {
			t.Errorf("Stop=%v,want nil", err)
		}
	})
	started := make(chan struct{})
	finished := make(chan error, 2)
	var queuedRuns atomic.Uint32
	if err := e.Submit(Work{Run: func(ctx context.Context) error { close(started); <-ctx.Done(); return ctx.Err() }, Complete: func(err error) { finished <- err }}); err != nil {
		t.Fatal(err)
	}
	<-started
	if err := e.Submit(Work{Run: func(context.Context) error { queuedRuns.Add(1); return nil }, Complete: func(err error) { finished <- err }}); err != nil {
		t.Fatal(err)
	}
	cancel(core.ErrReportConflict)
	select {
	case <-e.done:
	case <-t.Context().Done():
		t.Fatal("parent cancellation did not join workers")
	}
	for range 2 {
		if err := <-finished; !errors.Is(err, context.Canceled) || !errors.Is(err, core.ErrReportConflict) {
			t.Fatalf("parent terminal completion=%v,want cancellation and native cause", err)
		}
	}
	if got := queuedRuns.Load(); got != 0 {
		t.Fatalf("queued canceled action runs=%d,want zero", got)
	}
}

func TestBackgroundDeadlineIsCooperativeAndTyped(t *testing.T) {
	t.Parallel()
	duration, err := temporal.DurationFromMilliseconds(1)
	if err != nil {
		t.Fatal(err)
	}
	e, err := Open(OpenRequest{Parent: t.Context(), Policy: Policy{1, 1, duration}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, stop := context.WithTimeout(context.Background(), 5*time.Second)
		defer stop()
		if err := e.Stop(ctx); err != nil {
			t.Errorf("Stop=%v,want nil", err)
		}
	})
	completed := make(chan error, 1)
	if err := e.Submit(Work{Run: func(ctx context.Context) error { <-ctx.Done(); return nil }, Complete: func(err error) { completed <- err }}); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-completed:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("expired action completion=%v,want native deadline rather than false success", err)
		}
	case <-t.Context().Done():
		t.Fatal("cooperative deadline completion unavailable")
	}
}

func TestBackgroundCallbackPanicsKeepIdentityAndWorker(t *testing.T) {
	t.Parallel()
	e := backgroundFixture(t, 1, 1)
	finished := make(chan error, 1)
	if err := e.Submit(Work{Run: func(context.Context) error { panic(core.ErrReportConflict) }, Complete: func(err error) { finished <- err }}); err != nil {
		t.Fatal(err)
	}
	if err := <-finished; !errors.Is(err, core.ErrBackgroundCallbackPanic) || !errors.Is(err, core.ErrReportConflict) {
		t.Fatalf("action panic=%v,want callback panic and native cause", err)
	}
	if err := e.Submit(Work{Run: func(context.Context) error { return nil }, Complete: func(err error) { finished <- err }}); err != nil {
		t.Fatal(err)
	}
	if err := <-finished; err != nil {
		t.Fatalf("next action completion=%v,want live reused worker", err)
	}
}

func TestBackgroundCompletionPanicCannotDisappearFromStop(t *testing.T) {
	t.Parallel()
	duration, err := temporal.DurationFromSeconds(600)
	if err != nil {
		t.Fatal(err)
	}
	e, err := Open(OpenRequest{Parent: t.Context(), Policy: Policy{1, 1, duration}})
	if err != nil {
		t.Fatal(err)
	}
	completed := make(chan struct{})
	if err := e.Submit(Work{Run: func(context.Context) error { return nil }, Complete: func(error) { close(completed); panic(core.ErrReportConflict) }}); err != nil {
		t.Fatal(err)
	}
	<-completed
	if err := e.Stop(t.Context()); !errors.Is(err, core.ErrBackgroundCallbackPanic) || !errors.Is(err, core.ErrReportConflict) {
		t.Fatalf("completion panic Stop=%v,want panic identity and native cause", err)
	}
}

func TestBackgroundRepeatedWindowsHaveNoLifetimeQuota(t *testing.T) {
	t.Parallel()
	e := backgroundFixture(t, 1, 1)
	completed := make(chan error, 1)
	var actions uint32
	work := Work{Run: func(context.Context) error { actions++; return nil }, Complete: func(err error) { completed <- err }}
	for range 4096 {
		if err := e.Submit(work); err != nil {
			t.Fatal(err)
		}
		if err := <-completed; err != nil {
			t.Fatal(err)
		}
	}
	if err := e.Stop(t.Context()); err != nil {
		t.Fatal(err)
	}
	if actions != 4096 {
		t.Fatalf("native actions=%d,want 4096 across one fixed window", actions)
	}
}
