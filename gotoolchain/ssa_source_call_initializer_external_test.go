package gotoolchain_test

import (
	"context"
	"errors"
	"go/ast"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gotoolchain"
)

func TestSSASourceCallBorrowsSyntheticPackageInitializer(t *testing.T) {
	t.Parallel()
	a := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;func value()int{return 1};var Initial=value()`)
	err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], SourceValues: true, Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
		fn := v.Package.Func("init")
		if fn == nil || fn.Syntax() != nil {
			t.Fatalf("native init=%v, want actual synthetic function with nil syntax", fn)
		}
		var expression *ast.CallExpr
		count := 0
		for _, file := range v.Source.Syntax {
			ast.Inspect(file, func(node ast.Node) bool {
				if call, ok := node.(*ast.CallExpr); ok {
					expression = call
					count++
				}
				return true
			})
		}
		if count != 1 {
			t.Fatalf("initializer source calls=%d, want 1", count)
		}
		got, queryErr := v.CallForExpression(ctx, gotoolchain.SSASourceCallRequest{Function: fn, Expression: expression})
		if queryErr != nil || got.Instruction == nil || got.Instruction.Parent() != fn || got.Instruction.Common().StaticCallee() != v.Package.Func("value") || got.Instruction.Common().Pos() != expression.Lparen {
			t.Fatalf("synthetic source call=(%+v,%v), want exact native initializer call and nil", got, queryErr)
		}
		clone := *fn
		clone.Blocks = nil
		got, queryErr = v.CallForExpression(ctx, gotoolchain.SSASourceCallRequest{Function: &clone, Expression: expression})
		if got != (gotoolchain.SSASourceCallObservation{}) || !errors.Is(queryErr, core.ErrGoToolchainContract) {
			t.Fatalf("cloned initializer=(%+v,%v), want zero and contract refusal", got, queryErr)
		}
		return nil
	}})
	if err != nil {
		t.Fatalf("WithSSAUnit error=%v, want nil", err)
	}
}
