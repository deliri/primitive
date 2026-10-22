package gotoolchain_test

import (
	"context"
	"go/types"
	"testing"

	"github.com/deliri/primitive/v2026/gotoolchain"
)

func TestReceiverMethodGateUsesNativeCallbackAdaptersAndGenericArguments(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name, declarations, body string
		want                     bool
	}{
		{name: "callback consumes converted helper input", declarations: `func parse(input string)(Record,error){v:=Record{Count:len(input)};if err:=v.Inspect();err!=nil{return Record{},err};return v,nil};func commit(dst *Record,input []byte,parse func(string)(Record,error))error{v,err:=parse(string(input));if err!=nil{return err};*dst=v;return nil}`, body: `return commit(v,input,parse)`, want: true},
		{name: "generic helper binds actual record target", declarations: `func commit[V interface{Inspect()error}](dst *V,value V)error{if err:=value.Inspect();err!=nil{return err};*dst=value;return nil}`, body: `return commit(v,Record{})`, want: true},
		{name: "generic helper checks replacement before commit", declarations: `func commit[V interface{Inspect()error}](dst *V,value V)error{var zero V;value=zero;if err:=value.Inspect();err!=nil{return err};*dst=value;return nil}`, body: `return commit(v,Record{})`, want: true},
		{name: "generic helper cannot lend other method", declarations: `func commit[V interface{Other()error}](dst *V,value V)error{if err:=value.Other();err!=nil{return err};*dst=value;return nil}`, body: `return commit(v,Record{})`},
		{name: "generic helper cannot lend other argument", declarations: `func commit[V interface{Inspect()error}](dst *V,value V)error{var other V;if err:=other.Inspect();err!=nil{return err};*dst=value;return nil}`, body: `return commit(v,Record{})`},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			source := `package sample;type refusal uint8;const bad refusal=1;func(refusal)Error()string{return "refusal"};type Record struct{Count int};func(v Record)Inspect()error{if v.Count<0{return bad};return nil};func(Record)Other()error{return bad};` + "\n" + row.declarations + "\n" + `func(v *Record)Apply(input []byte)error{` + row.body + `}`
			analysis := nativeSymbolSourceAnalysis(t, t.TempDir(), source)
			unit := analysis.Units[0]
			record := unit.Types.Scope().Lookup("Record").Type()
			method := types.NewMethodSet(types.NewPointer(record)).Lookup(nil, "Apply").Obj().(*types.Func)
			target := types.NewMethodSet(record).Lookup(nil, "Inspect").Obj().(*types.Func)
			err := analysis.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: unit, SourceValues: true, Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
				got, err := view.ReceiverMethodGate(ctx, gotoolchain.ReceiverMethodGateRequest{Analysis: analysis, Method: method, Target: target})
				if err != nil || got.Validate() != nil || got.Source != gotoolchain.FunctionSourcePresent || got.Preserved != row.want {
					t.Fatalf("native adapter gate=%+v/%v,want present source and preserved=%t", got, err, row.want)
				}
				return nil
			}})
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}
