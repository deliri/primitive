package gotoolchain_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/gotoolchain"
	"golang.org/x/tools/go/ssa"
)

func BenchmarkSSABinaryOperationForExpression(b *testing.B) {
	b.ReportAllocs()
	const population = 257
	for _, debug := range []bool{false, true} {
		b.Run(fmt.Sprintf("debug=%t", debug), func(b *testing.B) {
			b.ReportAllocs()
			a := nativeSymbolSourceAnalysis(b, b.TempDir(), `package sample;func consumeBool(bool){};func consumeInt(int){};func Entry(x,y int){`+strings.Repeat(`consumeInt(1);`, population)+`consumeBool(x==y)}`)
			err := a.WithSSAUnit(b.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], SourceValues: debug, Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
				fn := view.Package.Func("Entry")
				request := gotoolchain.SSABinaryOperationRequest{Function: fn, Expression: nativeBinaryExpression(b, fn)}
				calls := 0
				for instruction, queryErr := range view.Instructions(ctx, gotoolchain.SSAInstructionRequest{Function: fn, Extent: gotoolchain.SSAInstructionExtentFunction}) {
					if queryErr != nil {
						return queryErr
					}
					if _, call := instruction.(*ssa.Call); call {
						calls++
					}
				}
				baseline, queryErr := view.BinaryOperationForExpression(ctx, request)
				if queryErr != nil || baseline.Operation == nil || calls != population+1 {
					b.Fatalf("native binary workload=%+v,%v calls=%d, want operation,nil,%d", baseline, queryErr, calls, population+1)
				}
				for b.Loop() {
					got, queryErr := view.BinaryOperationForExpression(ctx, request)
					if queryErr != nil || got.Operation != baseline.Operation {
						b.Fatalf("native binary query=%+v,%v, want actual operation,nil", got, queryErr)
					}
				}
				b.ReportMetric(1, "operations/op")
				b.ReportMetric(float64(calls), "native_calls/op")
				return nil
			}})
			if err != nil {
				b.Fatalf("WithSSAUnit error=%v, want nil", err)
			}
		})
	}
}
