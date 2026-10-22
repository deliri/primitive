package gotoolchain_test

import (
	"context"
	"testing"

	"github.com/deliri/primitive/v2026/gotoolchain"
	"golang.org/x/tools/go/ssa"
)

func BenchmarkSSAReceiverStoragePromotedBinding(b *testing.B) {
	a := nativeSymbolSourceAnalysis(b, b.TempDir(), `package sample;type Cell struct{};func(*Cell)First(){};func(*Cell)Second(){};type Box struct{Cell};func Entry(a *Box){first:=a.First;first();(*Box).Second(a)}`)
	err := a.WithSSAUnit(b.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], SourceValues: true, Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
		var calls [2]*ssa.Call
		count := 0
		for _, block := range v.Package.Func("Entry").Blocks {
			for _, instruction := range block.Instrs {
				if c, ok := instruction.(*ssa.Call); ok {
					if count < len(calls) {
						calls[count] = c
					}
					count++
				}
			}
		}
		if count != 2 {
			b.Fatalf("native fixture calls=%d, want 2", count)
		}
		request := gotoolchain.SSACallReceiverStorageRequest{Left: calls[0], Right: calls[1]}
		b.ReportAllocs()
		for b.Loop() {
			got, queryErr := v.SameReceiverStorage(ctx, request)
			if queryErr != nil || !got.Proven {
				b.Fatalf("receiver query=(%+v,%v), want proved and nil", got, queryErr)
			}
		}
		b.ReportMetric(1, "relations/op")
		return nil
	}})
	if err != nil {
		b.Fatalf("WithSSAUnit error=%v, want nil", err)
	}
}
