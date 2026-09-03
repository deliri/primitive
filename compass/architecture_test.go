package compass

import (
	"embed"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"reflect"
	"slices"
	"strings"
	"testing"
)

type compassProtocolFact[T any] struct{}

// compassContractInventory gives every production data carrier an explicit
// role. Reflection checks the field names while these generic fields make a
// wrong type or deleted type fail at compile time.
type compassContractInventory struct {
	Configuration      compassProtocolFact[Configuration]
	Project            compassProtocolFact[Project]
	ProjectName        compassProtocolFact[ProjectName]
	ReleaseCoordinates compassProtocolFact[ReleaseCoordinates]
}

//go:embed *.go
var compassSource embed.FS

func TestCompassProductionStructsHaveCompilerVisibleDataFlowRoles(t *testing.T) {
	t.Parallel()

	got, gotErr := compassProductionStructNames()
	if gotErr != nil {
		t.Fatalf("compassProductionStructNames() error = %v, want nil", gotErr)
	}
	want := compassClassifiedStructNames()
	if !slices.Equal(got, want) {
		t.Fatalf("Compass production structs = %q, want classified %q", got, want)
	}
}

func compassProductionStructNames() ([]string, error) {
	names, err := fs.Glob(compassSource, "*.go")
	if err != nil {
		return nil, err
	}
	var structs []string
	for _, name := range names {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		source, err := compassSource.ReadFile(name)
		if err != nil {
			return nil, err
		}
		file, err := parser.ParseFile(token.NewFileSet(), name, source, parser.SkipObjectResolution)
		if err != nil {
			return nil, err
		}
		structs = append(structs, compassStructNames(file)...)
	}
	slices.Sort(structs)
	return structs, nil
}

func compassStructNames(file *ast.File) []string {
	var names []string
	for _, declaration := range file.Decls {
		generic, ok := declaration.(*ast.GenDecl)
		if !ok || generic.Tok != token.TYPE {
			continue
		}
		for _, raw := range generic.Specs {
			specification, ok := raw.(*ast.TypeSpec)
			if !ok {
				continue
			}
			if _, ok := specification.Type.(*ast.StructType); ok {
				names = append(names, specification.Name.Name)
			}
		}
	}
	return names
}

func compassClassifiedStructNames() []string {
	contract := reflect.TypeFor[compassContractInventory]()
	names := make([]string, contract.NumField())
	for index := range contract.NumField() {
		names[index] = contract.Field(index).Name
	}
	slices.Sort(names)
	return names
}
