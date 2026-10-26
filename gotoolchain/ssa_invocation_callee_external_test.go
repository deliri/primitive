package gotoolchain_test

import (
	"context"
	"testing"

	"github.com/deliri/primitive/v2026/gotoolchain"
	"golang.org/x/tools/go/ssa"
)

func TestSSAInvocationCalleePreservesActualCallbackArgument(t *testing.T) {
	t.Parallel()
	analysis := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample
type Runner struct{}
func use(*Runner){}
func helper(r *Runner, callback func()){callback()}
func Entry(r *Runner){helper(r,func(){use(r)});helper(r,func(){use(r)})}
`)
	err := analysis.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: analysis.Units[0], Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
		entry := view.Package.Func("Entry")
		root, err := view.ArgumentInvocationRoot(ctx, gotoolchain.SSAArgumentInvocationsRequest{Function: entry, Argument: entry.Params[0]})
		if err != nil {
			return err
		}
		seen := 0
		for instruction, queryErr := range view.Instructions(ctx, gotoolchain.SSAInstructionRequest{Function: entry, Extent: gotoolchain.SSAInstructionExtentFunction}) {
			if queryErr != nil {
				return queryErr
			}
			call, ok := instruction.(*ssa.Call)
			if !ok {
				continue
			}
			for frame, bindingErr := range view.ArgumentInvocationBindings(ctx, gotoolchain.SSAArgumentInvocationRequest{Caller: root, Call: call}) {
				if bindingErr != nil {
					return bindingErr
				}
				callback := nativeCalleeCall(t, frame.Binding().Function).(*ssa.Call)
				ordinary, ordinaryErr := view.CalleeSource(ctx, gotoolchain.SSACalleeSourceRequest{Call: callback})
				if ordinaryErr != nil || ordinary != (gotoolchain.SSAFunctionValueObservation{}) {
					t.Fatalf("context-free callback source=%+v,%v, want absent,nil", ordinary, ordinaryErr)
				}
				observed, observeErr := view.InvocationCalleeSource(ctx, gotoolchain.SSAArgumentInvocationRequest{Caller: frame, Call: callback})
				if seen >= len(entry.AnonFuncs) || observeErr != nil || observed.Function != entry.AnonFuncs[seen] || observed.Value != call.Common().Args[1] {
					t.Fatalf("invocation=%d callback source=%+v,%v, want this call's native argument and distinct source literal", seen, observed, observeErr)
				}
				seen++
			}
		}
		if seen != 2 {
			t.Fatalf("callback invocation observations=%d, want two distinct actual caller contexts", seen)
		}
		return nil
	}})
	if err != nil {
		t.Fatalf("native invocation source error=%v, want nil", err)
	}
}

func TestSSAInvocationBindingsRetainCallbackCaptureCustody(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name, capture string
		want          int
	}{
		{"selected capture", "a", 1},
		{"different capture", "b", 0},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			analysis := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample
type Runner struct{}
func use(*Runner){}
func helper(r *Runner, callback func()){callback()}
func Entry(a,b *Runner){helper(a,func(){use(`+row.capture+`)})}
`)
			err := analysis.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: analysis.Units[0], Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
				entry := view.Package.Func("Entry")
				root, err := view.ArgumentInvocationRoot(ctx, gotoolchain.SSAArgumentInvocationsRequest{Function: entry, Argument: entry.Params[0]})
				if err != nil {
					return err
				}
				call := nativeCalleeCall(t, entry).(*ssa.Call)
				outer := 0
				for frame, frameErr := range view.ArgumentInvocationBindings(ctx, gotoolchain.SSAArgumentInvocationRequest{Caller: root, Call: call}) {
					if frameErr != nil {
						return frameErr
					}
					outer++
					callback := nativeCalleeCall(t, frame.Binding().Function).(*ssa.Call)
					captures := 0
					for capture, captureErr := range view.ArgumentInvocationBindings(ctx, gotoolchain.SSAArgumentInvocationRequest{Caller: frame, Call: callback}) {
						if captureErr != nil {
							return captureErr
						}
						if capture.Call() != callback || capture.Caller() != frame || capture.Binding().Function != entry.AnonFuncs[0] || capture.Binding().Local != entry.AnonFuncs[0].FreeVars[0] {
							t.Fatalf("capture binding=%+v, want exact native literal, free variable, call and caller", capture.Binding())
						}
						captures++
					}
					if captures != row.want {
						t.Fatalf("capture source=%s bindings=%d, want %d", row.capture, captures, row.want)
					}
				}
				if outer != 1 {
					t.Fatalf("outer helper invocations=%d, want exactly one selected native argument", outer)
				}
				return nil
			}})
			if err != nil {
				t.Fatalf("capture custody error=%v, want nil", err)
			}
		})
	}
}
