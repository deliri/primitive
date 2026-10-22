package gotoolchain_test

import (
	"context"
	"go/types"
	"testing"

	"github.com/deliri/primitive/v2026/gotoolchain"
)

func TestSSAArgumentCallPreservesNativeTargetAndOperand(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name, body string
		want       bool
	}{
		{"direct receiver", `r.Invoke()`, true},
		{"no call", ``, false},
		{"other receiver", `(&Runner{}).Invoke()`, false},
		{"other method", `r.Other()`, false},
		{"deferred receiver", `defer r.Invoke()`, true},
		{"spawn is not synchronous", `go r.Invoke()`, false},
		{"literal capture", `func(){r.Invoke()}()`, true},
		{"uninvoked literal", `_ = func(){r.Invoke()}`, false},
		{"literal parameter", `func(x *Runner){x.Invoke()}(r)`, true},
		{"literal foreign parameter", `func(x *Runner){x.Invoke()}(&Runner{})`, false},
		{"literal selected second parameter", `func(x,y *Runner){y.Invoke()}(&Runner{},r)`, true},
		{"literal wrong slot", `func(x,y *Runner){x.Invoke()}(&Runner{},r)`, false},
		{"bound native method", `f:=r.Invoke;f()`, true},
		{"replaced native method", `f:=r.Invoke;f=(&Runner{}).Invoke;f()`, false},
		{"copied literal", `f:=func(){r.Invoke()};g:=f;g()`, true},
		{"replaced literal", `f:=func(){r.Invoke()};f=func(){};f()`, false},
		{"nested literal", `func(){func(){r.Invoke()}()}()`, true},
		{"constant false arm", `if false {r.Invoke()}`, false},
		{"constant true arm", `if true {r.Invoke()}`, true},
		{"constant true else", `if true {} else {r.Invoke()}`, false},
		{"goto preserves later native call", `if flag {goto L};if false {return};L:r.Invoke()`, true},
		{"unknown branch lends possible call", `if flag {r.Invoke()}`, true},
		{"capture replaced before read", `func(){r=&Runner{};r.Invoke()}()`, false},
		{"capture read before later replacement", `func(){r.Invoke();r=&Runner{}}()`, true},
		{"capture replaced by parent", `f:=func(){r.Invoke()};r=&Runner{};f()`, false},
		{"deferred literal", `defer func(){r.Invoke()}()`, true},
		{"spawned literal", `go func(){r.Invoke()}()`, false},
		{"opaque named helper", `helper(r)`, false},
		{"method expression receiver", `(*Runner).Invoke(r)`, true},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			source := `package sample
type Runner struct{}
func(*Runner)Invoke(){}
func(*Runner)Other(){}
func helper(x *Runner){x.Invoke()}
var flag bool
func Entry(r *Runner){` + row.body + `}`
			analysis := nativeSymbolSourceAnalysis(t, t.TempDir(), source)
			unit := analysis.Units[0]
			object := unit.Types.Scope().Lookup("Entry").(*types.Func)
			target := types.NewMethodSet(types.NewPointer(unit.Types.Scope().Lookup("Runner").Type())).Lookup(nil, "Invoke").Obj().(*types.Func)
			err := analysis.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: unit, Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
				frame := view.Program.FuncValue(object)
				got, err := view.ArgumentCall(ctx, gotoolchain.SSAArgumentCallRequest{Function: frame, Argument: frame.Params[0], Target: target, Index: 0})
				if err != nil || got.Proven != row.want {
					t.Fatalf("ArgumentCall() = (%+v, %v), want proven=%t", got, err, row.want)
				}
				return nil
			}})
			if err != nil {
				t.Fatalf("WithSSAUnit(Entry) error = %v, want nil", err)
			}
		})
	}
}
