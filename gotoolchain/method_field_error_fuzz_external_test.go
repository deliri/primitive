package gotoolchain_test

import (
	"context"
	"go/types"
	"testing"

	"github.com/deliri/primitive/v2026/gotoolchain"
)

func FuzzMethodFieldErrorPropagationConservesTargetFieldAndCause(f *testing.F) {
	f.Add(uint8(0), uint8(0), false, false)
	f.Add(uint8(0), uint8(1), false, false)
	f.Add(uint8(1), uint8(1), true, false)
	f.Add(uint8(1), uint8(0), false, true)
	f.Fuzz(func(t *testing.T, field, usedField uint8, wrongTarget, discard bool) {
		field %= 2
		usedField %= 2
		member := "A"
		if usedField == 1 {
			member = "B"
		}
		target := "Inspect"
		if wrongTarget {
			target = "Other"
		}
		body := "return v." + member + "." + target + "()"
		if discard {
			body = "_=v." + member + "." + target + "();return nil"
		}
		analysis := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;type refusal uint8;const bad refusal=1;func(refusal)Error()string{return "refusal"};type Child struct{};func(Child)Inspect()error{return bad};func(Child)Other()error{return bad};type Parent struct{A,B Child};func(v Parent)Check()error{`+body+`}`)
		unit := analysis.Units[0]
		method := types.NewMethodSet(unit.Types.Scope().Lookup("Parent").Type()).Lookup(nil, "Check").Obj().(*types.Func)
		targetMethod := types.NewMethodSet(unit.Types.Scope().Lookup("Child").Type()).Lookup(nil, "Inspect").Obj().(*types.Func)
		want := field == usedField && !wrongTarget && !discard
		err := analysis.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: unit, SourceValues: true, Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
			got, err := view.MethodFieldErrorPropagation(ctx, gotoolchain.MethodFieldErrorPropagationRequest{Analysis: analysis, Method: method, Field: uint64(field), Target: targetMethod})
			if err != nil || got.Validate() != nil || got.Source != gotoolchain.FunctionSourcePresent || got.Propagated != want {
				t.Fatalf("field=%d used=%d wrong target=%t discard=%t: native observation=%+v/%v,want propagated=%t", field, usedField, wrongTarget, discard, got, err, want)
			}
			return nil
		}})
		if err != nil {
			t.Fatal(err)
		}
	})
}
