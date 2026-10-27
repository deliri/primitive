package gotoolchain_test

import (
	"context"
	"go/types"
	"testing"

	"github.com/deliri/primitive/v2026/gotoolchain"
)

func TestMethodFieldErrorPropagationRetainsComparedLoopGetter(t *testing.T) {
	t.Parallel()
	analysis := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;type refusal uint8;const bad refusal=1;func(refusal)Error()string{return "refusal"};type Kind uint8;const(One Kind=1;Two Kind=2);type Child struct{Label string;Count int;Kind Kind};func(v Child)Inspect()error{if v.Count<0{return bad};return nil};type Parent struct{A,B Child};func(v Parent)pick(kind Kind)Child{switch kind{case One:return v.A;case Two:return v.B;default:return Child{}}};func(v Parent)Check()error{for kind:=One;kind<=Two;kind++{child:=v.pick(kind);if child==(Child{}){continue};if child.Kind!=kind{return bad};if err:=child.Inspect();err!=nil{return err}};return nil}`)
	unit := analysis.Units[0]
	method := types.NewMethodSet(unit.Types.Scope().Lookup("Parent").Type()).Lookup(nil, "Check").Obj().(*types.Func)
	target := types.NewMethodSet(unit.Types.Scope().Lookup("Child").Type()).Lookup(nil, "Inspect").Obj().(*types.Func)
	err := analysis.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: unit, SourceValues: true, Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
		got, queryErr := view.MethodFieldErrorPropagation(ctx, gotoolchain.MethodFieldErrorPropagationRequest{Analysis: analysis, Method: method, Target: target})
		if queryErr != nil || got.Source != gotoolchain.FunctionSourcePresent || !got.Propagated {
			for instruction, err := range view.Instructions(ctx, gotoolchain.SSAInstructionRequest{Function: view.Program.FuncValue(method), Extent: gotoolchain.SSAInstructionExtentFunction}) {
				if err != nil {
					t.Fatalf("compared getter native frame = %v, want available loan", err)
				}
				t.Logf("native instruction %T: %s", instruction, instruction.String())
			}
			t.Fatalf("compared loop getter = %+v/%v, want native source and preserved selected child cause", got, queryErr)
		}
		return nil
	}})
	if err != nil {
		t.Fatalf("compared loop getter loan = %v, want nil", err)
	}
}

func TestMethodFieldErrorStoredGetterKeepsValueAndMutationDistinct(t *testing.T) {
	t.Parallel()
	for _, row := range [...]struct {
		name, body string
		want       bool
	}{
		{name: "whole replacement withdraws original getter", body: `child=v.fresh();return child.Inspect()`},
		{name: "field replacement withdraws original getter", body: `child.Count=1;return child.Inspect()`},
		{name: "neighbor getter cannot replace selected authority", body: `child=v.neighbor();return child.Inspect()`},
		{name: "escaped home lends no stable value", body: `replace(&child);return child.Inspect()`},
		{name: "loaded snapshot survives a later field write", body: `snapshot:=child;child.Count=1;return snapshot.Inspect()`, want: true},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			analysis := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;type refusal uint8;const bad refusal=1;func(refusal)Error()string{return "refusal"};type Child struct{Label string;Count int;Kind uint8};func(v Child)Inspect()error{if v.Count<0{return bad};return nil};type Parent struct{A,B Child};func(v Parent)pick()Child{return v.A};func(v Parent)neighbor()Child{return v.B};func(v Parent)fresh()Child{return Child{Count:1}};func replace(v *Child){*v=Child{Count:1}};func(v Parent)Check()error{child:=v.pick();if child==(Child{}){return nil};`+row.body+`}`)
			unit := analysis.Units[0]
			method := types.NewMethodSet(unit.Types.Scope().Lookup("Parent").Type()).Lookup(nil, "Check").Obj().(*types.Func)
			target := types.NewMethodSet(unit.Types.Scope().Lookup("Child").Type()).Lookup(nil, "Inspect").Obj().(*types.Func)
			err := analysis.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: unit, SourceValues: true, Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
				got, queryErr := view.MethodFieldErrorPropagation(ctx, gotoolchain.MethodFieldErrorPropagationRequest{Analysis: analysis, Method: method, Target: target})
				if queryErr != nil || got.Source != gotoolchain.FunctionSourcePresent || got.Propagated != row.want {
					t.Fatalf("stored getter = %+v/%v, want native source and original value propagation=%t", got, queryErr, row.want)
				}
				return nil
			}})
			if err != nil {
				t.Fatalf("stored getter mutation loan = %v, want nil", err)
			}
		})
	}
}

func BenchmarkMethodFieldErrorComparedGetter(b *testing.B) {
	b.ReportAllocs()
	analysis := nativeSymbolSourceAnalysis(b, b.TempDir(), `package sample;type refusal uint8;const bad refusal=1;func(refusal)Error()string{return "refusal"};type Kind uint8;const(One Kind=1;Two Kind=2);type Child struct{Label string;Count int;Kind Kind};func(v Child)Inspect()error{if v.Count<0{return bad};return nil};type Parent struct{A,B Child};func(v Parent)pick(kind Kind)Child{switch kind{case One:return v.A;case Two:return v.B;default:return Child{}}};func(v Parent)Check()error{for kind:=One;kind<=Two;kind++{child:=v.pick(kind);if child==(Child{}){continue};if child.Kind!=kind{return bad};if err:=child.Inspect();err!=nil{return err}};return nil}`)
	unit := analysis.Units[0]
	method := types.NewMethodSet(unit.Types.Scope().Lookup("Parent").Type()).Lookup(nil, "Check").Obj().(*types.Func)
	target := types.NewMethodSet(unit.Types.Scope().Lookup("Child").Type()).Lookup(nil, "Inspect").Obj().(*types.Func)
	err := analysis.WithSSAUnit(b.Context(), gotoolchain.SSAUnitRequest{Unit: unit, SourceValues: true, Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
		request := gotoolchain.MethodFieldErrorPropagationRequest{Analysis: analysis, Method: method, Target: target}
		for b.Loop() {
			got, queryErr := view.MethodFieldErrorPropagation(ctx, request)
			if queryErr != nil || !got.Propagated || got.Source != gotoolchain.FunctionSourcePresent {
				b.Fatalf("compared getter query=%+v/%v, want retained native field cause", got, queryErr)
			}
		}
		return nil
	}})
	if err != nil {
		b.Fatalf("compared getter benchmark loan=%v, want nil", err)
	}
}
