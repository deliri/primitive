package gotoolchain_test

import (
	"context"
	"errors"
	"io"
	"runtime"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gotoolchain"
	"golang.org/x/tools/go/ssa"
)

func withInvocationCalleeFixture(t testing.TB, directory string, use func(context.Context, gotoolchain.SSAUnitView, gotoolchain.SSAArgumentInvocationRequest) error) {
	t.Helper()
	analysis := nativeSymbolSourceAnalysis(t, directory, `package sample
type Runner struct{}
func use(*Runner){}
func helper(r *Runner, callback func()){callback()}
func other(r *Runner, callback func()){callback()}
func Entry(r *Runner){helper(r,func(){use(r)})}
`)
	err := analysis.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: analysis.Units[0], Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
		entry := view.Package.Func("Entry")
		root, err := view.ArgumentInvocationRoot(ctx, gotoolchain.SSAArgumentInvocationsRequest{Function: entry, Argument: entry.Params[0]})
		if err != nil {
			return err
		}
		var selected *gotoolchain.SSAArgumentInvocationFrame
		for frame, frameErr := range view.ArgumentInvocationBindings(ctx, gotoolchain.SSAArgumentInvocationRequest{Caller: root, Call: nativeCalleeCall(t, entry).(*ssa.Call)}) {
			if frameErr != nil {
				return frameErr
			}
			selected = frame
			break // Finish the borrowing iterator before a test mutates its call.
		}
		if selected == nil {
			t.Fatal("native callback fixture selected helper binding absent")
		}
		return use(ctx, view, gotoolchain.SSAArgumentInvocationRequest{Caller: selected, Call: nativeCalleeCall(t, selected.Binding().Function).(*ssa.Call)})
	}})
	if err != nil {
		t.Fatalf("native callback fixture error=%v, want nil", err)
	}
}

func TestSSAInvocationCalleeRejectsBrokenContextAndPreservesCauses(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name   string
		mutate func(*gotoolchain.SSAUnitView, *gotoolchain.SSAArgumentInvocationRequest)
		want   error
	}{
		{"missing caller", func(_ *gotoolchain.SSAUnitView, r *gotoolchain.SSAArgumentInvocationRequest) { r.Caller = nil }, core.ErrGoToolchainContract},
		{"missing call", func(_ *gotoolchain.SSAUnitView, r *gotoolchain.SSAArgumentInvocationRequest) { r.Call = nil }, core.ErrGoToolchainContract},
		{"missing view", func(v *gotoolchain.SSAUnitView, _ *gotoolchain.SSAArgumentInvocationRequest) {
			*v = gotoolchain.SSAUnitView{}
		}, core.ErrGoToolchainContract},
		{"copied call", func(_ *gotoolchain.SSAUnitView, r *gotoolchain.SSAArgumentInvocationRequest) {
			copy := *r.Call
			r.Call = &copy
		}, core.ErrGoToolchainContract},
		{"foreign call", func(v *gotoolchain.SSAUnitView, r *gotoolchain.SSAArgumentInvocationRequest) {
			r.Call = nativeCalleeCall(t, v.Package.Func("other")).(*ssa.Call)
		}, core.ErrGoToolchainContract},
		{"missing callable", func(_ *gotoolchain.SSAUnitView, r *gotoolchain.SSAArgumentInvocationRequest) { r.Call.Call.Value = nil }, core.ErrGoToolchainOutput},
		{"extra callback argument", func(_ *gotoolchain.SSAUnitView, r *gotoolchain.SSAArgumentInvocationRequest) {
			r.Call.Call.Args = append(r.Call.Call.Args, nil)
		}, core.ErrGoToolchainOutput},
		{"copied callback parameter", func(_ *gotoolchain.SSAUnitView, r *gotoolchain.SSAArgumentInvocationRequest) {
			copy := *r.Call.Call.Value.(*ssa.Parameter)
			r.Call.Call.Value = &copy
		}, core.ErrGoToolchainOutput},
		{"different parent callee", func(v *gotoolchain.SSAUnitView, r *gotoolchain.SSAArgumentInvocationRequest) {
			r.Caller.Call().Call.Value = v.Package.Func("other")
		}, core.ErrGoToolchainOutput},
		{"missing parent callback", func(_ *gotoolchain.SSAUnitView, r *gotoolchain.SSAArgumentInvocationRequest) {
			r.Caller.Call().Call.Args[1] = nil
		}, core.ErrGoToolchainOutput},
		{"truncated parent arguments", func(_ *gotoolchain.SSAUnitView, r *gotoolchain.SSAArgumentInvocationRequest) {
			r.Caller.Call().Call.Args = r.Caller.Call().Call.Args[:1]
		}, core.ErrGoToolchainOutput},
		{"native nonfunction signature cause", func(_ *gotoolchain.SSAUnitView, r *gotoolchain.SSAArgumentInvocationRequest) {
			r.Call.Call.Value = r.Caller.Binding().Function.Params[0]
		}, core.ErrGoToolchainOutput},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			withInvocationCalleeFixture(t, t.TempDir(), func(ctx context.Context, view gotoolchain.SSAUnitView, request gotoolchain.SSAArgumentInvocationRequest) error {
				healthy, err := view.InvocationCalleeSource(ctx, request)
				if err != nil || healthy.Function == nil {
					t.Fatalf("native callback before damage=%+v,%v, want actual source,nil", healthy, err)
				}
				row.mutate(&view, &request)
				observed, err := view.InvocationCalleeSource(ctx, request)
				if observed != (gotoolchain.SSAFunctionValueObservation{}) || !errors.Is(err, row.want) {
					t.Fatalf("damaged context source=%+v error=%v, want zero and %v", observed, err, row.want)
				}
				if row.name == "native nonfunction signature cause" {
					var native *runtime.TypeAssertionError
					if !errors.As(err, &native) {
						t.Fatalf("signature error=%v, want preserved native type assertion cause", err)
					}
				}
				return nil
			})
		})
	}
}

func TestSSAInvocationCalleeCancellationAndStoppedCapture(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"cancel", "stop", "panic"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			withInvocationCalleeFixture(t, t.TempDir(), func(ctx context.Context, view gotoolchain.SSAUnitView, request gotoolchain.SSAArgumentInvocationRequest) error {
				if mode == "cancel" {
					ctx, cancel := context.WithCancelCause(ctx)
					cancel(io.ErrClosedPipe)
					observed, err := view.InvocationCalleeSource(ctx, request)
					if observed != (gotoolchain.SSAFunctionValueObservation{}) || !errors.Is(err, context.Canceled) || !errors.Is(err, io.ErrClosedPipe) {
						t.Fatalf("canceled callback source=%+v,%v, want zero with cancellation and original cause", observed, err)
					}
					return nil
				}
				seen := 0
				defer func() {
					cause := recover()
					if seen != 1 || mode == "panic" && cause != io.ErrUnexpectedEOF || mode == "stop" && cause != nil {
						t.Fatalf("consumer mode=%s deliveries=%d panic=%v, want one exact prefix with original consumer outcome", mode, seen, cause)
					}
				}()
				view.ArgumentInvocationBindings(ctx, request)(func(frame *gotoolchain.SSAArgumentInvocationFrame, err error) bool {
					if err != nil || frame == nil {
						t.Fatalf("capture prefix frame=%v error=%v, want actual loan and nil", frame, err)
					}
					seen++
					request.Caller.Call().Call.Args[1] = nil
					if mode == "panic" {
						panic(io.ErrUnexpectedEOF)
					}
					return false
				})
				return nil
			})
		})
	}
}
