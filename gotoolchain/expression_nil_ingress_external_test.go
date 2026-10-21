package gotoolchain_test

import (
	"context"
	"go/ast"
	"testing"

	"github.com/deliri/primitive/v2026/gotoolchain"
)

func TestExpressionNilPackageInitializer(t *testing.T) {
	t.Parallel()
	analysis := nativeSymbolSourceAnalysis(t, t.TempDir(), "package sample;func observe(any)int{return 0};var First=observe(any(nil));var Second=observe(any((*int)(nil)))")
	calls := 0
	for _, unit := range analysis.Units {
		err := analysis.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: unit, SourceValues: true, Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
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
					if !ok || name.Name != "observe" {
						continue
					}
					got, err := view.ExpressionNil(ctx, gotoolchain.ExpressionRequest{Analysis: analysis, Expression: call.Args[0]})
					if err != nil || got.Proven != (calls == 0) {
						t.Fatalf("native initializer%d nil got%+v/%v", calls, got, err)
					}
					calls++
				}
			}
			return nil
		}})
		if err != nil {
			t.Fatal(err)
		}
	}
	if calls != 2 {
		t.Fatalf("initializer loans got%d want2", calls)
	}
}

func FuzzExpressionNilNativeInterfaceDistinction(f *testing.F) {
	for _, shape := range []uint8{0, 1, 2, 3, 4, 5} {
		f.Add(shape)
	}
	f.Fuzz(func(t *testing.T, shape uint8) {
		expressions := [6]string{"any(nil)", "any((*int)(nil))", "any([]int(nil))", "any(0)", "(*int)(nil)", "nil"}
		selected := shape % uint8(len(expressions))
		source := "package sample;func observe(any){};func F(){observe(" + expressions[selected] + ") }"
		analysis := nativeSymbolSourceAnalysis(t, t.TempDir(), source)
		calls := 0
		for _, unit := range analysis.Units {
			err := analysis.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: unit, SourceValues: true, Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
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
						if !ok || name.Name != "observe" {
							continue
						}
						got, err := view.ExpressionNil(ctx, gotoolchain.ExpressionRequest{Analysis: analysis, Expression: call.Args[0]})
						// A nil dynamic interface differs from an interface carrying a typed nil.
						want := selected == 0 || selected >= 4
						if err != nil || got.Validate() != nil || got.Proven != want {
							t.Fatalf("interface shape%d got%+v/%v want%t/nil", selected, got, err, want)
						}
						calls++
					}
				}
				return nil
			}})
			if err != nil {
				t.Fatal(err)
			}
		}
		if calls != 1 {
			t.Fatalf("native fuzz query loans got%d want1", calls)
		}
	})
}
