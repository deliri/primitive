package textrepair

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

var _ core.Validatable = Request{}

func TestProductionStructInventory(t *testing.T) {
	t.Parallel()
	// Request is the sole internal-flow carrier: caller text plus output budget.
	want := []string{"Request"}
	var got []string
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("ReadDir error = %v, want nil", err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), name, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatalf("ParseFile(%q) error = %v, want nil", name, err)
		}
		for _, declaration := range file.Decls {
			decl, ok := declaration.(*ast.GenDecl)
			if !ok || decl.Tok != token.TYPE {
				continue
			}
			for _, spec := range decl.Specs {
				typ := spec.(*ast.TypeSpec)
				if _, ok := typ.Type.(*ast.StructType); ok {
					got = append(got, typ.Name.Name)
				}
			}
		}
	}
	slices.Sort(got)
	if !slices.Equal(got, want) {
		t.Fatalf("production structs = %v, want classified %v", got, want)
	}
}
