package runworkspace

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

func TestLinuxResidueProcessObservationOpensTheConfiguredProcRoot(t *testing.T) {
	t.Parallel()

	parsed, err := parser.ParseFile(token.NewFileSet(), "residue_linux.go", nil, 0)
	if err != nil {
		t.Fatalf("parser.ParseFile(residue_linux.go) error = %v, want nil", err)
	}
	openRoot, openParent := false, false
	for _, declaration := range parsed.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok || function.Name.Name != "observeSubjectProcesses" {
			continue
		}
		ast.Inspect(function.Body, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			selector, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			owner, ok := selector.X.(*ast.Ident)
			if !ok || owner.Name != "filestore" {
				return true
			}
			openRoot = openRoot || selector.Sel.Name == "OpenRoot"
			openParent = openParent || selector.Sel.Name == "OpenParent"
			return true
		})
	}
	if !openRoot || openParent {
		t.Fatalf("observeSubjectProcesses() filestore root calls = OpenRoot:%t OpenParent:%t, want true/false", openRoot, openParent)
	}
}
