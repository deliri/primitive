package gotoolchain_test

import (
	"context"
	"testing"

	"github.com/deliri/primitive/v2026/gotoolchain"
)

func FuzzSSASynchronousCallsConserveInvocationTiming(f *testing.F) {
	f.Add(true, false, false, true)
	f.Add(false, false, false, true)
	f.Add(true, true, false, true)
	f.Add(true, false, true, true)
	f.Add(true, false, true, false)
	f.Fuzz(func(t *testing.T, invoked, spawned, deferred, includeDefers bool) {
		body := `f:=func(){invoke()};`
		if !invoked {
			body += `_=f`
		} else {
			if spawned {
				body += `go `
			} else if deferred {
				body += `defer `
			}
			body += `f()`
		}
		analysis := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;func invoke(){};func Entry(){`+body+`}`)
		unit := analysis.Units[0]
		timing := gotoolchain.SSACallTimingImmediate
		if includeDefers {
			timing = gotoolchain.SSACallTimingSynchronous
		}
		err := analysis.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: unit, Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
			calls := 0
			for call, err := range view.SynchronousCalls(ctx, gotoolchain.SSASynchronousCallRequest{Function: view.Package.Func("Entry"), Timing: timing}) {
				if err != nil {
					t.Fatalf("SynchronousCalls(fuzz) error = %v, want nil", err)
				}
				if target := call.Common().StaticCallee(); target != nil && target.Object() == unit.Types.Scope().Lookup("invoke") {
					calls++
				}
			}
			want := 0
			if invoked && !spawned && (!deferred || includeDefers) {
				want = 1
			}
			if calls != want {
				t.Fatalf("invoked=%t spawned=%t deferred=%t include=%t calls=%d, want %d", invoked, spawned, deferred, includeDefers, calls, want)
			}
			return nil
		}})
		if err != nil {
			t.Fatalf("WithSSAUnit(call-stream fuzz) error = %v, want nil", err)
		}
	})
}
