package gotoolchain_test

import (
	"context"
	"go/types"
	"testing"

	"github.com/deliri/primitive/v2026/gotoolchain"
)

func FuzzReceiverMethodGateConservesLastCheckedRecord(f *testing.F) {
	f.Add(true, false, false, false)
	f.Add(false, false, false, false)
	f.Add(true, true, false, false)
	f.Add(true, false, true, false)
	f.Add(true, false, false, true)
	f.Fuzz(func(t *testing.T, recheck, lateWrite, escape, bypass bool) {
		body := `stage:=Record{};if err:=stage.Inspect();err!=nil{return err};stage.Count++;`
		if recheck {
			body += `if err:=stage.Inspect();err!=nil{return err};`
		}
		if lateWrite {
			body += `stage.Count++;`
		}
		if escape {
			body += `mutate(&stage);`
		}
		if bypass {
			body += `if alternate{stage.Count++};`
		}
		body += `*v=stage;return nil`
		source := `package sample;type refusal uint8;const bad refusal=1;func(refusal)Error()string{return "refusal"};type Record struct{Count int};func(v Record)Inspect()error{if v.Count<0{return bad};return nil};func mutate(v *Record){v.Count++};var alternate bool;func(v *Record)Apply()error{` + body + `}`
		analysis := nativeSymbolSourceAnalysis(t, t.TempDir(), source)
		unit := analysis.Units[0]
		record := unit.Types.Scope().Lookup("Record").Type()
		method := types.NewMethodSet(types.NewPointer(record)).Lookup(nil, "Apply").Obj().(*types.Func)
		target := types.NewMethodSet(record).Lookup(nil, "Inspect").Obj().(*types.Func)
		want := recheck && !lateWrite && !escape && !bypass
		err := analysis.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: unit, SourceValues: true, Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
			got, err := view.ReceiverMethodGate(ctx, gotoolchain.ReceiverMethodGateRequest{Analysis: analysis, Method: method, Target: target})
			if err != nil || got.Validate() != nil || got.Source != gotoolchain.FunctionSourcePresent || got.Preserved != want {
				t.Fatalf("recheck=%t late write=%t escape=%t bypass=%t native gate=%+v/%v,want preserved=%t", recheck, lateWrite, escape, bypass, got, err, want)
			}
			return nil
		}})
		if err != nil {
			t.Fatal(err)
		}
	})
}

func TestReceiverMethodGateRefusesWritesAfterLastCheck(t *testing.T) {
	t.Parallel()
	for _, row := range []struct{ name, late string }{
		{name: "direct late field write", late: `stage.Count++;`},
		{name: "conditional late field write", late: `if alternate{stage.Count++};`},
		{name: "whole record replacement", late: `stage=Record{Count:-1};`},
		{name: "opaque pointer mutation", late: `mutate(&stage);`},
		{name: "captured pointer mutation", late: `func(){stage.Count++}();`},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			source := `package sample;type refusal uint8;const bad refusal=1;func(refusal)Error()string{return "refusal"};type Record struct{Count int};func(v Record)Inspect()error{if v.Count<0{return bad};return nil};func mutate(v *Record){v.Count++};var alternate bool;func(v *Record)Apply()error{stage:=Record{};stage.Count++;if err:=stage.Inspect();err!=nil{return err};` + row.late + `*v=stage;return nil}`
			analysis := nativeSymbolSourceAnalysis(t, t.TempDir(), source)
			unit := analysis.Units[0]
			record := unit.Types.Scope().Lookup("Record").Type()
			method := types.NewMethodSet(types.NewPointer(record)).Lookup(nil, "Apply").Obj().(*types.Func)
			target := types.NewMethodSet(record).Lookup(nil, "Inspect").Obj().(*types.Func)
			err := analysis.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: unit, SourceValues: true, Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
				got, err := view.ReceiverMethodGate(ctx, gotoolchain.ReceiverMethodGateRequest{Analysis: analysis, Method: method, Target: target})
				if err != nil || got.Preserved || got.Source != gotoolchain.FunctionSourcePresent {
					t.Fatalf("late write gate=%+v/%v,want present source without stale authority", got, err)
				}
				return nil
			}})
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}
