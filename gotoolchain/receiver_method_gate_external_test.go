package gotoolchain_test

import (
	"context"
	"go/types"
	"testing"

	"github.com/deliri/primitive/v2026/gotoolchain"
)

func TestReceiverMethodGateUsesCurrentStorageValueAndTarget(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name, declarations, body string
		want                     bool
	}{
		{name: "actual receiver method cause", body: `return v.Inspect()`, want: true},
		{name: "receiver is never inspected", body: `return nil`},
		{name: "other method cannot lend gate", body: `return v.Other()`},
		{name: "discarded method cause", body: `_=v.Inspect();return nil`},
		{name: "joined method cause", body: `return errors.Join(v.Inspect())`, want: true},
		{name: "copied pointer preserves storage", body: `copy:=v;return copy.Inspect()`, want: true},
		{name: "replaced pointer loses storage", body: `v=&Record{};return v.Inspect()`},
		{name: "different pointer cannot lend storage", body: `other:=&Record{};return other.Inspect()`},
		{name: "method expression preserves receiver", body: `return Record.Inspect(*v)`, want: true},
		{name: "checked staged value is committed", body: `stage:=Record{};if err:=stage.Inspect();err!=nil{return err};*v=stage;return nil`, want: true},
		{name: "unchecked staged value is committed", body: `stage:=Record{};*v=stage;return nil`},
		{name: "replaced staged value loses gate", body: `stage:=Record{};if err:=stage.Inspect();err!=nil{return err};stage=Record{Count:1};*v=stage;return nil`},
		{name: "mutated staged field loses gate", body: `stage:=Record{};if err:=stage.Inspect();err!=nil{return err};stage.Count++;*v=stage;return nil`},
		{name: "rechecked staged field regains gate", body: `stage:=Record{};if err:=stage.Inspect();err!=nil{return err};stage.Count++;if err:=stage.Inspect();err!=nil{return err};*v=stage;return nil`, want: true},
		{name: "discarded staged refusal loses gate", body: `stage:=Record{};_=stage.Inspect();*v=stage;return nil`},
		{name: "uninvoked closure lends no gate", body: `_=func()error{return v.Inspect()};return nil`},
		{name: "invoked closure preserves captured storage", body: `return func()error{return v.Inspect()}()`, want: true},
		{name: "invoked closure preserves receiver argument", body: `return func(x *Record)error{return x.Inspect()}(v)`, want: true},
		{name: "closure foreign argument cannot lend gate", body: `return func(x *Record)error{return x.Inspect()}(&Record{})`},
		{name: "named commit preserves checked value", declarations: `func commit(dst *Record,value Record)error{if err:=value.Inspect();err!=nil{return err};*dst=value;return nil}`, body: `return commit(v,Record{})`, want: true},
		{name: "named commit discards method cause", declarations: `func commit(dst *Record,value Record)error{_=value.Inspect();*dst=value;return nil}`, body: `return commit(v,Record{})`},
		{name: "named commit changes destination", declarations: `func commit(dst *Record,value Record)error{if err:=value.Inspect();err!=nil{return err};dst=&Record{};*dst=value;return nil}`, body: `return commit(v,Record{})`},
		{name: "callback input follows native byte slice type", declarations: `func parse(input []byte)(Record,error){value:=Record{};if err:=value.Inspect();err!=nil{return value,err};return value,nil};func commit(dst *Record,input []byte,parse func([]byte)(Record,error))error{value,err:=parse(input);if err!=nil{return err};*dst=value;return nil}`, body: `return commit(v,input,parse)`, want: true},
		{name: "unchecked callback result lends no gate", declarations: `func parse(input []byte)(Record,error){return Record{},nil};func commit(dst *Record,input []byte,parse func([]byte)(Record,error))error{value,err:=parse(input);if err!=nil{return err};*dst=value;return nil}`, body: `return commit(v,input,parse)`},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			source := `package sample;import "errors";var _=errors.Join;type refusal uint8;const bad refusal=1;func(refusal)Error()string{return "refusal"};type Record struct{Count int};func(v Record)Inspect()error{if v.Count<0{return bad};return nil};func(Record)Other()error{return bad};` + "\n" + row.declarations + "\n" + `func(v *Record)Apply(input []byte)error{` + row.body + `}`
			analysis := nativeSymbolSourceAnalysis(t, t.TempDir(), source)
			unit := analysis.Units[0]
			record := unit.Types.Scope().Lookup("Record").Type()
			method := types.NewMethodSet(types.NewPointer(record)).Lookup(nil, "Apply").Obj().(*types.Func)
			target := types.NewMethodSet(types.NewPointer(record)).Lookup(nil, "Inspect").Obj().(*types.Func)
			err := analysis.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: unit, SourceValues: true, Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
				got, err := view.ReceiverMethodGate(ctx, gotoolchain.ReceiverMethodGateRequest{Analysis: analysis, Method: method, Target: target})
				if err != nil || got.Validate() != nil || got.Source != gotoolchain.FunctionSourcePresent || got.Preserved != row.want {
					t.Fatalf("receiver method gate=%+v/%v,want present source and preserved=%t", got, err, row.want)
				}
				return nil
			}})
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}
