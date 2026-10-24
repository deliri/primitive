package gotoolchain_test

import (
	"context"
	"errors"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gotoolchain"
	"runtime"
	"testing"
)

func TestSSACalleeSourcePreservesNativeSignatureCause(t *testing.T) {
	t.Parallel()
	a := nativeSymbolSourceAnalysis(t, t.TempDir(), nativeFunctionArgumentPrelude+`func Entry(){accept("",named)}`)
	err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
		call := nativeFunctionArgumentCall(t, v.Package.Func("Entry"))
		call.Common().Value = call.Common().Args[0]
		got, err := v.CalleeSource(ctx, gotoolchain.SSACalleeSourceRequest{Call: call})
		var native *runtime.TypeAssertionError
		if got != (gotoolchain.SSAFunctionValueObservation{}) || !errors.Is(err, core.ErrGoToolchainOutput) || !errors.As(err, &native) {
			t.Fatalf("native signature cause got=%+v,%v,as=%v, want zero,output and native type assertion cause", got, err, native)
		}
		return nil
	}})
	if err != nil {
		t.Fatalf("WithSSAUnit got=%v, want nil", err)
	}
}
