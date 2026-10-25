package gotoolchain_test

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/deliri/primitive/v2026/gotoolchain"
	"golang.org/x/tools/go/ssa"
)

func TestSSAArgumentInvocationsPreservesConsumerPanic(t *testing.T) {
	t.Parallel()
	a := nativeSymbolSourceAnalysis(t, t.TempDir(), argumentInvocationSource(0, true))
	panicked := false
	func() {
		defer func() {
			if value := recover(); value != io.ErrClosedPipe {
				t.Fatalf("native consumer panic=%v want original cause", value)
			}
			panicked = true
		}()
		err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
			fn := v.Package.Func("Entry")
			v.ArgumentInvocations(ctx, gotoolchain.SSAArgumentInvocationsRequest{Function: fn, Argument: fn.Params[0]})(func(frame *gotoolchain.SSAArgumentInvocationFrame, queryErr error) bool {
				if queryErr != nil || frame == nil || frame.Kind() != gotoolchain.SSAInvocationKindRoot {
					t.Fatalf("panic precondition=%v,%v", frame, queryErr)
				}
				panic(io.ErrClosedPipe)
			})
			return nil
		}})
		if err != nil {
			t.Fatal(err)
		}
	}()
	if !panicked {
		t.Fatalf("consumer panic observed got=%t want true", panicked)
	}
}

func TestSSAArgumentInvocationBindingsStopsAndPreservesCause(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"stop after first binding", "canceled query"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			a := nativeSymbolSourceAnalysis(t, t.TempDir(), argumentInvocationDeclarations+`func Entry(a,b *Runner){func(x *Runner){use(x);use(a)}(a)}`)
			err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
				fn := v.Package.Func("Entry")
				if len(fn.AnonFuncs) != 1 || len(fn.AnonFuncs[0].FreeVars) != 1 || len(fn.AnonFuncs[0].Params) != 1 {
					t.Fatalf("native binding fixture got children=%v want one literal with one capture and one parameter", fn.AnonFuncs)
				}
				root, err := v.ArgumentInvocationRoot(ctx, gotoolchain.SSAArgumentInvocationsRequest{Function: fn, Argument: fn.Params[0]})
				if err != nil {
					return err
				}
				var selected *ssa.Call
				for instruction, queryErr := range v.Instructions(ctx, gotoolchain.SSAInstructionRequest{Function: fn, Extent: gotoolchain.SSAInstructionExtentFunction}) {
					if queryErr != nil {
						return queryErr
					}
					if call, ok := instruction.(*ssa.Call); ok {
						if selected != nil {
							t.Fatalf("native call got=%v after=%v want a single literal call", call, selected)
						}
						selected = call
					}
				}
				if selected == nil {
					t.Fatalf("native literal call got=%v want non-nil", selected)
				}
				if mode == "canceled query" {
					canceled, cancel := context.WithCancelCause(ctx)
					cancel(io.ErrClosedPipe)
					ctx = canceled
				}
				loans, refusals := 0, 0
				v.ArgumentInvocationBindings(ctx, gotoolchain.SSAArgumentInvocationRequest{Caller: root, Call: selected})(func(frame *gotoolchain.SSAArgumentInvocationFrame, queryErr error) bool {
					if queryErr != nil {
						if mode != "canceled query" || frame != nil || !errors.Is(queryErr, context.Canceled) || !errors.Is(queryErr, io.ErrClosedPipe) {
							t.Fatalf("binding cancellation=%v,%v", frame, queryErr)
						}
						refusals++
						return true
					}
					if frame == nil || frame.Binding().Function != fn.AnonFuncs[0] || frame.Caller() != root || frame.Call() != selected || loans != 0 {
						t.Fatalf("binding continued after consumer stop: %v after%d", frame, loans)
					}
					loans++
					return false
				})
				wantLoans, wantRefusals := 1, 0
				if mode == "canceled query" {
					wantLoans, wantRefusals = 0, 1
				}
				if loans != wantLoans || refusals != wantRefusals {
					t.Fatalf("binding loans/refusals=%d/%d want%d/%d", loans, refusals, wantLoans, wantRefusals)
				}
				return nil
			}})
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}
