package background_test

import (
	"context"
	"errors"
	"io"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/deliri/primitive/v2026/background"
	"github.com/deliri/primitive/v2026/core"
)

func TestParentTaskCompletionAndNativeCallbackFailures(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name            string
		actionError     error
		actionPanic     bool
		actionExit      bool
		completionPanic bool
		completionExit  bool
		wantAction      error
		wantCompletion  error
	}{
		{name: "natural completion without an invented deadline"},
		{name: "native action refusal", actionError: io.ErrClosedPipe, wantAction: io.ErrClosedPipe},
		{name: "joined native action causes", actionError: errors.Join(io.ErrClosedPipe, io.ErrUnexpectedEOF), wantAction: io.ErrUnexpectedEOF},
		{name: "action panic retains its native cause", actionPanic: true, wantAction: core.ErrBackgroundCallbackPanic},
		{name: "action Goexit joins as stopped", actionExit: true, wantAction: core.ErrBackgroundStopped},
		{name: "completion panic remains separate", completionPanic: true, wantCompletion: core.ErrBackgroundCallbackPanic},
		{name: "completion Goexit cannot strand its owner", completionExit: true, wantCompletion: core.ErrBackgroundStopped},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			runs, completions := 0, 0
			var observed error
			task, err := background.StartTask(background.TaskRequest{Parent: context.Background(), Work: background.Work{
				Run: func(ctx context.Context) error {
					runs++
					if _, bounded := ctx.Deadline(); bounded {
						return core.ErrBackgroundContract
					}
					if tc.actionPanic {
						panic(io.ErrNoProgress)
					}
					if tc.actionExit {
						runtime.Goexit()
					}
					return tc.actionError
				},
				Complete: func(err error) {
					completions++
					observed = err
					if tc.completionPanic {
						panic(io.ErrShortWrite)
					}
					if tc.completionExit {
						runtime.Goexit()
					}
				},
			}})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _, _ = task.Close() })
			result, err := task.Wait(t.Context())
			if err != nil || !errors.Is(result.ActionError, tc.wantAction) || !errors.Is(result.CompletionError, tc.wantCompletion) || !errors.Is(observed, tc.wantAction) || runs != 1 || completions != 1 {
				t.Fatalf("native task result=%+v wait=%v observed=%v runs/completions=%d/%d", result, err, observed, runs, completions)
			}
			if tc.actionPanic && !errors.Is(result.ActionError, io.ErrNoProgress) {
				t.Fatalf("native panic cause lost: %v", result.ActionError)
			}
			if tc.completionPanic && !errors.Is(result.CompletionError, io.ErrShortWrite) {
				t.Fatalf("completion panic cause lost: %v", result.CompletionError)
			}
			if tc.actionError != nil && !errors.Is(result.ActionError, io.ErrClosedPipe) {
				t.Fatalf("joined action cause lost: %v", result.ActionError)
			}
			again, err := task.Close()
			if err != nil || !errors.Is(again.ActionError, tc.wantAction) || !errors.Is(again.CompletionError, tc.wantCompletion) || completions != 1 {
				t.Fatalf("settled task changed observation or repeated callback: %+v/%v completions=%d", again, err, completions)
			}
		})
	}
}

func TestParentTaskCloseJoinsActionAndCompletionAndKeepsFirstCause(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		entered, release := make(chan struct{}), make(chan struct{})
		observing, releaseObserver := make(chan struct{}), make(chan struct{})
		task, err := background.StartTask(background.TaskRequest{Parent: context.Background(), Work: background.Work{
			Run:      func(ctx context.Context) error { close(entered); <-ctx.Done(); <-release; return context.Cause(ctx) },
			Complete: func(error) { close(observing); <-releaseObserver },
		}})
		if err != nil {
			t.Fatal(err)
		}
		var releaseOnce, observerOnce sync.Once
		defer func() {
			releaseOnce.Do(func() { close(release) })
			observerOnce.Do(func() { close(releaseObserver) })
			_ = task.Cancel(context.Canceled)
			_, _ = task.Wait(context.Background())
		}()
		<-entered
		if err := task.Cancel(io.ErrClosedPipe); err != nil {
			t.Fatal(err)
		}
		if err := task.Cancel(io.ErrUnexpectedEOF); err != nil {
			t.Fatal(err)
		}
		returned := make(chan struct{})
		var result background.TaskResult
		var waitError error
		go func() { result, waitError = task.Close(); close(returned) }()
		synctest.Wait()
		select {
		case <-returned:
			t.Fatal("close claimed a join before action cleanup")
		default:
		}
		releaseOnce.Do(func() { close(release) })
		<-observing
		synctest.Wait()
		select {
		case <-returned:
			t.Fatal("close claimed a join before completion cleanup")
		default:
		}
		observerOnce.Do(func() { close(releaseObserver) })
		<-returned
		if waitError != nil || !errors.Is(result.ActionError, context.Canceled) || !errors.Is(result.ActionError, io.ErrClosedPipe) || errors.Is(result.ActionError, io.ErrUnexpectedEOF) || result.CompletionError != nil {
			t.Fatalf("joined cancellation=%+v/%v, want only first native cause and completed observation", result, waitError)
		}
	})
}

func TestParentTaskFailedWaitDoesNotCancelOrInventAnActionObservation(t *testing.T) {
	t.Parallel()
	entered, release := make(chan struct{}), make(chan struct{})
	task, err := background.StartTask(background.TaskRequest{Parent: context.Background(), Work: background.Work{
		Run: func(ctx context.Context) error { close(entered); <-release; return ctx.Err() }, Complete: func(error) {},
	}})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _, _ = task.Close() }()
	<-entered
	ctx, cancel := context.WithCancelCause(t.Context())
	cancel(io.ErrNoProgress)
	result, err := task.Wait(ctx)
	// Always release the owned callback before reporting an assertion failure.
	close(release)
	completed, completionErr := task.Wait(t.Context())
	if !errors.Is(err, core.ErrBackgroundStopped) || !errors.Is(err, context.Canceled) || !errors.Is(err, io.ErrNoProgress) || result != (background.TaskResult{}) || completionErr != nil || completed != (background.TaskResult{}) {
		t.Fatalf("failed wait=%+v/%v later completion=%+v/%v, want zero refusal and independent successful action", result, err, completed, completionErr)
	}
}

func TestParentTaskRefusesInvalidOrTerminalIngressBeforeBorrowingCallbacks(t *testing.T) {
	t.Parallel()
	canceled, cancel := context.WithCancelCause(t.Context())
	cancel(io.ErrClosedPipe)
	expired, cancelDeadline := context.WithDeadlineCause(t.Context(), time.Unix(0, 0), io.ErrNoProgress)
	defer cancelDeadline()
	for _, tc := range []struct {
		name                   string
		parent                 context.Context
		noAction, noCompletion bool
		want, cause            error
	}{
		{name: "nil parent", want: core.ErrBackgroundContract},
		{name: "missing action", parent: t.Context(), noAction: true, want: core.ErrBackgroundContract},
		{name: "missing completion", parent: t.Context(), noCompletion: true, want: core.ErrBackgroundContract},
		{name: "canceled parent", parent: canceled, want: core.ErrBackgroundStopped, cause: io.ErrClosedPipe},
		{name: "expired parent", parent: expired, want: core.ErrBackgroundStopped, cause: io.ErrNoProgress},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var invoked atomic.Int64
			work := background.Work{Run: func(context.Context) error { invoked.Add(1); return nil }, Complete: func(error) { invoked.Add(1) }}
			if tc.noAction {
				work.Run = nil
			}
			if tc.noCompletion {
				work.Complete = nil
			}
			task, err := background.StartTask(background.TaskRequest{Parent: tc.parent, Work: work})
			if task != nil {
				_, _ = task.Close()
			}
			if task != nil || !errors.Is(err, tc.want) || tc.cause != nil && !errors.Is(err, tc.cause) || invoked.Load() != 0 {
				t.Fatalf("refused ingress=%v/%v callbacks=%d,want no task/borrow and %v/%v", task, err, invoked.Load(), tc.want, tc.cause)
			}
		})
	}
}

func TestParentTaskNilUnsetAndCopiedHandlesCannotSettleTheOriginal(t *testing.T) {
	t.Parallel()
	entered := make(chan context.Context, 1)
	task, err := background.StartTask(background.TaskRequest{Parent: context.Background(), Work: background.Work{
		Run: func(ctx context.Context) error { entered <- ctx; <-ctx.Done(); return context.Cause(ctx) }, Complete: func(error) {},
	}})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _, _ = task.Close() }()
	originalContext := <-entered
	copied := *task
	for _, invalid := range []*background.Task{nil, {}, &copied} {
		if err := invalid.Validate(); !errors.Is(err, core.ErrBackgroundContract) {
			t.Fatalf("unowned validation=%v", err)
		}
		if err := invalid.Cancel(io.ErrNoProgress); !errors.Is(err, core.ErrBackgroundContract) {
			t.Fatalf("unowned cancellation=%v", err)
		}
		if result, err := invalid.Wait(t.Context()); result != (background.TaskResult{}) || !errors.Is(err, core.ErrBackgroundContract) {
			t.Fatalf("unowned wait=%+v/%v", result, err)
		}
		if result, err := invalid.Close(); result != (background.TaskResult{}) || !errors.Is(err, core.ErrBackgroundContract) {
			t.Fatalf("unowned close=%+v/%v", result, err)
		}
	}
	if originalContext.Err() != nil {
		t.Fatalf("copied handle canceled original: %v", originalContext.Err())
	}
	if result, err := task.Wait(nil); result != (background.TaskResult{}) || !errors.Is(err, core.ErrBackgroundContract) {
		t.Fatalf("nil wait context=%+v/%v", result, err)
	}
	result, err := task.Close()
	if err != nil || !errors.Is(result.ActionError, context.Canceled) || result.CompletionError != nil {
		t.Fatalf("original close=%+v/%v", result, err)
	}
}

func TestParentTaskConcurrentCloseObservesOneCompletedCallback(t *testing.T) {
	t.Parallel()
	entered := make(chan struct{})
	var completions atomic.Int64
	task, err := background.StartTask(background.TaskRequest{Parent: context.Background(), Work: background.Work{
		Run:      func(ctx context.Context) error { close(entered); <-ctx.Done(); return ctx.Err() },
		Complete: func(error) { completions.Add(1) },
	}})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _, _ = task.Close() }()
	<-entered
	var owners sync.WaitGroup
	for range 8 {
		owners.Go(func() {
			result, err := task.Close()
			if err != nil || !errors.Is(result.ActionError, context.Canceled) || result.CompletionError != nil {
				t.Errorf("concurrent close=%+v/%v, want one completed cancellation", result, err)
			}
		})
	}
	owners.Wait()
	if completions.Load() != 1 {
		t.Fatalf("completion observations=%d, want one", completions.Load())
	}
}

func TestParentTaskKeepsParentCauseWhenActionReturnsNoError(t *testing.T) {
	t.Parallel()
	parent, cancel := context.WithCancelCause(t.Context())
	defer cancel(nil)
	entered := make(chan struct{})
	var observed error
	task, err := background.StartTask(background.TaskRequest{Parent: parent, Work: background.Work{
		Run:      func(ctx context.Context) error { close(entered); <-ctx.Done(); return nil },
		Complete: func(err error) { observed = err },
	}})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _, _ = task.Close() }()
	<-entered
	cancel(errors.Join(io.ErrClosedPipe, io.ErrNoProgress))
	result, err := task.Wait(t.Context())
	if err != nil || result.CompletionError != nil || !errors.Is(result.ActionError, context.Canceled) || !errors.Is(result.ActionError, io.ErrClosedPipe) || !errors.Is(result.ActionError, io.ErrNoProgress) || !errors.Is(observed, io.ErrNoProgress) {
		t.Fatalf("parent cancellation=%+v/%v observed=%v, want exact standard and joined native causes", result, err, observed)
	}
}

func FuzzParentTaskCompletionSemanticConservation(f *testing.F) {
	for _, selector := range []uint8{0, 1, 2, 3, 4} {
		f.Add(selector)
	}
	f.Fuzz(func(t *testing.T, selector uint8) {
		var want error
		switch selector % 5 {
		case 1:
			want = io.ErrClosedPipe
		case 2:
			want = io.ErrUnexpectedEOF
		case 3:
			want = core.ErrBackgroundCallbackPanic
		case 4:
			want = core.ErrBackgroundStopped
		}
		calls, observations := 0, 0
		var observed error
		task, err := background.StartTask(background.TaskRequest{Parent: t.Context(), Work: background.Work{
			Run: func(context.Context) error {
				calls++
				switch selector % 5 {
				case 1:
					return io.ErrClosedPipe
				case 2:
					return errors.Join(io.ErrClosedPipe, io.ErrUnexpectedEOF)
				case 3:
					panic(io.ErrNoProgress)
				case 4:
					runtime.Goexit()
				}
				return nil
			},
			Complete: func(err error) { observations++; observed = err },
		}})
		if err != nil {
			t.Fatal(err)
		}
		result, err := task.Wait(t.Context())
		closed, closeErr := task.Close()
		if err != nil || closeErr != nil || calls != 1 || observations != 1 || !errors.Is(observed, want) || !errors.Is(result.ActionError, want) || !errors.Is(closed.ActionError, want) || result.CompletionError != nil || closed.CompletionError != nil {
			t.Fatalf("action/observation closure=%+v/%v close=%+v/%v observed=%v calls=%d/%d want=%v", result, err, closed, closeErr, observed, calls, observations, want)
		}
		if selector%5 == 2 && (!errors.Is(result.ActionError, io.ErrClosedPipe) || !errors.Is(observed, io.ErrClosedPipe)) {
			t.Fatal("joined native cause lost")
		}
		if selector%5 == 3 && !errors.Is(result.ActionError, io.ErrNoProgress) {
			t.Fatal("native panic cause lost")
		}
	})
}
