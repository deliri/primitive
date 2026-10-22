package gotoolchain_test

import (
	"context"
	"go/types"
	"testing"

	"github.com/deliri/primitive/v2026/gotoolchain"
)

func TestMethodFieldErrorPropagationPreservesNativeTargetAndField(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name, declarations, fieldType, body string
		field                               uint64
		want                                bool
	}{
		{name: "direct selected child", body: `return v.A.Inspect()`, want: true},
		{name: "unread child", body: `return nil`},
		{name: "neighbor cannot lend target", body: `return v.B.Inspect()`},
		{name: "last field keeps its own slot", body: `return v.B.Inspect()`, field: 1, want: true},
		{name: "other method cannot lend target", body: `return v.A.Other()`},
		{name: "discarded target cause", body: `_=v.A.Inspect();return nil`},
		{name: "joined target cause", body: `return errors.Join(v.A.Inspect())`, want: true},
		{name: "joined unrelated cause", body: `_=v.A.Inspect();return errors.Join(bad)`},
		{name: "guarded target cause", body: `if err:=v.A.Inspect();err!=nil{return err};return nil`, want: true},
		{name: "shadowed parent loses authority", body: `{v:=Parent{};return v.A.Inspect()}`},
		{name: "method expression keeps child", body: `return Child.Inspect(v.A)`, want: true},
		{name: "whole parent helper preserves slot", declarations: `func inspect(x Parent)error{return x.A.Inspect()}`, body: `return inspect(v)`, want: true},
		{name: "child helper preserves selected value", declarations: `func inspect(x Child)error{return x.Inspect()}`, body: `return inspect(v.A)`, want: true},
		{name: "helper wrong argument slot", declarations: `func inspect(a,b Child)error{return a.Inspect()}`, body: `return inspect(Child{},v.A)`},
		{name: "helper nonfirst argument slot", declarations: `func inspect(a,b Child)error{return b.Inspect()}`, body: `return inspect(Child{},v.A)`, want: true},
		{name: "copied helper uses actual target", declarations: `func inspect(x Child)error{return x.Inspect()}`, body: `call:=inspect;return call(v.A)`, want: true},
		{name: "replaced helper loses prior target", declarations: `func inspect(x Child)error{return x.Inspect()};func ignore(Child)error{return nil}`, body: `call:=inspect;call=ignore;return call(v.A)`},
		{name: "nested helper returns selected child cause", declarations: `func inspect(x Child)error{return x.Inspect()};func forward(x Child)error{return inspect(x)}`, body: `return forward(v.A)`, want: true},
		{name: "nested helper keeps callee frame", declarations: `func inspect(a,b Child)error{return a.Inspect()};func forward(x Child)error{call:=inspect;return call(Child{},x)}`, body: `return forward(v.A)`},
		{name: "whole parent conversion preserves slot", declarations: `type Shared Parent;func inspect(x Shared)error{return x.A.Inspect()}`, body: `return inspect(Shared(v))`, want: true},
		{name: "fresh converted parent cannot lend field", declarations: `type Shared Parent;func inspect(x Shared)error{return x.A.Inspect()}`, body: `return inspect(Shared(Parent{}))`},
		{name: "uninvoked closure lends no target", body: `_=func()error{return v.A.Inspect()};return nil`},
		{name: "recursive helper invents no target", declarations: `func inspect(x Child,done bool)error{if done{return nil};return inspect(x,true)}`, body: `return inspect(v.A,false)`},
		{name: "slice item keeps owning field", fieldType: `[]Child`, body: `for _,item:=range v.A{if err:=item.Inspect();err!=nil{return err}};return nil`, want: true},
		{name: "neighbor slice cannot lend field", fieldType: `[]Child`, body: `for _,item:=range v.B{if err:=item.Inspect();err!=nil{return err}};return nil`},
		{name: "map value keeps owning field", fieldType: `map[int]Child`, body: `for _,item:=range v.A{if err:=item.Inspect();err!=nil{return err}};return nil`, want: true},
		{name: "array item keeps owning field", fieldType: `[2]Child`, body: `for _,item:=range v.A{if err:=item.Inspect();err!=nil{return err}};return nil`, want: true},
		{name: "pointer child keeps field", fieldType: `*Child`, body: `return v.A.Inspect()`, want: true},
		{name: "grandchild cannot replace selected receiver", declarations: `func inspect(x Child)error{return x.Next.Inspect()}`, body: `return inspect(v.A)`},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			fieldType := row.fieldType
			if fieldType == "" {
				fieldType = "Child"
			}
			source := `package sample
import "errors"
var _=errors.Join
type refusal uint8
const bad refusal=1
func(refusal)Error()string{return "refusal"}
type Child struct{Count int;Next *Child}
func(v Child)Inspect()error{if v.Count<0{return bad};return nil}
func(v Child)Other()error{return bad}
type Parent struct{A,B ` + fieldType + `}
` + row.declarations + `
func(v Parent)Check()error{` + row.body + `}`
			analysis := nativeSymbolSourceAnalysis(t, t.TempDir(), source)
			unit := analysis.Units[0]
			method := types.NewMethodSet(unit.Types.Scope().Lookup("Parent").Type()).Lookup(nil, "Check").Obj().(*types.Func)
			target := types.NewMethodSet(unit.Types.Scope().Lookup("Child").Type()).Lookup(nil, "Inspect").Obj().(*types.Func)
			err := analysis.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: unit, SourceValues: true, Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
				got, err := view.MethodFieldErrorPropagation(ctx, gotoolchain.MethodFieldErrorPropagationRequest{Analysis: analysis, Method: method, Field: row.field, Target: target})
				if err != nil || got.Validate() != nil || got.Source != gotoolchain.FunctionSourcePresent || got.Propagated != row.want {
					t.Fatalf("native field target observation=%+v/%v,want present source and propagated=%t", got, err, row.want)
				}
				return nil
			}})
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}
