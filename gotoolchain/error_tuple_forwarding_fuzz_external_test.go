package gotoolchain_test

import (
	"context"
	"github.com/deliri/primitive/v2026/gotoolchain"
	"go/ast"
	"testing"
)

func FuzzErrorPropagationTupleForwarding(f *testing.F) {
	for shape := uint8(0); shape < 8; shape++ {
		f.Add(shape)
	}
	f.Fuzz(func(t *testing.T, shape uint8) {
		signature, returns, results := "(int,error)", "7,supplied", "value,nil"
		assignment := "value,err:=validate();_=err;return " + results
		if shape&1 != 0 {
			signature, returns = "(error,int)", "supplied,7"
			assignment = "err,value:=validate();_=err;return nil,value"
		}
		body := "return validate()"
		if shape&4 != 0 {
			body = "return ((validate()))"
		}
		if shape&2 != 0 {
			body = assignment
		}
		source := "package sample;var supplied error;func validate()" + signature + "{return " + returns + "};func F()" + signature + "{" + body + "}"
		analysis := nativeSymbolSourceAnalysis(t, t.TempDir(), source)
		count := 0
		for _, unit := range analysis.Units {
			err := analysis.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: unit, SourceValues: true, Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
				for _, file := range unit.Syntax {
					for _, declaration := range file.Decls {
						function, ok := declaration.(*ast.FuncDecl)
						if !ok || function.Name.Name != "F" {
							continue
						}
						for syntax, err := range gotoolchain.WalkSyntax(ctx, gotoolchain.SyntaxWalkRequest{Root: function}) {
							if err != nil {
								return err
							}
							call, ok := syntax.Node.(*ast.CallExpr)
							if !ok {
								continue
							}
							name, ok := call.Fun.(*ast.Ident)
							if !ok || name.Name != "validate" {
								continue
							}
							got, err := view.ErrorPropagation(ctx, gotoolchain.ErrorPropagationRequest{Analysis: analysis, Call: call})
							want := shape&2 == 0
							if err != nil || got.Validate() != nil || got.Preserved != want {
								t.Fatalf("native tuple shape%d got%+v/%v want%t/nil", shape, got, err, want)
							}
							count++
						}
					}
				}
				return nil
			}})
			if err != nil {
				t.Fatal(err)
			}
		}
		if count != 1 {
			t.Fatalf("native tuple observations got%d want1", count)
		}
	})
}
