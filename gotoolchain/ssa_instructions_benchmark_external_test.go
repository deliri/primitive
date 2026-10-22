package gotoolchain_test

import (
	"context"
	"testing"

	"github.com/deliri/primitive/v2026/gotoolchain"
	"golang.org/x/tools/go/ssa"
)

func BenchmarkSSAInstructions(b *testing.B) {
	b.ReportAllocs()
	a := nativeSymbolSourceAnalysis(b, b.TempDir(), `package sample;func invoke(){};func Entry(){go invoke();defer invoke()}`)
	err := a.WithSSAUnit(b.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
		r := gotoolchain.SSAInstructionRequest{Function: v.Package.Func("Entry"), Extent: gotoolchain.SSAInstructionExtentLexical}
		for b.Loop() {
			calls := 0
			for instruction, err := range v.Instructions(ctx, r) {
				if err != nil || instruction == nil {
					b.Fatalf("instruction=(%v,%v), want native instruction and nil", instruction, err)
				}
				if _, ok := instruction.(ssa.CallInstruction); ok {
					calls++
				}
			}
			if calls != 2 {
				b.Fatalf("native call instructions=%d, want 2", calls)
			}
		}
		b.ReportMetric(2, "calls/op")
		return nil
	}})
	if err != nil {
		b.Fatalf("WithSSAUnit error=%v, want nil", err)
	}
}
