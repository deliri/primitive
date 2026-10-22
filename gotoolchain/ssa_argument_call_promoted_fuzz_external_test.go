package gotoolchain_test

import (
	"context"
	"go/types"
	"testing"

	"github.com/deliri/primitive/v2026/gotoolchain"
)

func FuzzSSAArgumentCallConservesPromotedPathAndTiming(f *testing.F) {
	f.Add(true, true, true, false)
	f.Add(false, true, true, false)
	f.Add(true, false, true, false)
	f.Add(true, true, false, false)
	f.Add(true, true, true, true)
	f.Fuzz(func(t *testing.T, selected, invoked, deferred, includeDefers bool) {
		receiver := "r.Cell"
		if !selected {
			receiver = "r.Other"
		}
		body := `f:=func(){` + receiver + `.Invoke()};`
		if invoked {
			if deferred {
				body += "defer "
			}
			body += "f()"
		} else {
			body += "_=f"
		}
		timing := gotoolchain.SSACallTimingImmediate
		if includeDefers {
			timing = gotoolchain.SSACallTimingSynchronous
		}
		analysis := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;type Cell struct{Count int};func(*Cell)Invoke(){};type Owner struct{Cell;Other Cell};func Entry(r *Owner){`+body+`}`)
		unit := analysis.Units[0]
		err := analysis.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: unit, Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
			frame := view.Package.Func("Entry")
			target := types.NewMethodSet(frame.Params[0].Type()).Lookup(nil, "Invoke").Obj().(*types.Func)
			got, err := view.ArgumentCall(ctx, gotoolchain.SSAArgumentCallRequest{Timing: timing, Function: frame, Argument: frame.Params[0], Target: target})
			want := selected && invoked && (!deferred || includeDefers)
			if err != nil || got.Proven != want {
				t.Fatalf("promoted selected=%t invoked=%t deferred=%t include=%t observation=(%+v, %v), want proven=%t", selected, invoked, deferred, includeDefers, got, err, want)
			}
			return nil
		}})
		if err != nil {
			t.Fatalf("WithSSAUnit(promoted fuzz) error = %v, want nil", err)
		}
	})
}
