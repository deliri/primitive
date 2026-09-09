package tailnetconfig_test

import (
	"embed"
	"github.com/deliri/primitive/v2026/tailnetconfig"
	"go/ast"
	"go/parser"
	"go/token"
	"reflect"
	"slices"
	"strings"
	"testing"
)

//go:embed *.go
var productionSources embed.FS

func TestProductionStructInventory(t *testing.T) {
	t.Parallel()
	// Configuration is the authored intent; it is not a provider receipt.
	want := []string{reflect.TypeFor[tailnetconfig.Configuration]().Name()}
	var got []string
	files, err := productionSources.ReadDir(".")
	if err != nil {
		t.Fatalf("source inventory = %v, want nil", err)
	}
	for _, file := range files {
		if strings.HasSuffix(file.Name(), "_test.go") {
			continue
		}
		source, err := productionSources.ReadFile(file.Name())
		if err != nil {
			t.Fatalf("source read = %v, want nil", err)
		}
		parsed, err := parser.ParseFile(token.NewFileSet(), file.Name(), source, 0)
		if err != nil {
			t.Fatalf("source parse = %v, want nil", err)
		}
		for _, declaration := range parsed.Decls {
			general, ok := declaration.(*ast.GenDecl)
			if !ok {
				continue
			}
			for _, spec := range general.Specs {
				named, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				if _, ok := named.Type.(*ast.StructType); ok {
					got = append(got, named.Name.Name)
				}
			}
		}
	}
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Fatalf("production structs = %v, want classified %v", got, want)
	}
}
