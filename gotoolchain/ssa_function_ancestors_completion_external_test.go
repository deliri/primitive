package gotoolchain_test

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/deliri/primitive/v2026/gotoolchain"
	"golang.org/x/tools/go/ssa"
)

func TestSSAFunctionAncestorsCompletionCancellation(t *testing.T) {
	t.Parallel()
	a := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;func Entry(){}`)
	err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
		root := v.Package.Func("Entry")
		ctx, cancel := context.WithCancelCause(ctx)
		defer cancel(nil)
		loans, refusals := 0, 0
		for fn, queryErr := range v.FunctionAncestors(ctx, gotoolchain.SSAFunctionAncestorsRequest{Function: root}) {
			if queryErr != nil {
				if fn != nil || !errors.Is(queryErr, context.Canceled) || !errors.Is(queryErr, io.ErrClosedPipe) {
					t.Fatalf("final ancestor cancellation=%v,%v, want nil and both native causes", fn, queryErr)
				}
				refusals++
				continue
			}
			if fn != root || loans != 0 || refusals != 0 {
				t.Fatalf("final native root=%v after%d/%d loans/refusals", fn, loans, refusals)
			}
			loans++
			cancel(io.ErrClosedPipe)
		}
		if loans != 1 || refusals != 1 {
			t.Fatalf("completion loans/refusals=%d/%d, want1/1", loans, refusals)
		}
		return nil
	}})
	if err != nil {
		t.Fatal(err)
	}
}

func TestSSAFunctionAncestorsNativeGenericReceiverInstantiation(t *testing.T) {
	t.Parallel()
	source := functionAncestorsSource + `type GenericHolder[T any] struct{};func(GenericHolder[T]) Echo(value T)T{return value};func InstantiateMethod(){GenericHolder[int]{}.Echo(3)}`
	a := nativeSymbolSourceAnalysis(t, t.TempDir(), source)
	err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
		var instance *ssa.Function
		for _, block := range v.Package.Func("InstantiateMethod").Blocks {
			for _, instruction := range block.Instrs {
				if call, ok := instruction.(*ssa.Call); ok {
					instance = call.Common().StaticCallee()
				}
			}
		}
		if instance == nil || instance.Origin() == nil || instance.Signature.Recv() == nil || len(instance.TypeArgs()) != 1 {
			t.Fatal("fixture lost real native generic receiver instantiation")
		}
		loans := 0
		for fn, queryErr := range v.FunctionAncestors(ctx, gotoolchain.SSAFunctionAncestorsRequest{Function: instance}) {
			if fn != instance || queryErr != nil || loans != 0 {
				t.Fatalf("generic receiver ancestor=%v,%v after%d loans, want native instance once", fn, queryErr, loans)
			}
			loans++
		}
		if loans != 1 {
			t.Fatalf("generic receiver loans=%d, want1", loans)
		}
		return nil
	}})
	if err != nil {
		t.Fatal(err)
	}
}
