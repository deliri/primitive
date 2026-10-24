package gotoolchain_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gotoolchain"
	"golang.org/x/tools/go/ssa"
)

func nativeConvertedBindingMode(t testing.TB, directory string, mode uint8) {
	t.Helper()
	mode %= 9
	form := mode % 3
	damage := mode / 3
	body := `Callback(Child)(r)`
	if form == 1 {
		body = `Callback(func(local *Runner){})(r)`
	}
	if form == 2 {
		body = `Capture(func(){_=r.N})()`
	}
	a := nativeSymbolSourceAnalysis(t, directory, `package sample;type Runner struct{N int};type Callback func(*Runner);type Capture func();func Child(r *Runner){};func Entry(r *Runner){`+body+`}`)
	err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
		fn := v.Package.Func("Entry")
		call := nativeCalleeCall(t, fn).(*ssa.Call)
		target := v.Package.Func("Child")
		if form != 0 {
			if len(fn.AnonFuncs) != 1 {
				t.Fatalf("binding semantic got=%d children, want one", len(fn.AnonFuncs))
			}
			target = fn.AnonFuncs[0]
		}
		var local ssa.Value
		if form == 2 {
			if len(target.FreeVars) != 1 {
				t.Fatalf("binding semantic got=%d captures, want one", len(target.FreeVars))
			}
			local = target.FreeVars[0]
		} else {
			if len(target.Params) != 1 {
				t.Fatalf("binding semantic got=%d parameters, want one", len(target.Params))
			}
			local = target.Params[0]
		}
		request := gotoolchain.SSAArgumentBindingsRequest{Call: call, Argument: fn.Params[0]}
		count := 0
		for got, queryErr := range v.ArgumentBindings(ctx, request) {
			if queryErr != nil || got.Validate() != nil || got.Function != target || got.Local != local {
				t.Fatalf("binding semantic baseline mode=%d got=%+v,%v, want exact source local=%v,nil", mode, got, queryErr, local)
			}
			count++
		}
		if count != 1 {
			t.Fatalf("binding semantic baseline got=%d, want one fact", count)
		}
		if damage == 0 {
			return nil
		}
		want := error(core.ErrGoToolchainOutput)
		if damage == 1 {
			copy := *call
			request.Call = &copy
			if request.Call == call {
				t.Fatalf("binding call clone got original, want distinct pointer")
			}
			want = core.ErrGoToolchainContract
		} else {
			representation, ok := call.Common().Value.(*ssa.ChangeType)
			if !ok {
				t.Fatalf("binding semantic got=%T, want native representation", call.Common().Value)
			}
			original := representation.X
			switch value := original.(type) {
			case *ssa.Function:
				copy := *value
				representation.X = &copy
			case *ssa.MakeClosure:
				copy := *value
				representation.X = &copy
			default:
				t.Fatalf("binding inner representation got=%T, want native source value", original)
			}
			if representation.X == original {
				t.Fatalf("binding source clone got original, want distinct native pointer")
			}
		}
		count = 0
		for got, queryErr := range v.ArgumentBindings(ctx, request) {
			if got != (gotoolchain.SSAArgumentBindingObservation{}) || !errors.Is(queryErr, want) {
				t.Fatalf("binding semantic refusal mode=%d got=%+v,%v, want zero,%v", mode, got, queryErr, want)
			}
			count++
		}
		if count != 1 {
			t.Fatalf("binding semantic refusal got=%d, want one typed refusal and no admitted prefix", count)
		}
		return nil
	}})
	if err != nil {
		t.Fatalf("binding semantic unit got=%v, want nil", err)
	}
}
func TestSSAArgumentBindingsConvertedFiniteSemanticDomain(t *testing.T) {
	t.Parallel()
	for mode := uint8(0); mode < 9; mode++ {
		t.Run(fmt.Sprintf("source%d_damage%d", mode%3, mode/3), func(t *testing.T) { t.Parallel(); nativeConvertedBindingMode(t, t.TempDir(), mode) })
	}
}
func FuzzSSAArgumentBindingsConservesConvertedSourceLoan(f *testing.F) {
	f.Add(uint8(0))
	f.Fuzz(func(t *testing.T, mode uint8) { nativeConvertedBindingMode(t, t.TempDir(), mode) })
}
