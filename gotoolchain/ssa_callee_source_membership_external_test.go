package gotoolchain_test

import (
	"context"
	"errors"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gotoolchain"
	"golang.org/x/tools/go/ssa"
	"testing"
)

// Constant arguments and a declaration target carry no instruction-order edge.
// Call membership must therefore be authenticated independently of operands.
func TestSSACalleeSourceRefusesDetachedNamedCall(t *testing.T) {
	t.Parallel()
	a := nativeSymbolSourceAnalysis(t, t.TempDir(), nativeFunctionArgumentPrelude+`func Entry(){named(1)}`)
	err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
		call := nativeCalleeCall(t, v.Package.Func("Entry"))
		got, err := v.CalleeSource(ctx, gotoolchain.SSACalleeSourceRequest{Call: call})
		if err != nil || got.Function != v.Package.Func("named") || got.Value != call.Common().Value {
			t.Fatalf("named membership baseline got=%+v,%v, want exact source and value,nil", got, err)
		}
		copy := *call.(*ssa.Call)
		if &copy == call {
			t.Fatalf("detached call got original pointer, want nonvacuous clone")
		}
		got, err = v.CalleeSource(ctx, gotoolchain.SSACalleeSourceRequest{Call: &copy})
		if got != (gotoolchain.SSAFunctionValueObservation{}) || !errors.Is(err, core.ErrGoToolchainContract) || errors.Is(err, core.ErrGoToolchainOutput) {
			t.Fatalf("detached named call got=%+v,%v, want zero,contract only", got, err)
		}
		return nil
	}})
	if err != nil {
		t.Fatalf("named membership unit got=%v, want nil", err)
	}
}
