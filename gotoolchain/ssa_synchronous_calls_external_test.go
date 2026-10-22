package gotoolchain_test

import (
	"context"
	"testing"

	"github.com/deliri/primitive/v2026/gotoolchain"
)

func TestSSASynchronousCallsFollowActualNativeInvocations(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name, body string
		timing     gotoolchain.SSACallTiming
		want       int
	}{
		{"direct call", `invoke()`, gotoolchain.SSACallTimingImmediate, 1},
		{"empty source", ``, gotoolchain.SSACallTimingSynchronous, 0},
		{"uninvoked literal", `_=func(){invoke()}`, gotoolchain.SSACallTimingSynchronous, 0},
		{"invoked literal", `func(){invoke()}()`, gotoolchain.SSACallTimingImmediate, 1},
		{"copied literal", `f:=func(){invoke()};g:=f;g()`, gotoolchain.SSACallTimingImmediate, 1},
		{"replaced literal", `f:=func(){invoke()};f=func(){};f()`, gotoolchain.SSACallTimingImmediate, 0},
		{"spawned call", `go invoke()`, gotoolchain.SSACallTimingSynchronous, 0},
		{"spawned literal", `go func(){invoke()}()`, gotoolchain.SSACallTimingSynchronous, 0},
		{"deferred call remains synchronous", `defer invoke()`, gotoolchain.SSACallTimingSynchronous, 1},
		{"deferred call is not immediate", `defer invoke()`, gotoolchain.SSACallTimingImmediate, 0},
		{"deferred literal remains synchronous", `defer func(){invoke()}()`, gotoolchain.SSACallTimingSynchronous, 1},
		{"deferred literal is not immediate", `defer func(){invoke()}()`, gotoolchain.SSACallTimingImmediate, 0},
		{"untaken constant branch", `if false{invoke()}`, gotoolchain.SSACallTimingSynchronous, 0},
		{"taken constant branch", `if true{invoke()}`, gotoolchain.SSACallTimingSynchronous, 1},
		{"unknown branch is possible", `if branch{invoke()}`, gotoolchain.SSACallTimingSynchronous, 1},
		{"opaque named helper grants no body", `helper()`, gotoolchain.SSACallTimingSynchronous, 0},
		{"nested literal", `func(){func(){invoke()}()}()`, gotoolchain.SSACallTimingImmediate, 1},
		{"each actual invocation is lent", `f:=func(){invoke()};f();f()`, gotoolchain.SSACallTimingImmediate, 2},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			analysis := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;var branch bool;func invoke(){};func helper(){invoke()};func Entry(){`+row.body+`}`)
			unit := analysis.Units[0]
			err := analysis.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: unit, Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
				seen := 0
				for call, err := range view.SynchronousCalls(ctx, gotoolchain.SSASynchronousCallRequest{Function: view.Package.Func("Entry"), Timing: row.timing}) {
					if err != nil {
						t.Fatalf("SynchronousCalls() error = %v, want nil", err)
					}
					if target := call.Common().StaticCallee(); target != nil && target.Object() == unit.Types.Scope().Lookup("invoke") {
						seen++
					}
				}
				if seen != row.want {
					t.Fatalf("native invoke observations=%d, want %d", seen, row.want)
				}
				return nil
			}})
			if err != nil {
				t.Fatalf("WithSSAUnit(synchronous calls) error = %v, want nil", err)
			}
		})
	}
}
