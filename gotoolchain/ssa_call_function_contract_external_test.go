package gotoolchain_test

import (
	"context"
	"errors"
	"go/token"
	"go/types"
	"runtime"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gotoolchain"
)

func TestSSACallFunctionSchemaLayerTriad(t *testing.T) {
	t.Parallel()
	signature := types.NewSignatureType(nil, nil, nil, types.NewTuple(), types.NewTuple(), false)
	for _, row := range []struct {
		name        string
		observation gotoolchain.SSACallFunctionObservation
		want        error
	}{
		{"named Go declaration", gotoolchain.SSACallFunctionObservation{Function: types.NewFunc(token.NoPos, types.NewPackage("example.com/sample", "sample"), "Function", signature)}, nil},
		{"no named declaration", gotoolchain.SSACallFunctionObservation{}, nil},
		{"declaration without package", gotoolchain.SSACallFunctionObservation{Function: types.NewFunc(token.NoPos, nil, "Function", signature)}, core.ErrGoToolchainOutput},
		{"declaration without signature", gotoolchain.SSACallFunctionObservation{Function: types.NewFunc(token.NoPos, types.NewPackage("example.com/sample", "sample"), "Function", nil)}, core.ErrGoToolchainOutput},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			if err := row.observation.Validate(); !errors.Is(err, row.want) {
				t.Fatalf("call schema got=%+v,%v, want %v", row.observation, err, row.want)
			}
		})
	}
}

func TestSSACallFunctionPreservesNativeSignatureCause(t *testing.T) {
	t.Parallel()
	a := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;import "time";func Entry(n int){_=time.Now()}`)
	err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
		fn := v.Package.Func("Entry")
		call := nativeCalleeCall(t, fn)
		call.Common().Value = fn.Params[0]
		got, queryErr := v.CallFunction(ctx, gotoolchain.SSACallFunctionRequest{Call: call})
		var cause *runtime.TypeAssertionError
		if got != (gotoolchain.SSACallFunctionObservation{}) || !errors.Is(queryErr, core.ErrGoToolchainOutput) || !errors.As(queryErr, &cause) {
			t.Fatalf("call signature cause got=%+v,%v,cause=%v, want zero,output and native type assertion cause", got, queryErr, cause)
		}
		return nil
	}})
	if err != nil {
		t.Fatalf("call cause unit got=%v, want nil", err)
	}
}
