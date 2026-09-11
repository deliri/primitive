package gomodule_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"slices"
	"strings"
	"testing"
)

func gomoduleSourceShape(file *ast.File) (structs, exported []string) {
	for _, declaration := range file.Decls {
		switch node := declaration.(type) {
		case *ast.GenDecl:
			for _, spec := range node.Specs {
				typeSpec, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				if _, ok := typeSpec.Type.(*ast.StructType); ok {
					structs = append(structs, typeSpec.Name.Name)
				}
			}
		case *ast.FuncDecl:
			if !node.Name.IsExported() {
				continue
			}
			name := node.Name.Name
			if node.Recv != nil {
				receiver := node.Recv.List[0].Type
				if pointer, ok := receiver.(*ast.StarExpr); ok {
					receiver = pointer.X
				}
				if identifier, ok := receiver.(*ast.Ident); ok {
					name = identifier.Name + "." + name
				}
			}
			exported = append(exported, name)
		}
	}
	return structs, exported
}

func TestPublicBoundaryAndStructInventory(t *testing.T) {
	t.Parallel()
	roles := map[string]string{
		"Path":       "sealed module identity; validated Go grammar, canonical scalar projection",
		"ImportPath": "sealed import identity; validated Go grammar including standard library names",
	}
	doors := map[string]string{
		"ParsePath":                "FuzzPathTextAdmission",
		"ParseImportPath":          "FuzzImportPathTextAdmission",
		"Path.UnmarshalJSON":       "FuzzPathJSONSemanticClosure",
		"ImportPath.UnmarshalJSON": "FuzzImportPathJSONSemanticClosure",
	}
	wantExports := []string{"ImportPath.MarshalJSON", "ImportPath.String", "ImportPath.UnmarshalJSON", "ImportPath.Validate", "ParseImportPath", "ParsePath", "Path.MarshalJSON", "Path.String", "Path.UnmarshalJSON", "Path.Validate"}
	files, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("ReadDir() error = %v, want nil", err)
	}
	var structs, exports, fuzz []string
	for _, entry := range files {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), entry.Name(), nil, 0)
		if err != nil {
			t.Fatalf("ParseFile(%q) error = %v, want nil", entry.Name(), err)
		}
		foundStructs, foundExports := gomoduleSourceShape(file)
		if strings.HasSuffix(entry.Name(), "_test.go") {
			for _, name := range foundExports {
				if strings.HasPrefix(name, "Fuzz") {
					fuzz = append(fuzz, name)
				}
			}
			continue
		}
		structs = append(structs, foundStructs...)
		exports = append(exports, foundExports...)
	}
	slices.Sort(structs)
	if !slices.Equal(structs, []string{"ImportPath", "Path"}) {
		t.Fatalf("production structs = %v, want classified ImportPath and Path", structs)
	}
	for _, name := range structs {
		if roles[name] == "" {
			t.Fatalf("struct %s role = %q, want intentional data-flow role", name, roles[name])
		}
	}
	slices.Sort(exports)
	if !slices.Equal(exports, wantExports) {
		t.Fatalf("public boundary = %v, want %v; classify each new external door", exports, wantExports)
	}
	for name, target := range doors {
		if !slices.Contains(exports, name) || !slices.Contains(fuzz, target) {
			t.Fatalf("external door %s fuzz target = %s; exports = %v, fuzz = %v, want both present", name, target, exports, fuzz)
		}
	}
}

func TestSourceInventoryMatcherFindsUnclassifiedCarriers(t *testing.T) {
	t.Parallel()
	file, err := parser.ParseFile(token.NewFileSet(), "fixture.go", `package fixture; type Carrier struct{}; func (*Carrier) Decode() {}; func Parse() {}; func private() {}`, 0)
	if err != nil {
		t.Fatalf("ParseFile(fixture) error = %v, want nil", err)
	}
	structs, exports := gomoduleSourceShape(file)
	if !slices.Equal(structs, []string{"Carrier"}) || !slices.Equal(exports, []string{"Carrier.Decode", "Parse"}) {
		t.Fatalf("source shape = (%v, %v), want (Carrier, Carrier.Decode and Parse)", structs, exports)
	}
}
