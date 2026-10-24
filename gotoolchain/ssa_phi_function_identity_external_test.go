package gotoolchain_test

import (
	"context"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gotoolchain"
	"golang.org/x/tools/go/ssa"
)

func TestSSAPhiInputsRefusesClonedFunctionIdentity(t *testing.T) {
	t.Parallel()
	a := nativeSymbolSourceAnalysis(t, t.TempDir(), nativePhiSource(3))
	err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
		fn := v.Package.Func("Entry")
		merge := nativeReturnedPhi(t, fn)
		request := gotoolchain.SSAPhiInputsRequest{Function: fn, Merge: merge}
		seen := 0
		for got, err := range v.PhiInputs(ctx, request) {
			if err != nil || got.Value != merge.Edges[seen] {
				t.Fatalf("function Phi precondition got=%+v,%v, want native function,nil", got, err)
			}
			seen++
		}
		if seen != 2 {
			t.Fatalf("function Phi precondition count got=%d, want2", seen)
		}
		function, ok := merge.Edges[1].(*ssa.Function)
		if !ok {
			t.Fatalf("function Phi fixture got=%T, want function reference", merge.Edges[1])
		}
		copied := *function
		merge.Edges[1] = &copied
		refused := 0
		for got, err := range v.PhiInputs(ctx, request) {
			if got != (gotoolchain.SSAPhiInputObservation{}) || !errors.Is(err, core.ErrGoToolchainOutput) {
				t.Fatalf("cloned function Phi got=%+v,%v, want zero,typed refusal", got, err)
			}
			refused++
		}
		if refused != 1 {
			t.Fatalf("cloned function refusal count got=%d, want1", refused)
		}
		return nil
	}})
	if err != nil {
		t.Fatalf("WithSSAUnit got=%v, want nil", err)
	}
}

func TestSSAPhiInputsAdmitsNativeInstantiatedFunction(t *testing.T) {
	t.Parallel()
	a := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;func left[T ~int](x T)T{return x};func right[T ~int](x T)T{return x};func Entry(flag bool)func(int)int{value:=left[int];if flag{value=right[int]};return value}`)
	err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
		fn := v.Package.Func("Entry")
		merge := nativeReturnedPhi(t, fn)
		seen := 0
		for got, err := range v.PhiInputs(ctx, gotoolchain.SSAPhiInputsRequest{Function: fn, Merge: merge}) {
			if err != nil || seen >= len(merge.Edges) || got.Value != merge.Edges[seen] {
				t.Fatalf("instantiated function input=%d got=%+v,%v, want native function,nil", seen, got, err)
			}
			function, ok := got.Value.(*ssa.Function)
			if !ok || function.Origin() == nil {
				t.Fatalf("instantiated function got=%T,%v, want actual native generic instance", got.Value, got.Value)
			}
			seen++
		}
		if seen != 2 {
			t.Fatalf("instantiated function count got=%d, want2", seen)
		}
		return nil
	}})
	if err != nil {
		t.Fatalf("WithSSAUnit got=%v, want nil", err)
	}
}
