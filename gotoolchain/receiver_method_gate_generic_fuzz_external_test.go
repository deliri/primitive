package gotoolchain_test

import (
	"context"
	"go/types"
	"testing"

	"github.com/deliri/primitive/v2026/gotoolchain"
)

func FuzzReceiverMethodGateConservesConcreteGenericTarget(f *testing.F) {
	f.Add(false, false, false)
	f.Add(true, false, false)
	f.Add(false, true, false)
	f.Add(false, false, true)
	f.Fuzz(func(t *testing.T, wrongMethod, wrongValue, discard bool) {
		name := "Inspect"
		if wrongMethod {
			name = "Other"
		}
		selected := "checked:=value;"
		if wrongValue {
			selected = "var other V;checked:=other;"
		}
		check := `if err:=checked.` + name + `();err!=nil{return err};`
		if discard {
			check = `_=checked.` + name + `();`
		}
		source := `package sample;type refusal uint8;const bad refusal=1;func(refusal)Error()string{return "refusal"};type Record struct{Count int};func(Record)Inspect()error{return bad};func(Record)Other()error{return bad};func commit[V interface{Inspect()error;Other()error}](dst *V,value V)error{` + selected + check + `*dst=value;return nil};func(v *Record)Apply()error{return commit(v,Record{})}`
		analysis := nativeSymbolSourceAnalysis(t, t.TempDir(), source)
		unit := analysis.Units[0]
		record := unit.Types.Scope().Lookup("Record").Type()
		method := types.NewMethodSet(types.NewPointer(record)).Lookup(nil, "Apply").Obj().(*types.Func)
		target := types.NewMethodSet(record).Lookup(nil, "Inspect").Obj().(*types.Func)
		want := !wrongMethod && !wrongValue && !discard
		err := analysis.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: unit, SourceValues: true, Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
			got, err := view.ReceiverMethodGate(ctx, gotoolchain.ReceiverMethodGateRequest{Analysis: analysis, Method: method, Target: target})
			if err != nil || got.Validate() != nil || got.Source != gotoolchain.FunctionSourcePresent || got.Preserved != want {
				t.Fatalf("wrong method=%t wrong value=%t discarded=%t: native generic gate=%+v/%v,want preserved=%t", wrongMethod, wrongValue, discard, got, err, want)
			}
			return nil
		}})
		if err != nil {
			t.Fatal(err)
		}
	})
}
