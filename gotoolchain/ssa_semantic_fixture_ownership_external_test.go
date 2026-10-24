package gotoolchain_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

func TestSSASemanticFixturesDeclareTempDirAtEachCaller(t *testing.T) {
	t.Parallel()
	for _, row := range []struct{ file, helper string }{
		{"ssa_argument_bindings_semantic_test.go", "nativeArgumentBindingMode"},
		{"ssa_closure_bindings_semantic_test.go", "nativeClosureBindingMode"},
		{"ssa_binary_operands_semantic_test.go", "nativeBinaryOperandsMode"},
		{"ssa_conversion_operand_semantic_test.go", "nativeConversionSemanticMode"},
	} {
		t.Run(row.helper, func(t *testing.T) {
			t.Parallel()
			source, err := parser.ParseFile(token.NewFileSet(), row.file, nil, 0)
			if err != nil {
				t.Fatalf("fixture source got=%v, want parsed native Go", err)
			}
			helpers, hidden, callers, owned := 0, 0, 0, 0
			for _, declaration := range source.Decls {
				fn, ok := declaration.(*ast.FuncDecl)
				if !ok || fn.Name.Name != row.helper {
					continue
				}
				helpers++
				ast.Inspect(fn.Body, func(node ast.Node) bool {
					call, ok := node.(*ast.CallExpr)
					if !ok {
						return true
					}
					selector, ok := call.Fun.(*ast.SelectorExpr)
					if ok && selector.Sel.Name == "TempDir" {
						hidden++
					}
					return true
				})
			}
			ast.Inspect(source, func(node ast.Node) bool {
				call, ok := node.(*ast.CallExpr)
				if !ok {
					return true
				}
				name, ok := call.Fun.(*ast.Ident)
				if !ok || name.Name != row.helper {
					return true
				}
				callers++
				if len(call.Args) != 3 {
					return true
				}
				directory, ok := call.Args[1].(*ast.CallExpr)
				if !ok || len(directory.Args) != 0 {
					return true
				}
				selector, ok := directory.Fun.(*ast.SelectorExpr)
				if !ok || selector.Sel.Name != "TempDir" {
					return true
				}
				owner, ok := selector.X.(*ast.Ident)
				if ok && owner.Name == "t" {
					owned++
				}
				return true
			})
			if helpers != 1 || hidden != 0 || callers != 2 || owned != callers {
				t.Fatalf("fixture ownership got helper=%d hiddenTempDir=%d callers=%d explicitOwner=%d, want1,0,2,2", helpers, hidden, callers, owned)
			}
		})
	}
}
