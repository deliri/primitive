package gotoolchain_test

import (
	"context"
	"go/types"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/gotoolchain"
)

func TestMethodFieldIteratorErrorBindsNativeCallFormAndCause(t *testing.T) {
	t.Parallel()
	forms := []struct{ name, head, call, foreign string }{
		{name: "value", head: `func(p Parent)items()iter.Seq2[int,error]`, call: `p.items()`, foreign: `(Parent{}).items()`},
		{name: "pointer", head: `func(p *Parent)items()iter.Seq2[int,error]`, call: `(&p).items()`, foreign: `(&Parent{}).items()`},
		{name: "expression", head: `func(p Parent)items()iter.Seq2[int,error]`, call: `Parent.items(p)`, foreign: `Parent.items(Parent{})`},
		{name: "helper", head: `func items(p Parent)iter.Seq2[int,error]`, call: `items(p)`, foreign: `items(Parent{})`},
		{name: "conversion", head: `type Shared Parent;func(p Shared)items()iter.Seq2[int,error]`, call: `Shared(p).items()`, foreign: `Shared(Parent{}).items()`},
	}
	flows := []struct {
		name, producer, consumer string
		foreign, want            bool
	}{
		{name: "direct_cause", producer: `yield(0,p.A.Inspect())`, consumer: `for _,err:=range CALL{if err!=nil{return err}};return nil`, want: true},
		{name: "helper_cause", producer: `refuse:=func(err error){yield(0,err)};if err:=p.A.Inspect();err!=nil{refuse(err);return}`, consumer: `for _,err:=range CALL{if err!=nil{return err}};return nil`, want: true},
		{name: "joined_cause", producer: `yield(0,errors.Join(p.A.Inspect()))`, consumer: `for _,err:=range CALL{if err!=nil{return err}};return nil`, want: true},
		{name: "copied_cause", producer: `first:=p.A.Inspect();second:=first;yield(0,second)`, consumer: `for _,err:=range CALL{if err!=nil{return err}};return nil`, want: true},
		{name: "neighbor_field", producer: `yield(0,p.B.Inspect())`, consumer: `for _,err:=range CALL{if err!=nil{return err}};return nil`},
		{name: "producer_erases", producer: `_=p.A.Inspect();yield(0,nil)`, consumer: `for _,err:=range CALL{if err!=nil{return err}};return nil`},
		{name: "consumer_erases", producer: `yield(0,p.A.Inspect())`, consumer: `for range CALL{};return nil`},
		{name: "unconsumed", producer: `yield(0,p.A.Inspect())`, consumer: `_=CALL;return nil`},
		{name: "foreign_receiver", producer: `yield(0,p.A.Inspect())`, consumer: `for _,err:=range CALL{if err!=nil{return err}};return nil`, foreign: true},
		{name: "unrelated_return", producer: `yield(0,p.A.Inspect())`, consumer: `for range CALL{return bad};return nil`},
	}
	for _, form := range forms {
		for _, flow := range flows {
			t.Run(form.name+" "+flow.name, func(t *testing.T) {
				t.Parallel()
				call := form.call
				if flow.foreign {
					call = form.foreign
				}
				source := `package sample
import("errors";"iter")
var _=errors.Join
type refusal uint8
const bad refusal=1
func(refusal)Error()string{return "refusal"}
type Child struct{Count int}
func(c Child)Inspect()error{if c.Count<0{return bad};return nil}
type Parent struct{A,B Child}
` + form.head + `{return func(yield func(int,error)bool){` + flow.producer + `}}
func(p Parent)Check()error{` + strings.ReplaceAll(flow.consumer, "CALL", call) + `}`
				analysis := nativeSymbolSourceAnalysis(t, t.TempDir(), source)
				unit := analysis.Units[0]
				method := types.NewMethodSet(unit.Types.Scope().Lookup("Parent").Type()).Lookup(nil, "Check").Obj().(*types.Func)
				target := types.NewMethodSet(unit.Types.Scope().Lookup("Child").Type()).Lookup(nil, "Inspect").Obj().(*types.Func)
				if err := analysis.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: unit, SourceValues: true, Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
					got, err := view.MethodFieldErrorPropagation(ctx, gotoolchain.MethodFieldErrorPropagationRequest{Analysis: analysis, Method: method, Field: 0, Target: target})
					if err != nil || got.Validate() != nil || got.Source != gotoolchain.FunctionSourcePresent || got.Propagated != flow.want {
						t.Fatalf("native iterator call/cause=%+v/%v,want present/propagated=%t", got, err, flow.want)
					}
					return nil
				}}); err != nil {
					t.Fatalf("native SSA scope=%v,want nil", err)
				}
			})
		}
	}
}
