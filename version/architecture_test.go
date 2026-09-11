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

// The token bound must dominate the materializing core decoder. Runtime
// round trips cannot prove this allocation-order contract.
func TestTagJSONNominalBoundPrecedesMaterialization(t *testing.T) {
	t.Parallel()
	source, err := versionSource.ReadFile("tag.go")
	if err != nil {
		t.Fatal(err)
	}
	file, err := parser.ParseFile(token.NewFileSet(), "tag.go", source, parser.SkipObjectResolution)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "UnmarshalJSON" {
			continue
		}
		bounded := false
		for _, statement := range fn.Body.List {
			if branch, ok := statement.(*ast.IfStmt); ok {
				comparison, ok := branch.Cond.(*ast.BinaryExpr)
				if ok && comparison.Op == token.GTR {
					limit, ok := comparison.Y.(*ast.Ident)
					call, callOK := comparison.X.(*ast.CallExpr)
					if ok && limit.Name == "tagJSONTokenMaximumBytes" && callOK && len(call.Args) == 1 {
						name, nameOK := call.Fun.(*ast.Ident)
						arg, argOK := call.Args[0].(*ast.Ident)
						if nameOK && name.Name == "len" && argOK && arg.Name == "token" && len(branch.Body.List) > 0 {
							_, bounded = branch.Body.List[len(branch.Body.List)-1].(*ast.ReturnStmt)
						}
					}
				}
			}
			ast.Inspect(statement, func(node ast.Node) bool {
				call, ok := node.(*ast.CallExpr)
				if !ok {
					return true
				}
				selector, ok := call.Fun.(*ast.SelectorExpr)
				if !ok || selector.Sel.Name != "DecodeJSONStringToken" {
					return true
				}
				found = true
				if !bounded {
					t.Error("JSON decoder bound = absent, want nominal rejection before materialization")
				}
				if len(call.Args) != 1 {
					t.Fatalf("decoder argument count = %d, want 1", len(call.Args))
				}
				argument, ok := call.Args[0].(*ast.Ident)
				if !ok || argument.Name != "token" {
					t.Error("JSON decoder input = unbounded source, want bounded token")
				}
				return true
			})
		}
	}
	if !found {
		t.Error("JSON decoder calls = 0, want shared core decoder")
	}
}
