package fuzzartifact

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/token"
	"slices"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
	"github.com/deliri/primitive/v2026/hostfacts"
)

// productionStructRoles is the wiring ratchet. A new production struct fails
// here until it is classified, and a deleted one fails until it is removed.
func productionStructRoles() map[string]string {
	return map[string]string{
		"EntryCount":    "typed observation counter",
		"FindRequest":   "validated rooted ingress",
		"GeneratedName": "fixed generated-name fact",
		"Observation":   "constant-size scan accounting",
		"finder":        "synchronous enumeration flow",
	}
}

func TestProductionStructDataFlowInventory(t *testing.T) {
	t.Parallel()

	roles := productionStructRoles()
	for _, name := range productionStructNames(t) {
		role, classified := roles[name]
		if !classified || role == "" {
			t.Errorf("production struct %s has role %q classified %t, want an intentional data-flow role", name, role, classified)
			continue
		}
		delete(roles, name)
	}
	for name, role := range roles {
		t.Errorf("inventory classifies %s as %q, but the package declares no such production struct", name, role)
	}
}

func TestPublicOperationsAreExactIntentEntryPoints(t *testing.T) {
	t.Parallel()

	got := productionFunctionNames(t)
	want := []string{
		"Find",
		"ParseArtifactKind",
		"ParseGeneratedName",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("exported Fuzzartifact operations = %q, want exactly %q", got, want)
	}
}

func productionFunctionNames(t *testing.T) []string {
	t.Helper()

	var names []string
	for _, file := range productionFiles(t) {
		for _, declaration := range file.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok || function.Recv != nil || !ast.IsExported(function.Name.Name) {
				continue
			}
			names = append(names, function.Name.Name)
		}
	}
	slices.Sort(names)
	return names
}

func productionFiles(t *testing.T) []*ast.File {
	t.Helper()
	directory, err := hostfacts.WorkingDirectory()
	if err != nil {
		t.Fatal(err)
	}
	root := openRootForTest(t, directory.String())
	fileSet := token.NewFileSet()
	var files []*ast.File
	err = filestore.Walk(t.Context(), filestore.WalkRequest{
		Location: filestore.Location{Root: root, Path: relativePathForTest(t, ".")},
		Visit: func(entry filestore.WalkEntry) (filestore.WalkDirective, error) {
			if entry.Entry.IsDir() {
				return filestore.WalkSkipDirectory, nil
			}
			if !strings.HasSuffix(entry.Entry.Name(), ".go") || strings.HasSuffix(entry.Entry.Name(), "_test.go") {
				return filestore.WalkContinue, nil
			}
			var source bytes.Buffer
			if _, err := filestore.Read(t.Context(), filestore.ReadRequest{Location: filestore.Location{Root: root, Path: entry.Path}, Destination: &source}); err != nil {
				return filestore.WalkContinue, err
			}
			file, err := parser.ParseFile(fileSet, entry.Path.String(), source.Bytes(), parser.SkipObjectResolution)
			if err == nil {
				files = append(files, file)
			}
			return filestore.WalkContinue, err
		},
	})
	if err != nil || len(files) == 0 {
		t.Fatalf("production source scan count/error=%d/%v, want nonempty parsed source", len(files), err)
	}
	return files
}

func readSourceForTest(t *testing.T, path string) []byte {
	t.Helper()
	absolute, err := core.ParseAbsolutePath(path)
	if err != nil {
		t.Fatal(err)
	}
	location, err := filestore.OpenParent(t.Context(), absolute)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := location.Root.Close(); err != nil {
			t.Errorf("source root close=%v, want nil", err)
		}
	})
	var source bytes.Buffer
	if _, err := filestore.Read(t.Context(), filestore.ReadRequest{Location: location, Destination: &source}); err != nil {
		t.Fatal(err)
	}
	return source.Bytes()
}

func productionStructNames(t *testing.T) []string {
	t.Helper()

	var names []string
	for _, file := range productionFiles(t) {
		for _, declaration := range file.Decls {
			general, ok := declaration.(*ast.GenDecl)
			if !ok || general.Tok != token.TYPE {
				continue
			}
			for _, specification := range general.Specs {
				typed := specification.(*ast.TypeSpec)
				if _, ok := typed.Type.(*ast.StructType); ok {
					names = append(names, typed.Name.Name)
				}
			}
		}
	}
	slices.Sort(names)
	return names
}

func TestExternalIngressHasSemanticFuzzCoverage(t *testing.T) {
	t.Parallel()
	cases := []struct{ name, target string }{
		{name: "Find", target: "FuzzFindNativeDirectorySemanticClosure"},
		{name: "ParseGeneratedName", target: "FuzzParseGeneratedNameSemanticClosure"},
		{name: "ParseArtifactKind", target: "FuzzArtifactKindTextSemanticClosure"},
		{name: "UnmarshalJSON", target: "FuzzArtifactKindJSONSemanticClosure"},
	}
	directory, err := hostfacts.WorkingDirectory()
	if err != nil {
		t.Fatal(err)
	}
	path, err := directory.Resolve("ingress_fuzz_test.go")
	if err != nil {
		t.Fatal(err)
	}
	file, err := parser.ParseFile(token.NewFileSet(), path.String(), readSourceForTest(t, path.String()), parser.SkipObjectResolution)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			found := false
			for _, decl := range file.Decls {
				function, ok := decl.(*ast.FuncDecl)
				if !ok || function.Name.Name != tc.target {
					continue
				}
				ast.Inspect(function.Body, func(node ast.Node) bool {
					call, ok := node.(*ast.CallExpr)
					if !ok {
						return true
					}
					switch fun := call.Fun.(type) {
					case *ast.Ident:
						if fun.Name == tc.name {
							found = true
						}
					case *ast.SelectorExpr:
						if fun.Sel.Name == tc.name {
							found = true
						}
					}
					return true
				})
			}
			if !found {
				t.Fatalf("external door %s target %s calls production=%v, want true", tc.name, tc.target, found)
			}
		})
	}
}
