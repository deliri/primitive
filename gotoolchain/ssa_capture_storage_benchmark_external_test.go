package gotoolchain_test

import (
	"context"
	"github.com/deliri/primitive/v2026/gotoolchain"
	"go/types"
	"golang.org/x/tools/go/ssa"
	"testing"
)

func BenchmarkSSAStableCaptureStorage(b *testing.B) {
	analysis := nativeSymbolSourceAnalysis(b, b.TempDir(), `package sample;type Record struct{};func observe(*Record){};func F(root *Record){copy:=root;fn:=func(){observe(copy)};fn()}`)
	unit := analysis.Units[0]
	err := analysis.WithSSAUnit(b.Context(), gotoolchain.SSAUnitRequest{Unit: unit, SourceValues: true, Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
		parent := view.Program.FuncValue(unit.Types.Scope().Lookup("F").(*types.Func))
		var operand ssa.Value
		for _, block := range parent.AnonFuncs[0].Blocks {
			for _, instruction := range block.Instrs {
				if call, ok := instruction.(*ssa.Call); ok && call.Common().StaticCallee() != nil && call.Common().StaticCallee().Name() == "observe" {
					operand = call.Common().Args[0]
				}
			}
		}
		if operand == nil {
			b.Fatal("native benchmark fixture omitted observed operand")
		}
		b.ReportAllocs()
		for b.Loop() {
			got, err := view.SameStorage(ctx, gotoolchain.SSAStorageRequest{Left: parent.Params[0], Right: operand})
			if err != nil || !got.Proven {
				b.Fatalf("native capture relation got%+v/%v wantproved/nil", got, err)
			}
		}
		b.ReportMetric(1, "relations/op")
		return nil
	}})
	if err != nil {
		b.Fatal(err)
	}
}
