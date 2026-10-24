package gotoolchain_test

import (
	"context"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gotoolchain"
	"golang.org/x/tools/go/ssa"
)

func TestSSALastFunctionArgumentBorrowsAcrossNativeCallKinds(t *testing.T) {
	t.Parallel()
	for _, row := range []struct{ name, body string }{{"ordinary call", "accept(\"\",named)"}, {"deferred call", "defer accept(\"\",named)"}, {"spawned call argument is not invocation proof", "go accept(\"\",named)"}} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			a := nativeSymbolSourceAnalysis(t, t.TempDir(), nativeFunctionArgumentPrelude+`func Entry(){`+row.body+`}`)
			err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
				var call ssa.CallInstruction
				for _, block := range v.Package.Func("Entry").Blocks {
					for _, instruction := range block.Instrs {
						if candidate, ok := instruction.(ssa.CallInstruction); ok {
							call = candidate
						}
					}
				}
				if call == nil {
					t.Fatalf("call kind fixture got nil, want native invocation")
				}
				got, err := v.LastFunctionArgument(ctx, gotoolchain.SSAFunctionArgumentRequest{Call: call})
				if err != nil || got.Validate() != nil || got.Function != v.Package.Func("named") || got.Value != call.Common().Args[1] {
					t.Fatalf("call kind got=%+v,%v, want exact source target and actual argument,nil", got, err)
				}
				for _, missing := range []ssa.CallInstruction{nil, (*ssa.Call)(nil), (*ssa.Defer)(nil), (*ssa.Go)(nil), &ssa.Defer{}, &ssa.Go{}} {
					got, err := v.LastFunctionArgument(ctx, gotoolchain.SSAFunctionArgumentRequest{Call: missing})
					if got != (gotoolchain.SSAFunctionValueObservation{}) || !errors.Is(err, core.ErrGoToolchainContract) || errors.Is(err, core.ErrGoToolchainOutput) {
						t.Fatalf("absent call kind got=%+v,%v, want zero,contract", got, err)
					}
				}
				return nil
			}})
			if err != nil {
				t.Fatalf("WithSSAUnit got=%v, want nil", err)
			}
		})
	}
}
