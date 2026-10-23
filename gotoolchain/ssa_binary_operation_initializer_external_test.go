package gotoolchain_test

import (
	"context"
	"errors"
	"fmt"
	"go/ast"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gotoolchain"
)

func TestSSABinaryOperationBorrowsActualPackageInitializer(t *testing.T) {
	t.Parallel()
	for _, debug := range []bool{false, true} {
		for _, clone := range []bool{false, true} {
			t.Run(fmt.Sprintf("debug=%t/clone=%t", debug, clone), func(t *testing.T) {
				t.Parallel()
				a := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;var left,right int;var Equal=left==right`)
				err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], SourceValues: debug, Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
					var expression *ast.BinaryExpr
					count := 0
					for _, file := range view.Source.Syntax {
						ast.Inspect(file, func(node ast.Node) bool {
							if binary, ok := node.(*ast.BinaryExpr); ok {
								expression = binary
								count++
							}
							return true
						})
					}
					fn := view.Package.Func("init")
					if count != 1 || fn == nil || fn.Syntax() != nil {
						t.Fatalf("synthetic initializer source=%d/%v, want one expression and source-free native initializer", count, fn)
					}
					if clone {
						copied := *fn
						copied.Blocks = nil
						fn = &copied
					}
					got, queryErr := view.BinaryOperationForExpression(ctx, gotoolchain.SSABinaryOperationRequest{Function: fn, Expression: expression})
					if clone {
						if got != (gotoolchain.SSABinaryOperationObservation{}) || !errors.Is(queryErr, core.ErrGoToolchainContract) {
							t.Fatalf("cloned initializer binary=%+v,%v, want zero,%v", got, queryErr, core.ErrGoToolchainContract)
						}
						return nil
					}
					if queryErr != nil || got.Operation == nil || got.Operation.Parent() != fn || got.Operation.Op != expression.Op || got.Operation.Pos() != expression.OpPos {
						t.Fatalf("native initializer binary=%+v,%v, want exact initializer operation and source,nil", got, queryErr)
					}
					return nil
				}})
				if err != nil {
					t.Fatalf("WithSSAUnit error=%v, want nil", err)
				}
			})
		}
	}
}
