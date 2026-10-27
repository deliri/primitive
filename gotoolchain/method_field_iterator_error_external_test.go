package gotoolchain_test

import (
	"context"
	"go/types"
	"golang.org/x/tools/go/ssa"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/gotoolchain"
)

func TestMethodFieldErrorPropagationBorrowsConsumedIteratorCause(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name, iterator, consumer string
		want                     bool
	}{
		{name: "consumed selected cause", iterator: `yield(0,p.A.Inspect())`, consumer: `for _,err:=range p.items(){if err!=nil{return err}};return nil`, want: true},
		{name: "consumed helper forwarded cause", iterator: `refuse:=func(err error){yield(0,err)};if err:=p.A.Inspect();err!=nil{refuse(err);return}`, consumer: `for _,err:=range p.items(){if err!=nil{return err}};return nil`, want: true},
		{name: "consumed method expression", iterator: `yield(0,p.A.Inspect())`, consumer: `for _,err:=range Parent.items(p){if err!=nil{return err}};return nil`, want: true},
		{name: "consumed joined cause", iterator: `yield(0,errors.Join(p.A.Inspect()))`, consumer: `for _,err:=range p.items(){if err!=nil{return err}};return nil`, want: true},
		{name: "neighbor cannot lend selected cause", iterator: `yield(0,p.B.Inspect())`, consumer: `for _,err:=range p.items(){if err!=nil{return err}};return nil`},
		{name: "producer erases selected cause", iterator: `_=p.A.Inspect();yield(0,nil)`, consumer: `for _,err:=range p.items(){if err!=nil{return err}};return nil`},
		{name: "consumer erases yielded cause", iterator: `yield(0,p.A.Inspect())`, consumer: `for range p.items(){};return nil`},
		{name: "unconsumed iterator cannot lend cause", iterator: `yield(0,p.A.Inspect())`, consumer: `_=p.items();return nil`},
		{name: "fresh receiver cannot lend selected cause", iterator: `yield(0,p.A.Inspect())`, consumer: `for _,err:=range (Parent{}).items(){if err!=nil{return err}};return nil`},
		{name: "unrelated returned error cannot lend cause", iterator: `yield(0,p.A.Inspect())`, consumer: `for range p.items(){return bad};return nil`},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			source := `package sample
import("errors";"iter")
var _=errors.Join
type refusal uint8
const bad refusal=1
func(refusal)Error()string{return "refusal"}
type Child struct{Count int}
func(c Child)Inspect()error{if c.Count<0{return bad};return nil}
type Parent struct{A,B Child}
func(p Parent)items()iter.Seq2[int,error]{return func(yield func(int,error)bool){` + row.iterator + `}}
func(p Parent)Check()error{` + row.consumer + `}`
			analysis := nativeSymbolSourceAnalysis(t, t.TempDir(), source)
			unit := analysis.Units[0]
			method := types.NewMethodSet(unit.Types.Scope().Lookup("Parent").Type()).Lookup(nil, "Check").Obj().(*types.Func)
			target := types.NewMethodSet(unit.Types.Scope().Lookup("Child").Type()).Lookup(nil, "Inspect").Obj().(*types.Func)
			if err := analysis.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: unit, SourceValues: true, Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
				got, err := view.MethodFieldErrorPropagation(ctx, gotoolchain.MethodFieldErrorPropagationRequest{Analysis: analysis, Method: method, Field: 0, Target: target})
				if err != nil || got.Validate() != nil || got.Source != gotoolchain.FunctionSourcePresent || got.Propagated != row.want {
					parent := types.NewMethodSet(unit.Types.Scope().Lookup("Parent").Type()).Lookup(unit.Types, "items").Obj().(*types.Func)
					for _, root := range []*ssa.Function{view.Program.FuncValue(method), view.Program.FuncValue(parent)} {
						var dump strings.Builder
						if _, dumpErr := root.WriteTo(&dump); dumpErr != nil {
							t.Fatalf("native parent dump=%v,want nil", dumpErr)
						}
						for _, child := range root.AnonFuncs {
							if _, dumpErr := child.WriteTo(&dump); dumpErr != nil {
								t.Fatalf("native closure dump=%v,want nil", dumpErr)
							}
							for _, block := range child.Blocks {
								for _, instruction := range block.Instrs {
									if returned, ok := instruction.(*ssa.Return); ok {
										t.Logf("native closure=%s syntax=%T return position=%s value=%s", child.Name(), child.Syntax(), unit.Fset.Position(returned.Pos()), returned)
									}
								}
							}
						}
						t.Logf("native compiler frames:\n%s", dump.String())
					}
					t.Fatalf("native iterator selected-field cause=%+v/%v,want present/propagated=%t", got, err, row.want)
				}
				return nil
			}}); err != nil {
				t.Fatalf("native SSA scope=%v,want nil", err)
			}
		})
	}
}
