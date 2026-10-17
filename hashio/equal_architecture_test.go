package hashio

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
	"testing"
)

// Equality results alone cannot prove timing behavior. Bind this capability
// directly to Go's native constant-time engine instead of timing a CPU.
func TestEqualBytesRunsTheGoConstantTimeEngine(t *testing.T) {
	t.Parallel()
	file, err := parser.ParseFile(token.NewFileSet(), "equal.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	owner := ""
	for _, spec := range file.Imports {
		path, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			t.Fatal(err)
		}
		if path == "crypto/subtle" {
			owner = "subtle"
			if spec.Name != nil {
				owner = spec.Name.Name
			}
		}
	}
	calls := 0
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "EqualBytes" {
			continue
		}
		ast.Inspect(fn.Body, func(node ast.Node) bool {
			if _, ok := node.(*ast.ForStmt); ok {
				t.Error("native equality was replaced by a local loop")
			}
			if _, ok := node.(*ast.RangeStmt); ok {
				t.Error("native equality was replaced by a local loop")
			}
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			selector, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			base, ok := selector.X.(*ast.Ident)
			if ok && owner != "" && base.Name == owner && selector.Sel.Name == "ConstantTimeCompare" {
				calls++
			}
			return true
		})
	}
	if calls != 1 {
		t.Fatalf("Go constant-time engine calls = %d, want one", calls)
	}
}
