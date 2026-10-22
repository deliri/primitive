package gotoolchain_test

import (
	"context"
	"go/types"
	"testing"

	"github.com/deliri/primitive/v2026/gotoolchain"
)

func BenchmarkSSAArgumentCall(b *testing.B) {
	b.ReportAllocs()
	analysis := nativeSymbolSourceAnalysis(b, b.TempDir(), `package sample;type Runner struct{};func(*Runner)Invoke(){};func Entry(r *Runner){func(){r.Invoke()}()}`)
	unit := analysis.Units[0]
	err := analysis.WithSSAUnit(b.Context(), gotoolchain.SSAUnitRequest{Unit: unit, Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
		frame := view.Package.Func("Entry")
		target := types.NewMethodSet(frame.Params[0].Type()).Lookup(nil, "Invoke").Obj().(*types.Func)
		request := gotoolchain.SSAArgumentCallRequest{Function: frame, Argument: frame.Params[0], Target: target}
		for b.Loop() {
			got, err := view.ArgumentCall(ctx, request)
			if err != nil || !got.Proven {
				b.Fatalf("ArgumentCall(benchmark) = (%+v, %v), want true and nil", got, err)
			}
		}
		return nil
	}})
	if err != nil {
		b.Fatalf("WithSSAUnit(benchmark) error = %v, want nil", err)
	}
}
