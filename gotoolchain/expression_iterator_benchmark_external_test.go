package gotoolchain_test

import (
	"context"
	"go/ast"
	"testing"

	"github.com/deliri/primitive/v2026/gotoolchain"
)

func BenchmarkExpressionNilIteratorFrames(b *testing.B) {
	analysis := nativeSymbolSourceAnalysis(b, b.TempDir(), "package sample;func sequence(yield func(int)bool){yield(1)};func observe(any){};func F(){for range sequence{for range sequence{var value any;observe(value)}}}")
	for _, unit := range analysis.Units {
		err := analysis.WithSSAUnit(b.Context(), gotoolchain.SSAUnitRequest{Unit: unit, SourceValues: true, Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
			var expression ast.Expr
			for _, file := range unit.Syntax {
				for syntax, err := range gotoolchain.WalkSyntax(ctx, gotoolchain.SyntaxWalkRequest{Root: file}) {
					if err != nil {
						return err
					}
					call, ok := syntax.Node.(*ast.CallExpr)
					if !ok {
						continue
					}
					name, ok := call.Fun.(*ast.Ident)
					if ok && name.Name == "observe" {
						expression = call.Args[0]
					}
				}
			}
			if expression == nil {
				b.Fatal("missing native observation argument")
			}
			request := gotoolchain.ExpressionRequest{Analysis: analysis, Expression: expression}
			b.ReportAllocs()
			for b.Loop() {
				got, err := view.ExpressionNil(ctx, request)
				if err != nil || !got.Proven {
					b.Fatalf("nested iterator nil got%+v/%v wanttrue/nil", got, err)
				}
			}
			return nil
		}})
		if err != nil {
			b.Fatal(err)
		}
	}
}
