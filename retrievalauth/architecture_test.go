package retrievalauth

import (
	"embed"
	"go/ast"
	"go/parser"
	"go/token"
	"slices"
	"sort"
	"strings"
	"testing"
)

//go:embed *.go
var retrievalAuthContractSources embed.FS

type (
	protocolFact[T any]      struct{}
	capabilityWrapper[T any] struct{}
)

type retrievalAuthContractInventory struct {
	RequestDocument      protocolFact[RequestDocument]
	RequestAssembly      protocolFact[RequestAssembly]
	Verification         protocolFact[Verification]
	Verified             capabilityWrapper[Verified]
	ResponseIssuance     protocolFact[ResponseIssuance]
	ResponseVerification protocolFact[ResponseVerification]
}

func TestRetrievalAuthDataFlowStructInventoryRatchet(t *testing.T) {
	t.Parallel()

	got := retrievalAuthProductionStructNames(t)
	want := retrievalAuthClassifiedStructNames(t)
	if !slices.Equal(got, want) {
		t.Fatalf("Retrievalauth production structs = %q, want classified %q", got, want)
	}
}

func retrievalAuthProductionStructNames(t *testing.T) []string {
	t.Helper()

	entries, err := retrievalAuthContractSources.ReadDir(".")
	if err != nil {
		t.Fatalf("retrievalAuthContractSources.ReadDir(.) error = %v, want nil", err)
	}
	names := make([]string, 0)
	files := token.NewFileSet()
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		source, readErr := retrievalAuthContractSources.ReadFile(entry.Name())
		if readErr != nil {
			t.Fatalf("retrievalAuthContractSources.ReadFile(%q) error = %v, want nil", entry.Name(), readErr)
		}
		file, parseErr := parser.ParseFile(files, entry.Name(), source, parser.SkipObjectResolution)
		if parseErr != nil {
			t.Fatalf("parser.ParseFile(%q) error = %v, want nil", entry.Name(), parseErr)
		}
		ast.Inspect(file, func(node ast.Node) bool {
			specification, ok := node.(*ast.TypeSpec)
			if !ok {
				return true
			}
			if _, ok := specification.Type.(*ast.StructType); ok {
				names = append(names, specification.Name.Name)
			}
			return true
		})
	}
	sort.Strings(names)
	return names
}

func retrievalAuthClassifiedStructNames(t *testing.T) []string {
	t.Helper()

	source, err := retrievalAuthContractSources.ReadFile("architecture_test.go")
	if err != nil {
		t.Fatalf("retrievalAuthContractSources.ReadFile(architecture_test.go) error = %v, want nil", err)
	}
	file, err := parser.ParseFile(token.NewFileSet(), "architecture_test.go", source, parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("parser.ParseFile(architecture_test.go) error = %v, want nil", err)
	}
	for _, declaration := range file.Decls {
		generic, ok := declaration.(*ast.GenDecl)
		if !ok || generic.Tok != token.TYPE {
			continue
		}
		for _, raw := range generic.Specs {
			specification := raw.(*ast.TypeSpec)
			if specification.Name.Name != "retrievalAuthContractInventory" {
				continue
			}
			structure := specification.Type.(*ast.StructType)
			names := make([]string, 0, len(structure.Fields.List))
			for _, field := range structure.Fields.List {
				for _, name := range field.Names {
					names = append(names, name.Name)
				}
			}
			sort.Strings(names)
			return names
		}
	}
	t.Fatal("retrievalAuthContractInventory declarations found = 0, want 1")
	return nil
}

var _ = retrievalAuthContractInventory{}
