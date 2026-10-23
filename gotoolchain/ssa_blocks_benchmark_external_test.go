package gotoolchain_test

import (
	"context"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/gotoolchain"
)

func BenchmarkSSABlocks(b *testing.B) {
	b.ReportAllocs()
	const population = 257
	a := nativeSymbolSourceAnalysis(b, b.TempDir(), `package sample;func invoke(){};func Entry(flag bool){`+strings.Repeat(`if flag{invoke()};`, population)+`}`)
	err := a.WithSSAUnit(b.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
		fn := v.Package.Func("Entry")
		const blocks = 2*population + 1
		if len(fn.Blocks) != blocks {
			b.Fatalf("native blocks=%d, want %d", len(fn.Blocks), blocks)
		}
		request := gotoolchain.SSABlockRequest{Function: fn}
		for b.Loop() {
			observed := 0
			for block, queryErr := range v.Blocks(ctx, request) {
				if queryErr != nil || block != fn.Blocks[observed] {
					b.Fatalf("native block=%v,%v, want actual block %d,nil", block, queryErr, observed)
				}
				observed++
			}
			if observed != blocks {
				b.Fatalf("native blocks=%d, want %d", observed, blocks)
			}
		}
		b.ReportMetric(blocks, "blocks/op")
		return nil
	}})
	if err != nil {
		b.Fatalf("WithSSAUnit error=%v, want nil", err)
	}
}
