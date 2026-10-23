package gotoolchain_test

import (
	"context"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/gotoolchain"
)

func BenchmarkSSAReferrersAfterNativePrefix(b *testing.B) {
	b.ReportAllocs()
	a := nativeSymbolSourceAnalysis(b, b.TempDir(), `package sample;func sink(int){};func Entry(x int){`+strings.Repeat("sink(x);", 257)+`f:=func(){sink(x)};f();f()}`)
	err := a.WithSSAUnit(b.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
		value, queryErr := nativeReferrerValue(ctx, view)
		if queryErr != nil || value == nil {
			b.Fatalf("native closure=%v,%v, want nonnil,nil", value, queryErr)
		}
		for b.Loop() {
			uses := 0
			for instruction, queryErr := range view.Referrers(ctx, gotoolchain.SSAReferrerRequest{Instruction: value}) {
				if queryErr != nil || instruction == nil {
					b.Fatalf("native use=%v,%v, want instruction,nil", instruction, queryErr)
				}
				uses++
			}
			if uses != 2 {
				b.Fatalf("native uses=%d, want 2", uses)
			}
		}
		b.ReportMetric(2, "uses/op")
		return nil
	}})
	if err != nil {
		b.Fatalf("WithSSAUnit error=%v, want nil", err)
	}
}
