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
	f.Add(uint8(0), uint8(2), false, false)  // Keyed selected slot.
	f.Add(uint8(1), uint8(5), false, false)  // Unkeyed selected slot.
	f.Add(uint8(0), uint8(6), false, false)  // Constructed but unvalidated slot.
	f.Add(uint8(0), uint8(8), false, false)  // Copied selected field.
	f.Add(uint8(0), uint8(9), false, false)  // Copied neighboring field.
	f.Add(uint8(0), uint8(10), false, false) // Returned selected child.
	f.Add(uint8(0), uint8(11), false, false) // Returned neighboring child.
	f.Add(uint8(0), uint8(12), false, false) // Returned record projection.
	f.Add(uint8(0), uint8(13), false, false) // Returned neighboring projection.
	f.Fuzz(func(t *testing.T, field, usedField uint8, wrongTarget, discard bool) {
		projection := (usedField >> 1) % 7
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
		call := "v." + member + "." + target + "()"
		prefix, helper := "", ""
		switch projection {
		case 1:
			call = "(Bridge{Selected:v." + member + "}).Check()"
		case 2:
			call = "(Bridge{Child{},v." + member + "}).Check()"
		case 3:
			call = "(Bridge{Other:v." + member + "}).Check()"
		case 4:
			prefix = "copied:=v." + member + ";"
			call = "copied." + target + "()"
		case 5:
			helper = "func(v Parent)selected()Child{return v." + member + "};"
			call = "v.selected()." + target + "()"
		case 6:
			helper = "func(v Parent)selected()Bridge{return Bridge{Selected:v." + member + "}};"
			call = "v.selected().Check()"
		}
		body := "return " + call
		if discard {
			body = "_=" + call + ";return nil"
		}
		body = prefix + body
		analysis := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;type refusal uint8;const bad refusal=1;func(refusal)Error()string{return "refusal"};type Child struct{};func(Child)Inspect()error{return bad};func(Child)Other()error{return bad};type Parent struct{A,B Child};type Bridge struct{Other,Selected Child};func(v Bridge)Check()error{return v.Selected.`+target+`()};`+helper+`func(v Parent)Check()error{`+body+`}`)
		unit := analysis.Units[0]
		method := types.NewMethodSet(unit.Types.Scope().Lookup("Parent").Type()).Lookup(nil, "Check").Obj().(*types.Func)
		targetMethod := types.NewMethodSet(unit.Types.Scope().Lookup("Child").Type()).Lookup(nil, "Inspect").Obj().(*types.Func)
		want := field == usedField && !wrongTarget && !discard && projection != 3
		err := analysis.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: unit, SourceValues: true, Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
			got, err := view.MethodFieldErrorPropagation(ctx, gotoolchain.MethodFieldErrorPropagationRequest{Analysis: analysis, Method: method, Field: uint64(field), Target: targetMethod})
			if err != nil || got.Validate() != nil || got.Source != gotoolchain.FunctionSourcePresent || got.Propagated != want {
				t.Fatalf("field=%d used=%d projection=%d wrong target=%t discard=%t: native observation=%+v/%v,want propagated=%t", field, usedField, projection, wrongTarget, discard, got, err, want)
			}
			return nil
		}})
		if err != nil {
			t.Fatal(err)
		}
	})
}
