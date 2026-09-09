package tailnet

import (
	"embed"
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

type inventoryRole uint8

const (
	roleCapability inventoryRole = iota + 1
	roleOperationTransport
)

func TestProductionStructInventory(t *testing.T) {
	t.Parallel()
	entries := []struct {
		typeOf reflect.Type
		role   inventoryRole
	}{
		{reflect.TypeFor[Client](), roleCapability},
		{reflect.TypeFor[GoogleIdentity](), roleCapability},
		{reflect.TypeFor[enrollmentTransport](), roleOperationTransport},
	}
	var want []string
	for _, entry := range entries {
		want = append(want, entry.typeOf.Name())
		if entry.role != roleCapability && entry.role != roleOperationTransport {
			t.Fatalf("inventory role = %v, want declared role", entry.role)
		}
	}
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
