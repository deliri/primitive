package gotoolchain_test

import (
	"context"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gotoolchain"
	"golang.org/x/tools/go/ssa"
)

func TestSSACallFunctionAuthenticatesInitializerClosure(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name   string
		damage uint8
	}{
		{"native initializer closure", 0},
		{"copied closure", 1},
		{"detached closure", 2},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			a := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;var Value=func()int{return 7}()`)
			err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], SourceValues: true, Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
				initial := v.Package.Func("init")
				var call *ssa.Call
				for _, block := range initial.Blocks {
					for _, instruction := range block.Instrs {
						if candidate, ok := instruction.(*ssa.Call); ok {
							call = candidate
						}
					}
				}
				if call == nil {
					t.Fatal("initializer call=nil, want actual closure call")
				}
				closure, ok := call.Common().Value.(*ssa.Function)
				if !ok || closure.Parent() != initial {
					t.Fatalf("initializer callee=%v, want native lexical child", call.Common().Value)
				}
				switch row.damage {
				case 1:
					copy := *closure
					call.Common().Value = &copy
				case 2:
					initial.AnonFuncs = nil
				}
				got, queryErr := v.CallFunction(ctx, gotoolchain.SSACallFunctionRequest{Call: call})
				if got != (gotoolchain.SSACallFunctionObservation{}) {
					t.Fatalf("literal identity=%+v, want no named function", got)
				}
				if row.damage == 0 {
					if queryErr != nil {
						t.Fatalf("native initializer error=%v, want nil", queryErr)
					}
				} else if !errors.Is(queryErr, core.ErrGoToolchainOutput) {
					t.Fatalf("forged initializer error=%v, want output refusal", queryErr)
				}
				return nil
			}})
			if err != nil {
				t.Fatalf("initializer unit error=%v, want nil", err)
			}
		})
	}
}
