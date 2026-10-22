package gotoolchain_test

import (
	"context"
	"go/types"
	"testing"

	"github.com/deliri/primitive/v2026/gotoolchain"
)

func BenchmarkSSANilBoxedReturns(b *testing.B) {
	b.ReportAllocs()
	a := nativeSymbolSourceAnalysis(b, b.TempDir(), `package sample;type E struct{};func(*E)Error()string{return "error"};func Entry()error{return (*E)(nil)}`)
	err := a.WithSSAUnit(b.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
		r := gotoolchain.SSANilBoxedReturnRequest{Function: v.Package.Func("Entry"), Interface: types.Universe.Lookup("error").Type().Underlying().(*types.Interface)}
		for b.Loop() {
			observed := 0
			for returned, err := range v.NilBoxedReturns(ctx, r) {
				if err != nil || returned == nil {
					b.Fatalf("nil-boxed return=(%v,%v), want actual native return and nil", returned, err)
				}
				observed++
			}
			if observed != 1 {
				b.Fatalf("nil-boxed return observations=%d, want 1", observed)
			}
		}
		b.ReportMetric(1, "return/op")
		return nil
	}})
	if err != nil {
		b.Fatalf("WithSSAUnit error=%v, want nil", err)
	}
}
