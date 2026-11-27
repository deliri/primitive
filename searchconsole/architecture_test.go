package searchconsole

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"slices"
	"strings"
	"testing"
)

func TestSearchConsoleProductionStructRoles(t *testing.T) {
	t.Parallel()
	packages, err := parser.ParseDir(token.NewFileSet(), ".", func(file fs.FileInfo) bool {
		return !strings.HasSuffix(file.Name(), "_test.go")
	}, 0)
	if err != nil || packages["searchconsole"] == nil {
		t.Fatalf("SDK source parse = %v, want package searchconsole", err)
	}
	var structs, classified []string
	for _, file := range packages["searchconsole"].Files {
		for _, decl := range file.Decls {
			switch d := decl.(type) {
			case *ast.GenDecl:
				for _, spec := range d.Specs {
					if named, ok := spec.(*ast.TypeSpec); ok {
						if _, ok := named.Type.(*ast.StructType); ok {
							structs = append(structs, named.Name.Name)
						}
					}
				}
			case *ast.FuncDecl:
				if d.Recv != nil && slices.Contains([]string{"searchProtocolFact", "searchSealedProjection", "searchCapability", "searchInternalFlow"}, d.Name.Name) {
					receiver := d.Recv.List[0].Type
					if indexed, ok := receiver.(*ast.IndexListExpr); ok {
						receiver = indexed.X
					}
					if name, ok := receiver.(*ast.Ident); ok {
						classified = append(classified, name.Name)
					}
				}
			}
		}
	}
	slices.Sort(structs)
	slices.Sort(classified)
	if !slices.Equal(structs, classified) {
		t.Fatalf("SDK struct roles = %q, want exactly %q", classified, structs)
	}
}
