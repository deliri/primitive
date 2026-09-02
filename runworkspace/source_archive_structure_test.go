package runworkspace

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

func TestArchiveEntryPathStructurallyConvertsWireSlashesToNativePaths(t *testing.T) {
	t.Parallel()

	parsed, err := parser.ParseFile(token.NewFileSet(), "source_archive.go", nil, 0)
	if err != nil {
		t.Fatalf("parser.ParseFile(source_archive.go) error = %v, want nil", err)
	}
	found := false
	ast.Inspect(parsed, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || selector.Sel.Name != "FromSlash" || len(call.Args) != 1 {
			return true
		}
		owner, ownerOK := selector.X.(*ast.Ident)
		argument, argumentOK := call.Args[0].(*ast.Ident)
		found = ownerOK && owner.Name == "filepath" && argumentOK && argument.Name == "trimmed"
		return !found
	})
	if !found {
		t.Fatal("archiveEntryPath filepath.FromSlash(trimmed) call present = false, want true for Windows-native extraction")
	}
}
