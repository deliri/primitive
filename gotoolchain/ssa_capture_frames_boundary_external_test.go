package gotoolchain_test

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gotoolchain"
	"golang.org/x/tools/go/ssa"
)

func TestSSACaptureFramesRefusesBrokenNativeCustody(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name   string
		damage uint8
		prefix int
		want   error
	}{
		{"missing root", 0, 0, core.ErrGoToolchainContract},
		{"missing value", 1, 0, core.ErrGoToolchainContract},
		{"typed nil parameter", 2, 0, core.ErrGoToolchainContract},
		{"foreign parameter", 3, 0, core.ErrGoToolchainContract},
		{"copied root", 4, 0, core.ErrGoToolchainContract},
		{"copied parameter", 5, 0, core.ErrGoToolchainContract},
		{"missing view", 6, 0, core.ErrGoToolchainContract},
		{"non-entry native value", 7, 0, core.ErrGoToolchainContract},
		{"missing capture target", 8, 1, core.ErrGoToolchainOutput},
		{"typed nil capture target", 9, 1, core.ErrGoToolchainOutput},
		{"foreign capture target", 10, 1, core.ErrGoToolchainOutput},
		{"copied capture target", 11, 1, core.ErrGoToolchainOutput},
		{"missing capture layout", 12, 1, core.ErrGoToolchainOutput},
		{"extra capture", 13, 1, core.ErrGoToolchainOutput},
		{"foreign trailing capture", 14, 1, core.ErrGoToolchainOutput},
		{"nil trailing capture", 15, 1, core.ErrGoToolchainOutput},
		{"typed nil trailing capture", 16, 1, core.ErrGoToolchainOutput},
		{"detached trailing capture", 17, 1, core.ErrGoToolchainOutput},
		{"missing local", 18, 1, core.ErrGoToolchainOutput},
		{"foreign local", 19, 1, core.ErrGoToolchainOutput},
		{"contradictory native block ordinal", 20, 1, core.ErrGoToolchainOutput},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			a := nativeSymbolSourceAnalysis(t, t.TempDir(), nativeClosureBindingSource)
			err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
				fn := v.Package.Func("Entry")
				closure := nativeSelectedClosure(t, fn)
				target := closure.Fn.(*ssa.Function)
				request := gotoolchain.SSACaptureFramesRequest{Function: fn, Argument: fn.Params[0]}
				view := v
				switch row.damage {
				case 0:
					request.Function = nil
				case 1:
					request.Argument = nil
				case 2:
					request.Argument = (*ssa.Parameter)(nil)
				case 3:
					request.Argument = v.Package.Func("Other").Params[0]
				case 4:
					copy := *fn
					request.Function = &copy
				case 5:
					copy := *fn.Params[0]
					request.Argument = &copy
				case 6:
					view = gotoolchain.SSAUnitView{}
				case 7:
					request.Argument = closure.Bindings[0]
				case 8:
					closure.Fn = nil
				case 9:
					closure.Fn = (*ssa.Function)(nil)
				case 10:
					closure.Fn = v.Package.Func("Other").AnonFuncs[0]
				case 11:
					copy := *target
					closure.Fn = &copy
				case 12:
					closure.Bindings = nil
				case 13:
					closure.Bindings = append(closure.Bindings, closure.Bindings[0])
				case 14:
					closure.Bindings[1] = nativeSelectedClosure(t, v.Package.Func("Other")).Bindings[1]
				case 15:
					closure.Bindings[1] = nil
				case 16:
					closure.Bindings[1] = (*ssa.Alloc)(nil)
				case 17:
					copy := *closure.Bindings[1].(*ssa.Alloc)
					closure.Bindings[1] = &copy
				case 18:
					target.FreeVars[1] = nil
				case 19:
					target.FreeVars[1] = v.Package.Func("Other").AnonFuncs[0].FreeVars[1]
				case 20:
					fn.Blocks[0].Index += 100
				}
				count, refusals := 0, 0
				for got, queryErr := range view.CaptureFrames(ctx, request) {
					if queryErr != nil {
						if got != (gotoolchain.SSAArgumentBindingObservation{}) || !errors.Is(queryErr, row.want) {
							t.Fatalf("capture custody %s got=%+v,%v, want zero and %v", row.name, got, queryErr, row.want)
						}
						refusals++
						continue
					}
					if got.Function != fn || got.Local != fn.Params[0] || count != 0 {
						t.Fatalf("capture custody %s borrowed invalid prefix=%+v after%d", row.name, got, count)
					}
					count++
				}
				if count != row.prefix || refusals != 1 {
					t.Fatalf("capture custody %s prefix/refusals=%d/%d, want%d/1", row.name, count, refusals, row.prefix)
				}
				return nil
			}})
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestSSACaptureFramesStopAndCancellationPrecedeMalformedCaptureTail(t *testing.T) {
	t.Parallel()
	a := nativeSymbolSourceAnalysis(t, t.TempDir(), nativeClosureBindingSource)
	err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
		fn := v.Package.Func("Entry")
		closure := nativeSelectedClosure(t, fn)
		closure.Bindings[1] = nativeSelectedClosure(t, v.Package.Func("Other")).Bindings[1]
		request := gotoolchain.SSACaptureFramesRequest{Function: fn, Argument: fn.Params[0]}
		for mode := 0; mode < 3; mode++ {
			query, cancel := context.WithCancelCause(ctx)
			if mode == 2 {
				cancel(io.ErrClosedPipe)
			}
			count, refusals := 0, 0
			for got, queryErr := range v.CaptureFrames(query, request) {
				if queryErr != nil {
					if got != (gotoolchain.SSAArgumentBindingObservation{}) || !errors.Is(queryErr, context.Canceled) || !errors.Is(queryErr, io.ErrClosedPipe) {
						t.Fatalf("capture cancellation mode%d got=%+v,%v, want zero and both native causes", mode, got, queryErr)
					}
					refusals++
					continue
				}
				if got.Function != fn || got.Local != fn.Params[0] || count != 0 {
					t.Fatalf("capture stop mode%d prefix=%+v after%d, want exact root", mode, got, count)
				}
				count++
				if mode == 0 {
					break
				}
				cancel(io.ErrClosedPipe)
			}
			cancel(nil)
			want, wantRefusals := 1, 0
			if mode == 1 {
				wantRefusals = 1
			}
			if mode == 2 {
				want, wantRefusals = 0, 1
			}
			if count != want || refusals != wantRefusals {
				t.Fatalf("capture stop mode%d prefix/refusals=%d/%d, want%d/%d", mode, count, refusals, want, wantRefusals)
			}
		}
		return nil
	}})
	if err != nil {
		t.Fatal(err)
	}
}

func TestSSACaptureFramesCallbackRejectsDeliveryAfterStop(t *testing.T) {
	t.Parallel()
	a := nativeSymbolSourceAnalysis(t, t.TempDir(), nativeClosureBindingSource)
	err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
		fn := v.Package.Func("Entry")
		calls := 0
		v.CaptureFrames(ctx, gotoolchain.SSACaptureFramesRequest{Function: fn, Argument: fn.Params[0]})(func(got gotoolchain.SSAArgumentBindingObservation, queryErr error) bool {
			if calls != 0 || queryErr != nil || got.Function != fn || got.Local != fn.Params[0] {
				t.Fatalf("capture stopped callback delivery=%+v,%v after%d, want only exact root", got, queryErr, calls)
			}
			calls++
			return false
		})
		if calls != 1 {
			t.Fatalf("capture stopped callback calls=%d, want1", calls)
		}
		return nil
	}})
	if err != nil {
		t.Fatal(err)
	}
}
