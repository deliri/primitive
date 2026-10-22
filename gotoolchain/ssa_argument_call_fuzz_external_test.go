package gotoolchain_test

import (
	"context"
	"go/types"
	"testing"

	"github.com/deliri/primitive/v2026/gotoolchain"
)

func FuzzSSAArgumentCallConservesTargetOperandAndInvocation(f *testing.F) {
	f.Add(true, true, true, false)
	f.Add(false, true, true, false)
	f.Add(true, false, true, false)
	f.Add(true, true, false, false)
	f.Add(true, true, true, true)
	f.Fuzz(func(t *testing.T, sameTarget, sameReceiver, invoked, spawned bool) {
		receiver := "r"
		if !sameReceiver {
			receiver = "(&Runner{})"
		}
		method := "Invoke"
		if !sameTarget {
			method = "Other"
		}
		body := `f:=func(){` + receiver + `.` + method + `()};`
		if invoked {
			if spawned {
				body += "go "
			}
			body += "f()"
		} else {
			body += "_=f"
		}
		analysis := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;type Runner struct{};func(*Runner)Invoke(){};func(*Runner)Other(){};func Entry(r *Runner){`+body+`}`)
		unit := analysis.Units[0]
		err := analysis.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: unit, Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
			frame := view.Package.Func("Entry")
			target := types.NewMethodSet(frame.Params[0].Type()).Lookup(nil, "Invoke").Obj().(*types.Func)
			got, err := view.ArgumentCall(ctx, gotoolchain.SSAArgumentCallRequest{Function: frame, Argument: frame.Params[0], Target: target})
			want := sameTarget && sameReceiver && invoked && !spawned
			if err != nil || got.Proven != want {
				t.Fatalf("ArgumentCall(target=%t receiver=%t invoked=%t spawned=%t) = (%+v, %v), want proven=%t", sameTarget, sameReceiver, invoked, spawned, got, err, want)
			}
			return nil
		}})
		if err != nil {
			t.Fatalf("WithSSAUnit(fuzz native invocation) error = %v, want nil", err)
		}
	})
}
