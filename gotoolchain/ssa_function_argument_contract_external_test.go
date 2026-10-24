package gotoolchain_test

import (
	"context"
	"errors"
	"runtime"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gotoolchain"
	"golang.org/x/tools/go/ssa"
)

func TestSSALastFunctionArgumentSchemaLayerTriad(t *testing.T) {
	t.Parallel()
	a := nativeSymbolSourceAnalysis(t, t.TempDir(), nativeFunctionArgumentPrelude+`func Entry(){accept("",named)}`)
	err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
		fn := v.Package.Func("named")
		for _, row := range []struct {
			name string
			mode uint8
		}{{"source backed target", 0}, {"no target", 1}, {"missing native program", 2}, {"missing source package", 3}, {"missing native signature", 4}, {"missing native body", 5}, {"synthetic frame without source", 6}, {"missing argument value", 7}, {"typed nil argument value", 8}, {"nonfunction argument value", 9}, {"argument without target", 10}} {
			value := *fn
			got := gotoolchain.SSAFunctionArgumentObservation{Function: &value, Value: fn}
			var want error
			switch row.mode {
			case 1:
				got = gotoolchain.SSAFunctionArgumentObservation{}
			case 2:
				value.Prog = nil
				want = core.ErrGoToolchainOutput
			case 3:
				value.Pkg = nil
				want = core.ErrGoToolchainOutput
			case 4:
				value.Signature = nil
				want = core.ErrGoToolchainOutput
			case 5:
				value.Blocks = nil
				want = core.ErrGoToolchainOutput
			case 6:
				got.Function = v.Package.Func("init")
				want = core.ErrGoToolchainOutput
			case 7:
				got.Value = nil
				want = core.ErrGoToolchainOutput
			case 8:
				got.Value = (*ssa.Function)(nil)
				want = core.ErrGoToolchainOutput
			case 9:
				got.Value = fn.Params[0]
				want = core.ErrGoToolchainOutput
			case 10:
				got.Function = nil
				want = core.ErrGoToolchainOutput
			}
			if err := got.Validate(); !errors.Is(err, want) {
				t.Fatalf("%s schema got=%+v,%v, want %v", row.name, got, err, want)
			}
		}
		return nil
	}})
	if err != nil {
		t.Fatalf("WithSSAUnit got=%v, want nil", err)
	}
}

func TestSSALastFunctionArgumentPreservesNativeSignatureCause(t *testing.T) {
	t.Parallel()
	a := nativeSymbolSourceAnalysis(t, t.TempDir(), nativeFunctionArgumentPrelude+`func Entry(){accept("",named)}`)
	err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
		call := nativeFunctionArgumentCall(t, v.Package.Func("Entry"))
		call.Common().Value = call.Common().Args[0]
		got, err := v.LastFunctionArgument(ctx, gotoolchain.SSAFunctionArgumentRequest{Call: call})
		var native *runtime.TypeAssertionError
		if got != (gotoolchain.SSAFunctionArgumentObservation{}) || !errors.Is(err, core.ErrGoToolchainOutput) || !errors.As(err, &native) {
			t.Fatalf("native signature cause got=%+v,%v,as=%v, want zero,output and native type assertion cause", got, err, native)
		}
		return nil
	}})
	if err != nil {
		t.Fatalf("WithSSAUnit got=%v, want nil", err)
	}
}
