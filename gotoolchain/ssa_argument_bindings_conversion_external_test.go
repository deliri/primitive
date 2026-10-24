package gotoolchain_test

import (
	"context"
	"github.com/deliri/primitive/v2026/gotoolchain"
	"golang.org/x/tools/go/ssa"
	"testing"
)

func TestSSAArgumentBindingsFollowsAuthenticatedFunctionRepresentations(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name, body                 string
		named, captured, converted bool
	}{
		{"native named parameter", `Child(r)`, true, false, false},
		{"converted named parameter", `Callback(Child)(r)`, true, false, true},
		{"converted literal parameter", `Callback(func(local *Runner){})(r)`, false, false, true},
		{"native captured runner", `func(){_=r.N}()`, false, true, false},
		{"converted captured runner", `Capture(func(){_=r.N})()`, false, true, true},
		{"composed captured representation", `Capture2(Capture(func(){_=r.N}))()`, false, true, true},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			a := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;type Runner struct{N int};type Callback func(*Runner);type Capture func();type Capture2 func();func Child(r *Runner){};func Entry(r *Runner){`+row.body+`}`)
			err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
				fn := v.Package.Func("Entry")
				call := nativeCalleeCall(t, fn).(*ssa.Call)
				want := v.Package.Func("Child")
				if !row.named {
					if len(fn.AnonFuncs) != 1 {
						t.Fatalf("representation fixture got=%d source children, want one", len(fn.AnonFuncs))
					}
					want = fn.AnonFuncs[0]
				}
				if row.converted {
					if _, ok := call.Common().Value.(*ssa.ChangeType); !ok {
						t.Fatalf("representation fixture got=%T, want actual native ChangeType", call.Common().Value)
					}
				}
				source, err := v.CalleeSource(ctx, gotoolchain.SSACalleeSourceRequest{Call: call})
				if err != nil || source.Function != want || source.Value != call.Common().Value {
					t.Fatalf("representation source got=%+v,%v, want actual target=%v and original value,nil", source, err, want)
				}
				var local ssa.Value
				if row.captured {
					if len(want.FreeVars) != 1 {
						t.Fatalf("capture fixture got=%d slots, want one", len(want.FreeVars))
					}
					local = want.FreeVars[0]
				} else {
					if len(want.Params) != 1 {
						t.Fatalf("parameter fixture got=%d slots, want one", len(want.Params))
					}
					local = want.Params[0]
				}
				count := 0
				for got, queryErr := range v.ArgumentBindings(ctx, gotoolchain.SSAArgumentBindingsRequest{Call: call, Argument: fn.Params[0]}) {
					if queryErr != nil || got.Function != want || got.Local != local || got.Validate() != nil {
						t.Fatalf("representation binding got=%+v,%v, want target=%v local=%v,nil", got, queryErr, want, local)
					}
					count++
				}
				if count != 1 {
					t.Fatalf("representation bindings got=%d, want exactly one native binding", count)
				}
				return nil
			}})
			if err != nil {
				t.Fatalf("representation unit got=%v, want nil", err)
			}
		})
	}
}
