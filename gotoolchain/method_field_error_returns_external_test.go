package gotoolchain_test

import (
	"context"
	"go/types"
	"testing"

	"github.com/deliri/primitive/v2026/gotoolchain"
)

func TestMethodFieldErrorPropagationRetainsReturnedFieldOrigin(t *testing.T) {
	t.Parallel()
	for _, row := range [...]struct {
		name, helper, body string
		want               bool
	}{
		{name: "method getter returns selected child", helper: `func(v Parent)pick()Child{return v.A}`, body: `return v.pick().Inspect()`, want: true},
		{name: "getter result copied locally", helper: `func(v Parent)pick()Child{return v.A}`, body: `child:=v.pick();return child.Inspect()`, want: true},
		{name: "record comparison before validating getter result", helper: `func(v Parent)pick()Child{return v.A}`, body: `child:=v.pick();if child==(Child{}){return nil};return child.Inspect()`, want: true},
		{name: "function getter binds parent argument", helper: `func pick(v Parent)Child{return v.A}`, body: `return pick(v).Inspect()`, want: true},
		{name: "method expression binds receiver argument", helper: `func(v Parent)pick()Child{return v.A}`, body: `return Parent.pick(v).Inspect()`, want: true},
		{name: "returned record retains constructed child slot", helper: `func(v Parent)pick()Bridge{return Bridge{Selected:v.A}}`, body: `return v.pick().Check()`, want: true},
		{name: "second tuple result is selected child", helper: `func(v Parent)pick()(Child,Child){return v.B,v.A}`, body: `_,child:=v.pick();return child.Inspect()`, want: true},
		{name: "neighbor getter cannot lend selected origin", helper: `func(v Parent)pick()Child{return v.B}`, body: `return v.pick().Inspect()`},
		{name: "unused selected getter cannot lend authority", helper: `func(v Parent)pick()Child{return v.A}`, body: `_ = v.pick();return v.B.Inspect()`},
		{name: "discarded getter validation cause cannot escape", helper: `func(v Parent)pick()Child{return v.A}`, body: `_ = v.pick().Inspect();return nil`},
		{name: "fresh returned child has no receiver origin", helper: `func(v Parent)pick()Child{return Child{}}`, body: `return v.pick().Inspect()`},
		{name: "first tuple result is neighboring child", helper: `func(v Parent)pick()(Child,Child){return v.B,v.A}`, body: `child,_:=v.pick();return child.Inspect()`},
		{name: "getter overwrites copied parent before return", helper: `func(v Parent)pick()Child{v.A=Child{};return v.A}`, body: `return v.pick().Inspect()`},
		{name: "caller overwrites getter result before validation", helper: `func(v Parent)pick()Child{return v.A}`, body: `child:=v.pick();child=Child{};return child.Inspect()`},
		{name: "getter recursion grants no return authority", helper: `func(v Parent)pick()Child{return v.pick()}`, body: `return v.pick().Inspect()`},
		{name: "opaque getter grants no source authority", helper: `var pick func(Parent)Child`, body: `return pick(v).Inspect()`},
		{name: "wrong projected slot cannot lend selected child", helper: `func(v Parent)pick()Bridge{return Bridge{Other:v.A,Selected:v.B}}`, body: `return v.pick().Check()`},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			analysis := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;type refusal uint8;func(refusal)Error()string{return "refusal"};type Child struct{Count int};func(v Child)Inspect()error{if v.Count<0{return refusal(1)};return nil};type Parent struct{A,B Child};type Bridge struct{Other,Selected Child};func(v Bridge)Check()error{return v.Selected.Inspect()};`+row.helper+`;func(v Parent)Check()error{`+row.body+`}`)
			unit := analysis.Units[0]
			method := types.NewMethodSet(unit.Types.Scope().Lookup("Parent").Type()).Lookup(nil, "Check").Obj().(*types.Func)
			target := types.NewMethodSet(unit.Types.Scope().Lookup("Child").Type()).Lookup(nil, "Inspect").Obj().(*types.Func)
			err := analysis.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: unit, SourceValues: true, Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
				got, queryErr := view.MethodFieldErrorPropagation(ctx, gotoolchain.MethodFieldErrorPropagationRequest{Analysis: analysis, Method: method, Target: target})
				if queryErr != nil || got.Source != gotoolchain.FunctionSourcePresent || got.Propagated != row.want {
					t.Fatalf("returned native field = %+v/%v, want source present and propagation=%t", got, queryErr, row.want)
				}
				return nil
			}})
			if err != nil {
				t.Fatalf("returned-field native loan = %v, want nil", err)
			}
		})
	}
}

func BenchmarkMethodFieldErrorReturnedRecord(b *testing.B) {
	b.ReportAllocs()
	analysis := nativeSymbolSourceAnalysis(b, b.TempDir(), `package sample;type Child struct{Count int};func(Child)Inspect()error{return nil};type Parent struct{A,B Child};type Bridge struct{Other,Selected Child};func(v Bridge)Check()error{return v.Selected.Inspect()};func(v Parent)project()Bridge{return Bridge{Selected:v.A}};func(v Parent)Check()error{return v.project().Check()}`)
	unit := analysis.Units[0]
	method := types.NewMethodSet(unit.Types.Scope().Lookup("Parent").Type()).Lookup(nil, "Check").Obj().(*types.Func)
	target := types.NewMethodSet(unit.Types.Scope().Lookup("Child").Type()).Lookup(nil, "Inspect").Obj().(*types.Func)
	err := analysis.WithSSAUnit(b.Context(), gotoolchain.SSAUnitRequest{Unit: unit, SourceValues: true, Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
		request := gotoolchain.MethodFieldErrorPropagationRequest{Analysis: analysis, Method: method, Target: target}
		for b.Loop() {
			got, queryErr := view.MethodFieldErrorPropagation(ctx, request)
			if queryErr != nil || !got.Propagated || got.Source != gotoolchain.FunctionSourcePresent {
				b.Fatalf("returned record query=%+v/%v, want retained field and native source", got, queryErr)
			}
		}
		return nil
	}})
	if err != nil {
		b.Fatalf("returned record benchmark loan=%v, want nil", err)
	}
}
