package gotoolchain_test

import (
	"context"
	"go/types"
	"testing"

	"github.com/deliri/primitive/v2026/gotoolchain"
)

func TestMethodFieldErrorPropagationUsesPointerAndGenericOrigins(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name, source string
		pointer      bool
	}{
		{name: "pointer parent receiver", pointer: true, source: `package sample;type Child struct{};func(Child)Inspect()error{return nil};type Parent struct{A Child};func(v *Parent)Check()error{return v.A.Inspect()}`},
		{name: "pointer child receiver", source: `package sample;type Child struct{};func(*Child)Inspect()error{return nil};type Parent struct{A Child};func(v Parent)Check()error{return v.A.Inspect()}`},
		{name: "generic parent and child origins", source: `package sample;type Child[T ~int]struct{Count T};func(Child[T])Inspect()error{return nil};type Parent[T ~int]struct{A Child[T]};func(v Parent[T])Check()error{return v.A.Inspect()};func Use(){_=Parent[int]{}.Check()}`},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			analysis := nativeSymbolSourceAnalysis(t, t.TempDir(), row.source)
			unit := analysis.Units[0]
			parent := unit.Types.Scope().Lookup("Parent").Type()
			if row.pointer {
				parent = types.NewPointer(parent)
			}
			method := types.NewMethodSet(parent).Lookup(nil, "Check").Obj().(*types.Func)
			target := types.NewMethodSet(types.NewPointer(unit.Types.Scope().Lookup("Child").Type())).Lookup(nil, "Inspect").Obj().(*types.Func)
			for _, selection := range unit.TypesInfo.Selections {
				if actual, ok := selection.Obj().(*types.Func); ok && actual.Name() == "Check" {
					method = actual
				}
			}
			err := analysis.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: unit, SourceValues: true, Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
				got, err := view.MethodFieldErrorPropagation(ctx, gotoolchain.MethodFieldErrorPropagationRequest{Analysis: analysis, Method: method, Target: target})
				if err != nil || !got.Propagated || got.Source != gotoolchain.FunctionSourcePresent {
					t.Fatalf("native method origins=%s/%s observation=%+v/%v,want selected child cause", method.FullName(), target.FullName(), got, err)
				}
				return nil
			}})
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}
