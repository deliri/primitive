package gotoolchain_test

import (
	"context"
	"github.com/deliri/primitive/v2026/gotoolchain"
	"go/ast"
	"testing"
)

func TestSSAExpressionCallPreservesNativeGoTypeForms(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name, source string
		present      bool
	}{
		{"comparison has no producer", `package sample;func observe(bool){};func Entry(x,y int){observe(x==y)}`, false},
		{"generic source frame", `package sample;func produce[T any](v T)T{return v};func observe[T any](T){};func Entry[T any](v T){observe(produce(v))}`, true},
		{"instantiated generic callee", `package sample;func produce[T any](v T)T{return v};func observe(int){};func Entry(){observe(produce(7))}`, true},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			a := nativeSymbolSourceAnalysis(t, t.TempDir(), row.source)
			err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], SourceValues: true, Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
				fn := v.Package.Func("Entry")
				var expression ast.Expr
				for node := range ast.Preorder(fn.Syntax()) {
					call, ok := node.(*ast.CallExpr)
					if !ok {
						continue
					}
					name, ok := call.Fun.(*ast.Ident)
					if ok && name.Name == "observe" {
						expression = call.Args[0]
						break
					}
				}
				if expression == nil {
					t.Fatal("fixture has no observed native type form")
				}
				got, queryErr := v.ExpressionCall(ctx, gotoolchain.SSAExpressionCallRequest{Function: fn, Expression: expression})
				if queryErr != nil || got.Validate() != nil || (got.Call != nil) != row.present || got.ResultIndex != 0 {
					t.Fatalf("native Go type form got=%+v,%v, want present=%v result0", got, queryErr, row.present)
				}
				if got.Call != nil {
					if got.Call.Parent() != fn {
						t.Fatal("producer escaped native frame")
					}
					named, err := v.CallFunction(ctx, gotoolchain.SSACallFunctionRequest{Call: got.Call})
					if err != nil || named.Function == nil || named.Function.Name() != "produce" {
						t.Fatalf("generic native target=%+v,%v, want produce", named, err)
					}
				}
				return nil
			}})
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}
