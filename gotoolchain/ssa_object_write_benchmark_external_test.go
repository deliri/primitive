package gotoolchain_test

import (
	"context"
	"testing"

	"github.com/deliri/primitive/v2026/gotoolchain"
)

func BenchmarkSSAObjectWritten(b *testing.B) {
	b.ReportAllocs()
	a := nativeSymbolSourceAnalysis(b, b.TempDir(), `package sample;func Entry()(result error){defer func(){result=nil}();return}`)
	err := a.WithSSAUnit(b.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], SourceValues: true, Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
		fn := v.Package.Func("Entry")
		request := gotoolchain.SSAObjectWriteRequest{Function: fn.AnonFuncs[0], Object: fn.Signature.Results().At(0)}
		for b.Loop() {
			observation, err := v.ObjectWritten(ctx, request)
			if err != nil || !observation.Proven {
				b.Fatalf("native object write=(%+v,%v), want proven and nil", observation, err)
			}
		}
		b.ReportMetric(1, "write/op")
		return nil
	}})
	if err != nil {
		b.Fatalf("WithSSAUnit error=%v, want nil", err)
	}
}
