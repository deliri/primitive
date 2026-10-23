package gotoolchain_test

import (
	"context"
	"go/ast"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/gotoolchain"
	"golang.org/x/tools/go/ssa"
)

func BenchmarkSSASourceCallLastNativeExpression(b *testing.B) {
	a := nativeSymbolSourceAnalysis(b, b.TempDir(), `package sample;func sink(){};func Entry(){`+strings.Repeat(`sink();`, 257)+`}`)
	err := a.WithSSAUnit(b.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], SourceValues: true, Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
		fn := v.Package.Func("Entry")
		var expression *ast.CallExpr
		count := 0
		ast.Inspect(fn.Syntax(), func(node ast.Node) bool {
			if call, ok := node.(*ast.CallExpr); ok {
				expression = call
				count++
			}
			return true
		})
		if count != 257 {
			b.Fatalf("source expressions=%d, want 257", count)
		}
		request := gotoolchain.SSASourceCallRequest{Function: fn, Expression: expression}
		b.ReportAllocs()
		for b.Loop() {
			got, queryErr := v.CallForExpression(ctx, request)
			if queryErr != nil || got.Instruction == nil || got.Instruction.Common().Pos() != expression.Lparen {
				b.Fatalf("source lookup=(%+v,%v), want exact native call and nil", got, queryErr)
			}
		}
		b.ReportMetric(1, "lookups/op")
		return nil
	}})
	if err != nil {
		b.Fatalf("WithSSAUnit error=%v, want nil", err)
	}
}

func BenchmarkSSASourceCallFallback(b *testing.B) {
	for _, row := range []struct {
		name, body string
		debug      bool
	}{{"deferred", "defer sink();", true}, {"spawned", "go sink();", true}, {"debug_free", "sink();", false}} {
		b.Run(row.name, func(b *testing.B) {
			a := nativeSymbolSourceAnalysis(b, b.TempDir(), `package sample;func sink(){};func Entry(){`+strings.Repeat(row.body, 257)+`}`)
			err := a.WithSSAUnit(b.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], SourceValues: row.debug, Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
				fn := v.Package.Func("Entry")
				var expression *ast.CallExpr
				count := 0
				ast.Inspect(fn.Syntax(), func(node ast.Node) bool {
					if call, ok := node.(*ast.CallExpr); ok {
						expression = call
						count++
					}
					return true
				})
				calls := 0
				for instruction, queryErr := range v.Instructions(ctx, gotoolchain.SSAInstructionRequest{Function: fn, Extent: gotoolchain.SSAInstructionExtentFunction}) {
					if queryErr != nil {
						return queryErr
					}
					switch instruction.(type) {
					case *ssa.Call, *ssa.Defer, *ssa.Go:
						calls++
					}
				}
				if count != 257 || calls != 257 {
					b.Fatalf("source expressions/native calls=%d/%d, want 257/257", count, calls)
				}
				request := gotoolchain.SSASourceCallRequest{Function: fn, Expression: expression}
				b.ReportAllocs()
				for b.Loop() {
					got, queryErr := v.CallForExpression(ctx, request)
					if queryErr != nil || got.Instruction == nil || got.Instruction.Common().Pos() != expression.Lparen {
						b.Fatalf("source lookup=(%+v,%v), want exact native call and nil", got, queryErr)
					}
				}
				b.ReportMetric(1, "lookups/op")
				return nil
			}})
			if err != nil {
				b.Fatalf("WithSSAUnit error=%v, want nil", err)
			}
		})
	}
}
