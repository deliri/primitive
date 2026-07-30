package fuzzfinder

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"slices"
	"strings"
	"testing"
)

// productionStructRoles is the wiring ratchet. A new production struct fails
// here until it is classified, and a deleted one fails until it is removed.
func productionStructRoles() map[string]string {
	return map[string]string{
		"EntryCount":     "typed observation counter",
		"FindRequest":    "validated rooted ingress",
		"GeneratedName":  "fixed generated-name fact",
		"Observation":    "bounded external result",
		"RetentionLimit": "validated execution policy",
		"finder":         "bounded internal flow",
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
		"NewRetentionLimit",
		"ParseArtifactKind",
		"ParseGeneratedName",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("exported Fuzzfinder operations = %q, want exactly %q", got, want)
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

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("os.ReadDir(package directory) error = %v, want nil", err)
	}
	fileSet := token.NewFileSet()
	var files []*ast.File
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		file, parseErr := parser.ParseFile(fileSet, entry.Name(), nil, parser.SkipObjectResolution)
		if parseErr != nil {
			t.Fatalf("parser.ParseFile(%q) error = %v, want nil", entry.Name(), parseErr)
		}
		files = append(files, file)
	}
	if len(files) == 0 {
		t.Fatalf("production file count = %d, want > 0", len(files))
	}
	return files
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
