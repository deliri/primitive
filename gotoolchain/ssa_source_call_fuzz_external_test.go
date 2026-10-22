package gotoolchain_test

import (
	"context"
	"go/ast"
	"testing"

	"github.com/deliri/primitive/v2026/gotoolchain"
	"golang.org/x/tools/go/ssa"
)

func FuzzSSASourceCallConservesExpressionSelection(f *testing.F) {
	f.Add(uint8(0), uint8(0), true)
	f.Add(uint8(1), uint8(2), false)
	f.Add(uint8(2), uint8(1), true)
	f.Fuzz(func(t *testing.T, selected, mode uint8, debug bool) {
		selected %= 3
		mode %= 3
		body := ""
		for index := uint8(0); index < 3; index++ {
			prefix := ""
			if index == selected {
				switch mode {
				case 0:
				case 1:
					prefix = "defer "
				case 2:
					prefix = "go "
				}
			}
			body += prefix + []string{"first();", "second();", "third();"}[index]
		}
		a := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;func first(){};func second(){};func third(){};func Entry(){`+body+`}`)
		err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], SourceValues: debug, Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
			fn := v.Package.Func("Entry")
			var expressions [3]*ast.CallExpr
			count := 0
			ast.Inspect(fn.Syntax(), func(node ast.Node) bool {
				if call, ok := node.(*ast.CallExpr); ok {
					if count < len(expressions) {
						expressions[count] = call
					}
					count++
				}
				return true
			})
			if count != 3 {
				t.Fatalf("source expressions=%d, want 3", count)
			}
			got, queryErr := v.CallForExpression(ctx, gotoolchain.SSASourceCallRequest{Function: fn, Expression: expressions[selected]})
			if queryErr != nil || got.Validate() != nil || got.Instruction == nil {
				t.Fatalf("source selection=(%+v,%v), want selected instruction and nil", got, queryErr)
			}
			kind := uint8(255)
			switch got.Instruction.(type) {
			case *ssa.Call:
				kind = 0
			case *ssa.Defer:
				kind = 1
			case *ssa.Go:
				kind = 2
			}
			if kind != mode || got.Instruction.Common().Pos() != expressions[selected].Lparen || got.Instruction.Common().StaticCallee() != v.Package.Func([]string{"first", "second", "third"}[selected]) {
				t.Fatalf("native call kind/position/target=%d/%v/%v, want %d/selected source/exact native target", kind, got.Instruction.Common().Pos(), got.Instruction.Common().StaticCallee(), mode)
			}
			return nil
		}})
		if err != nil {
			t.Fatalf("WithSSAUnit error=%v, want nil", err)
		}
	})
}
