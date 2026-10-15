package process_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/process"
)

func TestExecutionScopeNativeAdmissionAndCompletion(t *testing.T) {
	t.Parallel()
	for _, refused := range []bool{false, true} {
		t.Run(fmt.Sprintf("refused_%v", refused), func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithTimeout(t.Context(), processTestBackstop)
			defer cancel()
			var scope process.ExecutionScope
			request := processRequest(t, "silent", process.Streams{Stdin: bytes.NewReader(nil), Stdout: io.Discard, Stderr: io.Discard})
			if refused {
				request.Command = absolutePath(t, filepath.Join(t.TempDir(), "missing-executable"))
			}
			execution, err := scope.Begin(ctx, request)
			if refused {
				if execution != nil || !errors.Is(err, os.ErrNotExist) || scope.ActiveCount() != 0 {
					t.Fatalf("refused begin=%v/%v active=%d, want original native cause and empty custody", execution, err, scope.ActiveCount())
				}
				return
			}
			if err != nil || execution == nil || scope.ActiveCount() != 1 {
				t.Fatalf("begin=%v/%v active=%d, want one native child", execution, err, scope.ActiveCount())
			}
			defer func() {
				cancel()
				if scope.ActiveCount() != 0 {
					if _, err := scope.Wait(process.ExecutionWaitRequest{Execution: execution}); err != nil && !errors.Is(err, context.Canceled) {
						t.Error(err)
					}
				}
			}()
			visits := 0
			err = scope.VisitActive(ctx, process.ActiveExecutionRequest{Visit: func(active *process.Execution) error {
				visits++
				if active != execution {
					return core.ErrProcessContract
				}
				_, err := active.Identity()
				return err
			}})
			if err != nil || visits != 1 {
				t.Fatalf("visits=%d error=%v, want same single native capability", visits, err)
			}
			result, err := scope.Wait(process.ExecutionWaitRequest{Execution: execution})
			if err != nil || result.Validate() != nil || scope.ActiveCount() != 0 {
				t.Fatalf("wait=%v validation=%v active=%d, want complete native result and empty custody", err, result.Validate(), scope.ActiveCount())
			}
			visits = 0
			err = scope.VisitActive(ctx, process.ActiveExecutionRequest{Visit: func(*process.Execution) error { visits++; return nil }})
			if err != nil || visits != 0 {
				t.Fatalf("completed visits=%d error=%v, want neutral empty custody", visits, err)
			}
			if _, err := scope.Wait(process.ExecutionWaitRequest{Execution: execution}); !errors.Is(err, core.ErrProcessContract) {
				t.Fatalf("second wait=%v, want typed single-wait refusal", err)
			}
		})
	}
}

func TestExecutionScopeNativeContextAndVisitorRefusal(t *testing.T) {
	t.Parallel()
	canceled, cancel := context.WithCancelCause(t.Context())
	cancel(core.ErrTemporalContract)
	for _, tc := range []struct {
		name  string
		ctx   context.Context
		scope *process.ExecutionScope
		visit func(*process.Execution) error
		want  error
	}{
		{name: "nil scope refuses live visit", ctx: t.Context(), want: core.ErrProcessContract, visit: func(*process.Execution) error { return nil }},
		{name: "nil context refuses before visitation", scope: new(process.ExecutionScope), want: core.ErrNilContext, visit: func(*process.Execution) error { return nil }},
		{name: "custom cancellation remains native", ctx: canceled, scope: new(process.ExecutionScope), want: context.Canceled, visit: func(*process.Execution) error { return nil }},
		{name: "missing callback refuses empty scope", ctx: t.Context(), scope: new(process.ExecutionScope), want: core.ErrProcessContract},
		{name: "neutral empty scope performs no visit", ctx: t.Context(), scope: new(process.ExecutionScope), visit: func(*process.Execution) error { return core.ErrTemporalContract }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := tc.scope.VisitActive(tc.ctx, process.ActiveExecutionRequest{Visit: tc.visit})
			if !errors.Is(err, tc.want) || tc.scope.ActiveCount() != 0 {
				t.Fatalf("visit=%v active=%d,want %v and empty custody", err, tc.scope.ActiveCount(), tc.want)
			}
			if tc.ctx == canceled && !errors.Is(err, core.ErrTemporalContract) {
				t.Fatalf("cancellation=%v,want original custom cause too", err)
			}
		})
	}
	var absent *process.ExecutionScope
	request := processRequest(t, "silent", process.Streams{Stdin: bytes.NewReader(nil), Stdout: io.Discard, Stderr: io.Discard})
	if execution, err := absent.Begin(t.Context(), request); execution != nil || !errors.Is(err, core.ErrProcessContract) {
		t.Fatalf("nil scope begin=%v/%v,want no native child and typed refusal", execution, err)
	}
	if _, err := absent.Wait(process.ExecutionWaitRequest{}); !errors.Is(err, core.ErrProcessContract) {
		t.Fatalf("nil scope wait=%v,want typed refusal", err)
	}
}

func FuzzExecutionScopeConservesNativeCustody(f *testing.F) {
	for _, choice := range []uint8{0, 1, 2, 3, 4, 7, 8, 16, 20, 23, 31, 32, 48, 255} {
		f.Add(choice, []byte("native\x00input\n"))
	}
	f.Fuzz(func(t *testing.T, choice uint8, payload []byte) {
		// This bounds only the companion echo workload; scope admission imposes no
		// stream extent or execution-count quota.
		payload = payload[:min(len(payload), 1024)]
		owner, stop := context.WithTimeout(t.Context(), processTestBackstop)
		defer stop()
		ctx, cancel := context.WithCancelCause(owner)
		defer cancel(nil)
		var scope, foreign process.ExecutionScope
		var stdout bytes.Buffer
		request := processRequest(t, "copy", process.Streams{Stdin: bytes.NewReader(payload), Stdout: &stdout, Stderr: io.Discard})
		ready := make(chan struct{})
		canceledWait := choice&16 != 0
		if canceledWait {
			request = processRequest(t, "wait", process.Streams{Stdin: bytes.NewReader(nil), Stdout: &readyWriter{ready: ready}, Stderr: io.Discard})
		}
		if choice&8 != 0 {
			cancel(core.ErrTemporalContract)
		}
		execution, err := scope.Begin(ctx, request)
		if choice&8 != 0 {
			if execution != nil || scope.ActiveCount() != 0 || !errors.Is(err, context.Canceled) || !errors.Is(err, core.ErrTemporalContract) {
				t.Fatalf("refused begin=%v/%v active=%d,want original cancellation causes and no child", execution, err, scope.ActiveCount())
			}
			return
		}
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			cancel(core.ErrTemporalContract)
			if scope.ActiveCount() != 0 {
				if _, err := scope.Wait(process.ExecutionWaitRequest{Execution: execution}); err != nil && !errors.Is(err, context.Canceled) {
					t.Error(err)
				}
			}
		}()
		if scope.ActiveCount() != 1 {
			t.Fatalf("active=%d,want one native child", scope.ActiveCount())
		}
		if _, err := foreign.Wait(process.ExecutionWaitRequest{Execution: execution}); !errors.Is(err, core.ErrProcessContract) || scope.ActiveCount() != 1 {
			t.Fatalf("foreign wait=%v active=%d,want refusal without forgetting owner's wait", err, scope.ActiveCount())
		}
		visits := 0
		var visitCause error
		if choice&1 != 0 {
			visitCause = errors.Join(io.ErrClosedPipe, os.ErrPermission)
		}
		visitCtx := ctx
		var visitCancel context.CancelCauseFunc
		if choice&4 != 0 {
			visitCtx, visitCancel = context.WithCancelCause(ctx)
			defer visitCancel(nil)
		}
		if choice&2 != 0 {
			visitCtx = nil
		}
		err = scope.VisitActive(visitCtx, process.ActiveExecutionRequest{Visit: func(active *process.Execution) error {
			visits++
			if active != execution {
				return core.ErrProcessContract
			}
			if visitCancel != nil {
				visitCancel(core.ErrTemporalContract)
			}
			return visitCause
		}})
		if choice&2 != 0 {
			if !errors.Is(err, core.ErrNilContext) || visits != 0 {
				t.Fatalf("nil-context visits=%d error=%v,want no callback and typed refusal", visits, err)
			}
		} else {
			if visits != 1 || (visitCause != nil && (!errors.Is(err, io.ErrClosedPipe) || !errors.Is(err, os.ErrPermission))) {
				t.Fatalf("native visits=%d error=%v,want exact borrowed child and original joined callback causes", visits, err)
			}
			if choice&4 != 0 {
				if !errors.Is(err, context.Canceled) || !errors.Is(err, core.ErrTemporalContract) {
					t.Fatalf("final visitor cancellation=%v,want both original context causes", err)
				}
			} else if !errors.Is(err, visitCause) {
				t.Fatalf("visitor cause=%v,want %v", err, visitCause)
			}
		}
		if canceledWait {
			select {
			case <-ready:
			case <-owner.Done():
				t.Fatal(owner.Err())
			}
			cancel(core.ErrTemporalContract)
		}
		var completionCause, nativeWaitCause error
		if choice&32 != 0 {
			completionCause = errors.Join(io.ErrClosedPipe, os.ErrPermission)
		}
		completions := 0
		result, err := scope.Wait(process.ExecutionWaitRequest{Execution: execution, Complete: func(result process.Result, nativeErr error) error {
			completions++
			nativeWaitCause = nativeErr
			return completionCause
		}})
		if completions != 1 || (completionCause != nil && !errors.Is(err, completionCause)) || (nativeWaitCause != nil && !errors.Is(err, nativeWaitCause)) {
			t.Fatalf("completion calls=%d error=%v, want exactly one cleanup and both original native causes", completions, err)
		}
		if result.Validate() != nil || scope.ActiveCount() != 0 {
			t.Fatalf("completion error=%v validation=%v active=%d,want completed native custody", err, result.Validate(), scope.ActiveCount())
		}
		if canceledWait {
			if !errors.Is(err, context.Canceled) || !errors.Is(err, core.ErrTemporalContract) || !errors.Is(err, core.ErrProcessWait) {
				t.Fatalf("native cancellation=%v,want wait and original context causes", err)
			}
		} else if (completionCause == nil && err != nil) || !bytes.Equal(stdout.Bytes(), payload) {
			t.Fatalf("echo=%q error=%v,want exact native payload %q", stdout.Bytes(), err, payload)
		}
		if _, err := scope.Wait(process.ExecutionWaitRequest{Execution: execution}); !errors.Is(err, core.ErrProcessContract) {
			t.Fatalf("duplicate wait=%v,want native single-wait refusal", err)
		}
	})
}

func TestExecutionScopeCanceledBeginConservesCustomCause(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancelCause(t.Context())
	cancel(core.ErrTemporalContract)
	var scope process.ExecutionScope
	request := processRequest(t, "silent", process.Streams{Stdin: bytes.NewReader(nil), Stdout: io.Discard, Stderr: io.Discard})
	execution, err := scope.Begin(ctx, request)
	if execution != nil || scope.ActiveCount() != 0 || !errors.Is(err, context.Canceled) || !errors.Is(err, core.ErrTemporalContract) {
		t.Fatalf("begin=%v/%v active=%d,want both original context causes and no native child", execution, err, scope.ActiveCount())
	}
}

func TestExecutionScopeVisitorFinalCancellationConservesAllCauses(t *testing.T) {
	t.Parallel()
	for _, callbackRefused := range []bool{false, true} {
		t.Run(fmt.Sprintf("callback_refused_%v", callbackRefused), func(t *testing.T) {
			t.Parallel()
			owner, cancelOwner := context.WithTimeout(t.Context(), processTestBackstop)
			defer cancelOwner()
			var scope process.ExecutionScope
			request := processRequest(t, "silent", process.Streams{Stdin: bytes.NewReader(nil), Stdout: io.Discard, Stderr: io.Discard})
			execution, err := scope.Begin(owner, request)
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				cancelOwner()
				if scope.ActiveCount() != 0 {
					if _, err := scope.Wait(process.ExecutionWaitRequest{Execution: execution}); err != nil && !errors.Is(err, context.Canceled) {
						t.Error(err)
					}
				}
			}()
			ctx, cancel := context.WithCancelCause(owner)
			defer cancel(nil)
			visits := 0
			err = scope.VisitActive(ctx, process.ActiveExecutionRequest{Visit: func(*process.Execution) error {
				visits++
				cancel(core.ErrTemporalContract)
				if callbackRefused {
					return errors.Join(io.ErrClosedPipe, os.ErrPermission)
				}
				return nil
			}})
			if visits != 1 || !errors.Is(err, context.Canceled) || !errors.Is(err, core.ErrTemporalContract) || (callbackRefused && (!errors.Is(err, io.ErrClosedPipe) || !errors.Is(err, os.ErrPermission))) || scope.ActiveCount() != 1 {
				t.Fatalf("visits=%d error=%v active=%d,want callback and both context causes without changing native custody", visits, err, scope.ActiveCount())
			}
			if result, err := scope.Wait(process.ExecutionWaitRequest{Execution: execution}); err != nil || result.Validate() != nil || scope.ActiveCount() != 0 {
				t.Fatalf("native wait=%v validation=%v active=%d,want completed independent owner", err, result.Validate(), scope.ActiveCount())
			}
		})
	}
}

func TestExecutionScopeNativeWaitConservesCustomCancellation(t *testing.T) {
	t.Parallel()
	owner, stop := context.WithTimeout(t.Context(), processTestBackstop)
	defer stop()
	ctx, cancel := context.WithCancelCause(owner)
	defer cancel(nil)
	ready := make(chan struct{})
	var scope process.ExecutionScope
	request := processRequest(t, "wait", process.Streams{Stdin: bytes.NewReader(nil), Stdout: &readyWriter{ready: ready}, Stderr: io.Discard})
	execution, err := scope.Begin(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		cancel(core.ErrTemporalContract)
		if scope.ActiveCount() != 0 {
			_, _ = scope.Wait(process.ExecutionWaitRequest{Execution: execution})
		}
	}()
	select {
	case <-ready:
	case <-owner.Done():
		t.Fatal(owner.Err())
	}
	cancel(core.ErrTemporalContract)
	result, err := scope.Wait(process.ExecutionWaitRequest{Execution: execution})
	var native process.Failure
	if !errors.Is(err, context.Canceled) || !errors.Is(err, core.ErrTemporalContract) || !errors.Is(err, core.ErrProcessWait) || !errors.As(err, &native) || native.Kind() != process.FailureKindWait || result.Validate() != nil || scope.ActiveCount() != 0 {
		t.Fatalf("native wait=%v validation=%v active=%d,want original custom cancellation, native wait cause and completed custody", err, result.Validate(), scope.ActiveCount())
	}
}

func TestExecutionScopeCompletionRemainsOwnedAndPreservesRefusal(t *testing.T) {
	t.Parallel()
	owner, cancel := context.WithTimeout(t.Context(), processTestBackstop)
	defer cancel()
	var scope process.ExecutionScope
	request := processRequest(t, "silent", process.Streams{Stdin: bytes.NewReader(nil), Stdout: io.Discard, Stderr: io.Discard})
	execution, err := scope.Begin(owner, request)
	if err != nil {
		t.Fatal(err)
	}
	ready, release := make(chan struct{}), make(chan struct{})
	var closeRelease sync.Once
	done := make(chan error, 1)
	var calls atomic.Int32
	complete := process.ExecutionWaitRequest{Execution: execution, Complete: func(result process.Result, waitErr error) error {
		calls.Add(1)
		if result.Validate() != nil || waitErr != nil {
			return errors.Join(core.ErrProcessContract, waitErr)
		}
		close(ready)
		<-release
		return errors.Join(io.ErrClosedPipe, os.ErrPermission)
	}}
	go func() { _, err := scope.Wait(complete); done <- err }()
	defer func() {
		closeRelease.Do(func() { close(release) })
		cancel()
		select {
		case <-done:
		case <-processTestDeadline(t, processTestBackstop):
			t.Error("native completion did not join")
		}
	}()
	select {
	case <-ready:
	case <-owner.Done():
		t.Fatal(owner.Err())
	}
	if scope.ActiveCount() != 1 {
		t.Fatalf("active=%d,want custody through pending completion", scope.ActiveCount())
	}
	if _, err := scope.Wait(complete); !errors.Is(err, core.ErrProcessContract) || calls.Load() != 1 || scope.ActiveCount() != 1 {
		t.Fatalf("duplicate wait=%v callbacks=%d active=%d,want refusal without another cleanup or lost custody", err, calls.Load(), scope.ActiveCount())
	}
	closeRelease.Do(func() { close(release) })
	select {
	case err := <-done:
		done <- err
		if !errors.Is(err, io.ErrClosedPipe) || !errors.Is(err, os.ErrPermission) || scope.ActiveCount() != 0 {
			t.Fatalf("completion=%v active=%d,want original joined cleanup causes and released custody", err, scope.ActiveCount())
		}
	case <-owner.Done():
		t.Fatal(owner.Err())
	}
}

func TestExecutionScopeCompletionPanicReleasesCompletedNativeCustody(t *testing.T) {
	t.Parallel()
	owner, cancel := context.WithTimeout(t.Context(), processTestBackstop)
	defer cancel()
	var scope process.ExecutionScope
	request := processRequest(t, "silent", process.Streams{Stdin: bytes.NewReader(nil), Stdout: io.Discard, Stderr: io.Discard})
	execution, err := scope.Begin(owner, request)
	if err != nil {
		t.Fatal(err)
	}
	var observed error
	func() {
		defer func() { observed, _ = recover().(error) }()
		_, _ = scope.Wait(process.ExecutionWaitRequest{Execution: execution, Complete: func(process.Result, error) error { panic(core.ErrTemporalContract) }})
	}()
	if !errors.Is(observed, core.ErrTemporalContract) || scope.ActiveCount() != 0 {
		t.Fatalf("completion panic=%v active=%d,want unchanged native panic and released completed custody", observed, scope.ActiveCount())
	}
}
