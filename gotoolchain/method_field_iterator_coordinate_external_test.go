package gotoolchain_test

import (
	"context"
	"github.com/deliri/primitive/v2026/gotoolchain"
	"go/types"
	"testing"
)

func TestMethodFieldIteratorErrorPreservesYieldCoordinateAndControl(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name, sequence, callback, producer, consumer, result string
		want                                                 bool
	}{
		{name: "unrelated_method_before_captured_range", sequence: `iter.Seq2[error,error]`, callback: `func(error,error)bool`, producer: `refuse:=func(err error){yield(err,bad)};refuse(p.A.Other());rows:=func(yield func(error)bool){};for err:=range rows{refuse(err)}`, consumer: `for first,second:=range p.items(){_=second;if first!=nil{return first}};return nil`},
		{name: "captureless_sibling_call_cannot_be_callback_operand", sequence: `iter.Seq2[error,error]`, callback: `func(error,error)bool`, producer: `refuse:=func(err error){yield(bad,err)};refuse(p.A.Inspect());rows:=func(yield func(error)bool){};for err:=range rows{refuse(err)}`, consumer: `for first,second:=range p.items(){_=first;if second!=nil{return second}};return nil`, want: true},
		{name: "direct_error_before_range", producer: `yield(0,p.B.Inspect())`, consumer: `if err:=p.A.Inspect();err!=nil{return err};for _,err:=range p.items(){if err!=nil{return err}};return nil`, want: true},
		{name: "deferred_named_result_erasure", producer: `yield(0,p.A.Inspect())`, consumer: `defer func(){result=nil}();for _,err:=range p.items(){if err!=nil{return err}};return nil`, result: `(result error)`},
		{name: "single_error", sequence: `iter.Seq[error]`, callback: `func(error)bool`, producer: `yield(p.A.Inspect())`, consumer: `for err:=range p.items(){if err!=nil{return err}};return nil`, want: true},
		{name: "first_error_then_value", sequence: `iter.Seq2[error,int]`, callback: `func(error,int)bool`, producer: `yield(p.A.Inspect(),0)`, consumer: `for err:=range p.items(){if err!=nil{return err}};return nil`, want: true},
		{name: "first_of_two_errors", sequence: `iter.Seq2[error,error]`, callback: `func(error,error)bool`, producer: `yield(p.A.Inspect(),bad)`, consumer: `for first,second:=range p.items(){_=second;if first!=nil{return first}};return nil`, want: true},
		{name: "second_of_two_errors", sequence: `iter.Seq2[error,error]`, callback: `func(error,error)bool`, producer: `yield(bad,p.A.Inspect())`, consumer: `for first,second:=range p.items(){_=first;if second!=nil{return second}};return nil`, want: true},
		{name: "wrong_second_error", sequence: `iter.Seq2[error,error]`, callback: `func(error,error)bool`, producer: `yield(bad,p.A.Inspect())`, consumer: `for first,second:=range p.items(){_=second;if first!=nil{return first}};return nil`},
		{name: "wrong_first_error", sequence: `iter.Seq2[error,error]`, callback: `func(error,error)bool`, producer: `yield(p.A.Inspect(),bad)`, consumer: `for first,second:=range p.items(){_=first;if second!=nil{return second}};return nil`},
		{name: "consumer_returns_only_nil", producer: `yield(0,p.A.Inspect())`, consumer: `for _,err:=range p.items(){if err==nil{return err}};return nil`},
		{name: "consumer_breaks_on_error", producer: `yield(0,p.A.Inspect())`, consumer: `for _,err:=range p.items(){if err!=nil{break}};return nil`},
		{name: "consumer_joins_cause", producer: `yield(0,p.A.Inspect())`, consumer: `for _,err:=range p.items(){if err!=nil{return errors.Join(err)}};return nil`, want: true},
		{name: "producer_conditional_loss", producer: `err:=p.A.Inspect();if p.B.Count>0{yield(0,err)};return`, consumer: `for _,err:=range p.items(){if err!=nil{return err}};return nil`},
		{name: "replaced_callback_before_helper_invocation", producer: `refuse:=func(err error){yield(0,err)};yield=func(int,error)bool{return true};refuse(p.A.Inspect())`, consumer: `for _,err:=range p.items(){if err!=nil{return err}};return nil`},
		{name: "replaced_callback_before_capture", producer: `yield=func(int,error)bool{return true};refuse:=func(err error){yield(0,err)};refuse(p.A.Inspect())`, consumer: `for _,err:=range p.items(){if err!=nil{return err}};return nil`},
		{name: "helper_erases_cause", producer: `refuse:=func(err error){yield(0,bad)};err:=p.A.Inspect();refuse(err)`, consumer: `for _,err:=range p.items(){if err!=nil{return err}};return nil`},
		{name: "uninvoked_helper", producer: `refuse:=func(err error){yield(0,err)};_=refuse;_=p.A.Inspect()`, consumer: `for _,err:=range p.items(){if err!=nil{return err}};return nil`},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			sequence, callback := row.sequence, row.callback
			if sequence == "" {
				sequence = `iter.Seq2[int,error]`
				callback = `func(int,error)bool`
			}
			result := row.result
			if result == "" {
				result = "error"
			}
			source := `package sample
import("errors";"iter")
var _=errors.Join
type refusal uint8
const bad refusal=1
func(refusal)Error()string{return "refusal"}
type Child struct{Count int}
func(c Child)Inspect()error{if c.Count<0{return bad};return nil}
func(c Child)Other()error{return bad}
type Parent struct{A,B Child}
func(p Parent)items()` + sequence + `{return func(yield ` + callback + `){` + row.producer + `}}
func(p Parent)Check()` + result + `{` + row.consumer + `}`
			analysis := nativeSymbolSourceAnalysis(t, t.TempDir(), source)
			unit := analysis.Units[0]
			method := types.NewMethodSet(unit.Types.Scope().Lookup("Parent").Type()).Lookup(nil, "Check").Obj().(*types.Func)
			target := types.NewMethodSet(unit.Types.Scope().Lookup("Child").Type()).Lookup(nil, "Inspect").Obj().(*types.Func)
			if err := analysis.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: unit, SourceValues: true, Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
				got, err := view.MethodFieldErrorPropagation(ctx, gotoolchain.MethodFieldErrorPropagationRequest{Analysis: analysis, Method: method, Field: 0, Target: target})
				if err != nil || got.Validate() != nil || got.Source != gotoolchain.FunctionSourcePresent || got.Propagated != row.want {
					t.Fatalf("native yielded-coordinate/control=%+v/%v,want present/propagated=%t", got, err, row.want)
				}
				return nil
			}}); err != nil {
				t.Fatalf("native SSA scope=%v,want nil", err)
			}
		})
	}
}
