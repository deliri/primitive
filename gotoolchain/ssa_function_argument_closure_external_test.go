package gotoolchain_test

import (
	"context"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gotoolchain"
	"golang.org/x/tools/go/ssa"
)

func TestSSALastFunctionArgumentRefusesBrokenClosureLoan(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name string
		mode uint8
	}{{"missing captured function", 0}, {"typed nil captured function", 1}, {"nonfunction capture target", 2}, {"missing native capture binding", 3}, {"extra native capture binding", 4}, {"foreign lexical capture target", 5}, {"nil captured operand", 6}, {"foreign captured operand", 7}} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			a := nativeSymbolSourceAnalysis(t, t.TempDir(), nativeFunctionArgumentPrelude+`func Entry(n int){accept("",func(x int){_=n+x})};func Other(n int){accept("",func(x int){_=n+x})}`)
			err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
				fn := v.Package.Func("Entry")
				call := nativeFunctionArgumentCall(t, fn)
				closure, ok := call.Common().Args[1].(*ssa.MakeClosure)
				if !ok || len(closure.Bindings) != 1 {
					t.Fatalf("closure fixture got=%T, want one real native capture", call.Common().Args[1])
				}
				before, err := v.LastFunctionArgument(ctx, gotoolchain.SSAFunctionArgumentRequest{Call: call})
				if err != nil || before.Function != fn.AnonFuncs[0] || before.Value != closure {
					t.Fatalf("closure baseline got=%+v,%v, want exact source and closure,nil", before, err)
				}
				switch row.mode {
				case 0:
					closure.Fn = nil
				case 1:
					closure.Fn = (*ssa.Function)(nil)
				case 2:
					closure.Fn = fn.Params[0]
				case 3:
					closure.Bindings = nil
				case 4:
					closure.Bindings = append(closure.Bindings, closure.Bindings[0])
				case 5:
					closure.Fn = v.Package.Func("Other").AnonFuncs[0]
				case 6:
					closure.Bindings[0] = nil
				case 7:
					closure.Bindings[0] = v.Package.Func("Other").Params[0]
				}
				got, err := v.LastFunctionArgument(ctx, gotoolchain.SSAFunctionArgumentRequest{Call: call})
				if got != (gotoolchain.SSAFunctionValueObservation{}) || !errors.Is(err, core.ErrGoToolchainOutput) {
					t.Fatalf("closure loan got=%+v,%v, want zero,output refusal", got, err)
				}
				return nil
			}})
			if err != nil {
				t.Fatalf("WithSSAUnit got=%v, want nil", err)
			}
		})
	}
}
