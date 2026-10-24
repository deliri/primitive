package gotoolchain_test

import (
	"context"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gotoolchain"
	"golang.org/x/tools/go/ssa"
)

func TestSSAArgumentBindingsKeepsNativeFunctionIdentity(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name, source string
		cloned       bool
	}{
		{"ordinary cloned target is refused", `package sample;func helper(a *int){};func Entry(a *int){helper(a)}`, true},
		{"native generic sibling argument is admitted", `package sample;func identity[T ~int](x T)T{return x};func helper(f func(int)int,a *int){};func Entry(a *int){helper(identity[int],a)}`, false},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			a := nativeSymbolSourceAnalysis(t, t.TempDir(), row.source)
			err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
				fn := v.Package.Func("Entry")
				var call *ssa.Call
				for _, block := range fn.Blocks {
					for _, instruction := range block.Instrs {
						if candidate, ok := instruction.(*ssa.Call); ok {
							call = candidate
						}
					}
				}
				if call == nil {
					t.Fatalf("binding fixture got=%v, want actual helper call", call)
				}
				if row.cloned {
					target := call.Common().StaticCallee()
					copied := *target
					call.Common().Value = &copied
				}
				seen, refused := 0, 0
				for got, err := range v.ArgumentBindings(ctx, gotoolchain.SSAArgumentBindingsRequest{Call: call, Argument: fn.Params[0]}) {
					if row.cloned {
						if got != (gotoolchain.SSAArgumentBindingObservation{}) || !errors.Is(err, core.ErrGoToolchainOutput) {
							t.Fatalf("cloned target got=%+v,%v, want zero,typed refusal", got, err)
						}
						refused++
						continue
					}
					helper := v.Package.Func("helper")
					if err != nil || got.Function != helper || got.Local != helper.Params[1] {
						t.Fatalf("generic sibling binding got=%+v,%v, want actual pointer parameter,nil", got, err)
					}
					seen++
				}
				wantSeen, wantRefused := 1, 0
				if row.cloned {
					wantSeen, wantRefused = 0, 1
				}
				if seen != wantSeen || refused != wantRefused {
					t.Fatalf("function identity binding got=%d,%d, want%d,%d", seen, refused, wantSeen, wantRefused)
				}
				return nil
			}})
			if err != nil {
				t.Fatalf("WithSSAUnit got=%v, want nil", err)
			}
		})
	}
}
