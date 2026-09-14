package accesspermit

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestProductionStructInventory(t *testing.T) {
	t.Parallel()
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("Glob() error = %v, want nil", err)
	}
	var got []string
	for _, path := range files {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			t.Fatalf("ParseFile(%s) error = %v, want nil", path, err)
		}
		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.TYPE {
				continue
			}
			for _, spec := range gen.Specs {
				typeSpec := spec.(*ast.TypeSpec)
				if _, ok := typeSpec.Type.(*ast.StructType); ok {
					got = append(got, typeSpec.Name.Name)
				}
			}
		}
	}
	slices.Sort(got)
	// Binding, Window, Terms: typed protocol facts. Document and both response
	// records: sealed projections. Verified: unforgeable authenticated capability.
	want := []string{"Binding", "CheckInResponse", "Document", "RegistrationResponse", "Terms", "Verified", "Window"}
	if !slices.Equal(got, want) {
		t.Fatalf("production structs = %v, want classified %v", got, want)
	}
}
