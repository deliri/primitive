package gotoolchain_test

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gotoolchain"
	"golang.org/x/tools/go/ssa"
)

func TestSSAArgumentBindingsRefusesBrokenCaptures(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name   string
		damage uint8
	}{
		{"missing capture", 0}, {"foreign capture", 1}, {"nil capture", 2}, {"typed nil capture", 3},
		{"detached closure", 4}, {"detached target", 5}, {"missing local", 6}, {"wrong capture type", 7}, {"foreign source target", 8}, {"closure created after invocation", 9},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			a := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;func Other(a *int){func(){_=a}()};func Entry(a *int,n int){func(){_=a}()}`)
			err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
				fn := v.Package.Func("Entry")
				call := nativeArgumentBindingLastCall(t, fn)
				closure, ok := call.Common().Value.(*ssa.MakeClosure)
				if !ok || len(closure.Bindings) != 1 {
					t.Fatalf("closure fixture got=%v, want one native capture", call.Common().Value)
				}
				switch row.damage {
				case 0:
					closure.Bindings = nil
				case 1:
					closure.Bindings[0] = v.Package.Func("Other").Params[0]
				case 2:
					closure.Bindings[0] = nil
				case 3:
					closure.Bindings[0] = (*ssa.Alloc)(nil)
				case 4:
					copied := *closure
					call.Common().Value = &copied
				case 5:
					copied := *closure.Fn.(*ssa.Function)
					closure.Fn = &copied
				case 6:
					closure.Fn.(*ssa.Function).FreeVars[0] = nil
				case 7:
					closure.Bindings[0] = fn.Params[1]
				case 8:
					closure.Fn = v.Package.Func("Other").AnonFuncs[0]
				case 9:
					block := call.Block()
					closureIndex, callIndex := -1, -1
					for index, instruction := range block.Instrs {
						if instruction == closure {
							closureIndex = index
						}
						if instruction == call {
							callIndex = index
						}
					}
					if closureIndex < 0 || callIndex <= closureIndex {
						t.Fatalf("closure fixture got indices=%d,%d, want creation before call", closureIndex, callIndex)
					}
					block.Instrs[closureIndex], block.Instrs[callIndex] = block.Instrs[callIndex], block.Instrs[closureIndex]
				default:
					t.Fatalf("capture damage got=%d, want named boundary", row.damage)
				}
				refusals := 0
				for got, queryErr := range v.ArgumentBindings(ctx, gotoolchain.SSAArgumentBindingsRequest{Call: call, Argument: fn.Params[0]}) {
					if got != (gotoolchain.SSAArgumentBindingObservation{}) || !errors.Is(queryErr, core.ErrGoToolchainOutput) {
						t.Fatalf("corrupt capture got=%+v,%v, want zero,typed output refusal", got, queryErr)
					}
					refusals++
				}
				if refusals != 1 {
					t.Fatalf("capture refusals got=%d, want exactly one", refusals)
				}
				return nil
			}})
			if err != nil {
				t.Fatalf("WithSSAUnit got=%v, want nil", err)
			}
		})
	}
}

func TestSSAArgumentBindingsPreservesConsumerStopAndCancellation(t *testing.T) {
	t.Parallel()
	a := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;func helper(x,y *int){};func Entry(a *int){helper(a,a)}`)
	err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
		fn := v.Package.Func("Entry")
		r := gotoolchain.SSAArgumentBindingsRequest{Call: nativeArgumentBindingLastCall(t, fn), Argument: fn.Params[0]}
		stopped := 0
		v.ArgumentBindings(ctx, r)(func(got gotoolchain.SSAArgumentBindingObservation, err error) bool {
			if stopped != 0 || err != nil || got.Local != got.Function.Params[0] {
				t.Fatalf("consumer stop got count=%d value=%+v error=%v, want first exact binding,nil", stopped, got, err)
			}
			stopped++
			return false
		})
		if stopped != 1 {
			t.Fatalf("consumer stop count=%d, want one", stopped)
		}
		query, cancel := context.WithCancelCause(ctx)
		defer cancel(nil)
		count, refusals := 0, 0
		for got, queryErr := range v.ArgumentBindings(query, r) {
			if count == 0 {
				if queryErr != nil || got.Local != got.Function.Params[0] {
					t.Fatalf("cancel prelude got=%+v,%v, want first binding,nil", got, queryErr)
				}
				count++
				cancel(io.ErrClosedPipe)
				continue
			}
			if got != (gotoolchain.SSAArgumentBindingObservation{}) || !errors.Is(queryErr, context.Canceled) || !errors.Is(queryErr, io.ErrClosedPipe) {
				t.Fatalf("cancel continuation got=%+v,%v, want zero and both cause identities", got, queryErr)
			}
			refusals++
		}
		if count != 1 || refusals != 1 {
			t.Fatalf("cancel accounting got=%d,%d, want one native prefix and one refusal", count, refusals)
		}
		return nil
	}})
	if err != nil {
		t.Fatalf("WithSSAUnit got=%v, want nil", err)
	}
}
