package version

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

type versionProtocolFact[T any] struct{}

// versionContractInventory classifies the only two release identities. A
// release is derived from Compass; a tag is derived from that release.
type versionContractInventory struct {
	Release versionProtocolFact[Release]
	Tag     versionProtocolFact[Tag]
}

//go:embed *.go
var versionSource embed.FS

func TestVersionProductionStructsHaveCompilerVisibleDataFlowRoles(t *testing.T) {
	t.Parallel()

	got, gotErr := versionProductionStructNames()
	if gotErr != nil {
		t.Fatalf("versionProductionStructNames() error = %v, want nil", gotErr)
	}
	want := versionClassifiedStructNames()
	if !slices.Equal(got, want) {
		t.Fatalf("Version production structs = %q, want classified %q", got, want)
	}
}

func versionProductionStructNames() ([]string, error) {
	names, err := fs.Glob(versionSource, "*.go")
	if err != nil {
		return nil, err
	}
	var structs []string
	for _, name := range names {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		source, err := versionSource.ReadFile(name)
		if err != nil {
			return nil, err
		}
		file, err := parser.ParseFile(token.NewFileSet(), name, source, parser.SkipObjectResolution)
		if err != nil {
			return nil, err
		}
		structs = append(structs, versionStructNames(file)...)
	}
	slices.Sort(structs)
	return structs, nil
}

func versionStructNames(file *ast.File) []string {
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

func versionClassifiedStructNames() []string {
	contract := reflect.TypeFor[versionContractInventory]()
	names := make([]string, contract.NumField())
	for index := range contract.NumField() {
		names[index] = contract.Field(index).Name
	}
	slices.Sort(names)
	return names
}
