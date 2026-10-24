package gotoolchain_test

import (
	"context"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gotoolchain"
	"golang.org/x/tools/go/ssa"
)

func TestSSAArgumentBindingsConvertedClosureLayerTriad(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name string
		mode uint8
	}{{"actual captured runner", 0}, {"missing native capture", 1}, {"different caller runner", 2}} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			a := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;type Runner struct{N int};type Capture func();func Entry(r,other *Runner){Capture(func(){_=r.N})()}`)
			err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
				fn := v.Package.Func("Entry")
				call := nativeCalleeCall(t, fn).(*ssa.Call)
				argument := fn.Params[0]
				representation, ok := call.Common().Value.(*ssa.ChangeType)
				if !ok {
					t.Fatalf("converted triad got=%T, want native ChangeType", call.Common().Value)
				}
				closure, ok := representation.X.(*ssa.MakeClosure)
				if !ok || len(closure.Bindings) != 1 {
					t.Fatalf("converted triad got=%T, want one native capture", representation.X)
				}
				if row.mode == 1 {
					closure.Bindings = nil
				}
				if row.mode == 2 {
					argument = fn.Params[1]
				}
				count := 0
				for got, queryErr := range v.ArgumentBindings(ctx, gotoolchain.SSAArgumentBindingsRequest{Call: call, Argument: argument}) {
					if row.mode == 1 {
						if got != (gotoolchain.SSAArgumentBindingObservation{}) || !errors.Is(queryErr, core.ErrGoToolchainOutput) {
							t.Fatalf("converted refusal got=%+v,%v, want zero,output", got, queryErr)
						}
					} else if queryErr != nil || got.Function != fn.AnonFuncs[0] || got.Local != fn.AnonFuncs[0].FreeVars[0] {
						t.Fatalf("converted binding got=%+v,%v, want actual source local,nil", got, queryErr)
					}
					count++
				}
				want := 1
				if row.mode == 2 {
					want = 0
				}
				if count != want {
					t.Fatalf("converted triad got=%d facts, want %d", count, want)
				}
				return nil
			}})
			if err != nil {
				t.Fatalf("converted triad unit got=%v, want nil", err)
			}
		})
	}
}
