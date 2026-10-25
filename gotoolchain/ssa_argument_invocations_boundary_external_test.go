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

func TestSSAArgumentInvocationBindingsLayerTriad(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"selected argument", "different argument", "missing caller", "missing call", "missing view", "copied call", "foreign call", "missing callee", "extra argument"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			a := nativeSymbolSourceAnalysis(t, t.TempDir(), argumentInvocationDeclarations+`func Entry(a,b *Runner){helper(a);helper(b)};func Another(a *Runner){helper(a)}`)
			err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
				fn := v.Package.Func("Entry")
				root, err := v.ArgumentInvocationRoot(ctx, gotoolchain.SSAArgumentInvocationsRequest{Function: fn, Argument: fn.Params[0]})
				if err != nil {
					return err
				}
				var calls [2]*ssa.Call
				count := 0
				for instruction, queryErr := range v.Instructions(ctx, gotoolchain.SSAInstructionRequest{Function: fn, Extent: gotoolchain.SSAInstructionExtentFunction}) {
					if queryErr != nil {
						return queryErr
					}
					if call, ok := instruction.(*ssa.Call); ok {
						if count >= len(calls) {
							t.Fatalf("native call index got%d want below%d", count, len(calls))
						}
						calls[count] = call
						count++
					}
				}
				if count != 2 {
					t.Fatalf("fixture calls%d want2", count)
				}
				request := gotoolchain.SSAArgumentInvocationRequest{Caller: root, Call: calls[0]}
				loan := v
				wantErr := error(nil)
				switch mode {
				case "different argument":
					request.Call = calls[1]
				case "missing caller":
					request.Caller = nil
					wantErr = core.ErrGoToolchainContract
				case "missing call":
					request.Call = nil
					wantErr = core.ErrGoToolchainContract
				case "missing view":
					loan = gotoolchain.SSAUnitView{}
					wantErr = core.ErrGoToolchainContract
				case "copied call":
					copied := *calls[0]
					request.Call = &copied
					wantErr = core.ErrGoToolchainContract
				case "foreign call":
					for instruction, queryErr := range v.Instructions(ctx, gotoolchain.SSAInstructionRequest{Function: v.Package.Func("Another"), Extent: gotoolchain.SSAInstructionExtentFunction}) {
						if queryErr != nil {
							return queryErr
						}
						if call, ok := instruction.(*ssa.Call); ok {
							request.Call = call
							break
						}
					}
					if request.Call == calls[0] {
						t.Fatalf("foreign call got=%v want distinct call from Another", request.Call)
					}
					wantErr = core.ErrGoToolchainContract
				case "missing callee":
					calls[0].Call.Value = nil
					wantErr = core.ErrGoToolchainOutput
				case "extra argument":
					calls[0].Call.Args = append(calls[0].Call.Args, nil)
					wantErr = core.ErrGoToolchainOutput
				}
				observations, refusals := 0, 0
				for frame, queryErr := range loan.ArgumentInvocationBindings(ctx, request) {
					if queryErr != nil {
						if frame != nil || wantErr == nil || !errors.Is(queryErr, wantErr) {
							t.Fatalf("binding refusal=%v,%v want nil,%v", frame, queryErr, wantErr)
						}
						refusals++
						continue
					}
					if frame == nil || frame.Caller() != root || frame.Call() != calls[0] || frame.Kind() != gotoolchain.SSAInvocationKindArgument || frame.Binding().Function != v.Package.Func("helper") || frame.Binding().Local != v.Package.Func("helper").Params[0] {
						t.Fatalf("binding got=%+v want helper=%v caller=%v call=%v", frame, v.Package.Func("helper"), root, calls[0])
					}
					observations++
				}
				want, wantRefusals := 0, 0
				if mode == "selected argument" {
					want = 1
				}
				if wantErr != nil {
					wantRefusals = 1
				}
				if observations != want || refusals != wantRefusals {
					t.Fatalf("binding observations/refusals=%d/%d want%d/%d", observations, refusals, want, wantRefusals)
				}
				return nil
			}})
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestSSAArgumentInvocationsStopBeforeMalformedTail(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"consumer stops", "cancel after root"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			a := nativeSymbolSourceAnalysis(t, t.TempDir(), argumentInvocationSource(0, true))
			err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
				fn := v.Package.Func("Entry")
				if len(fn.Blocks) == 0 {
					t.Fatalf("native entry blocks got%d want at least1", len(fn.Blocks))
				}
				fn.Blocks[0].Instrs = append(fn.Blocks[0].Instrs, nil)
				ctx, cancel := context.WithCancelCause(ctx)
				defer cancel(nil)
				loans, refusals := 0, 0
				v.ArgumentInvocations(ctx, gotoolchain.SSAArgumentInvocationsRequest{Function: fn, Argument: fn.Params[0]})(func(frame *gotoolchain.SSAArgumentInvocationFrame, queryErr error) bool {
					if queryErr != nil {
						if mode != "cancel after root" || frame != nil || !errors.Is(queryErr, context.Canceled) || !errors.Is(queryErr, io.ErrClosedPipe) {
							t.Fatalf("delivery refusal=%v,%v want cancellation before bad tail", frame, queryErr)
						}
						refusals++
						return true
					}
					if frame == nil || frame.Kind() != gotoolchain.SSAInvocationKindRoot || loans != 0 {
						t.Fatalf("stopped prefix got frame=%+v after%d loans, want root once", frame, loans)
					}
					loans++
					if mode == "consumer stops" {
						return false
					}
					cancel(io.ErrClosedPipe)
					return true
				})
				wantRefusals := 0
				if mode == "cancel after root" {
					wantRefusals = 1
				}
				if loans != 1 || refusals != wantRefusals {
					t.Fatalf("prefix loans/refusals=%d/%d want1/%d", loans, refusals, wantRefusals)
				}
				return nil
			}})
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestSSAArgumentInvocationsNestedCallsKeepBorrowedAncestry(t *testing.T) {
	t.Parallel()
	a := nativeSymbolSourceAnalysis(t, t.TempDir(), argumentInvocationDeclarations+`func Entry(a,b *Runner){outer:=func(){inner:=func(){use(a)};inner()};outer();outer()}`)
	err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
		fn := v.Package.Func("Entry")
		if len(fn.AnonFuncs) != 1 || len(fn.AnonFuncs[0].AnonFuncs) != 1 {
			t.Fatalf("native nested children got=%v want one outer and one inner literal", fn.AnonFuncs)
		}
		var firstCall *ssa.Call
		loans := 0
		for frame, queryErr := range v.ArgumentInvocations(ctx, gotoolchain.SSAArgumentInvocationsRequest{Function: fn, Argument: fn.Params[0]}) {
			if queryErr != nil {
				return queryErr
			}
			if frame == nil {
				t.Fatalf("native frame got=%v at loan%d want non-nil", frame, loans)
			}
			switch loans {
			case 0:
				if frame.Kind() != gotoolchain.SSAInvocationKindRoot {
					t.Fatalf("first frame kind got=%v want root", frame.Kind())
				}
			case 1, 3:
				if frame.Binding().Function != fn.AnonFuncs[0] || frame.Caller().Binding().Function != fn {
					t.Fatalf("outer frame got=%+v want function=%v caller=%v", frame, fn.AnonFuncs[0], fn)
				}
				if loans == 1 {
					firstCall = frame.Call()
				} else if frame.Call() == firstCall {
					t.Fatalf("second outer call got=%v want distinct from first=%v", frame.Call(), firstCall)
				}
			case 2, 4:
				if frame.Binding().Function != fn.AnonFuncs[0].AnonFuncs[0] || frame.Caller().Binding().Function != fn.AnonFuncs[0] || frame.Caller().Caller().Binding().Function != fn {
					t.Fatalf("inner frame got=%+v want function=%v with two caller levels", frame, fn.AnonFuncs[0].AnonFuncs[0])
				}
			default:
				t.Fatalf("native loan index got%d want below5", loans)
			}
			loans++
		}
		if loans != 5 {
			t.Fatalf("nested invocation loans%d want5", loans)
		}
		return nil
	}})
	if err != nil {
		t.Fatal(err)
	}
}
