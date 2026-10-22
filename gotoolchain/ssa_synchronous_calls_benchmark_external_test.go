package gotoolchain_test

import (
	"context"
	"testing"

	"github.com/deliri/primitive/v2026/gotoolchain"
)

func BenchmarkSSASynchronousCalls(b *testing.B) {
	b.ReportAllocs()
	analysis := nativeSymbolSourceAnalysis(b, b.TempDir(), `package sample;func invoke(){};func Entry(){func(){invoke()}()}`)
	unit := analysis.Units[0]
	err := analysis.WithSSAUnit(b.Context(), gotoolchain.SSAUnitRequest{Unit: unit, Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
		request := gotoolchain.SSASynchronousCallRequest{Function: view.Package.Func("Entry"), Timing: gotoolchain.SSACallTimingSynchronous}
		for b.Loop() {
			calls := 0
			for call, err := range view.SynchronousCalls(ctx, request) {
				if err != nil || call == nil {
					b.Fatalf("call-stream benchmark=(%v, %v), want actual call and nil", call, err)
				}
				calls++
			}
			if calls != 2 {
				b.Fatalf("call-stream benchmark observations=%d, want literal and leaf call", calls)
			}
		}
		b.ReportMetric(2, "calls/op")
		return nil
	}})
	if err != nil {
		b.Fatalf("WithSSAUnit(call-stream benchmark) error = %v, want nil", err)
	}
}
