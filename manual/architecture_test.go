package manual

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
var manualContractSources embed.FS

type (
	protocolFact[T any]      struct{}
	machineProjection[T any] struct{}
	capabilityWrapper[T any] struct{}
)

type manualContractInventory struct {
	Definition    protocolFact[Definition]
	Outcome       protocolFact[Outcome]
	Page          protocolFact[Page[renderTopic]]
	Book          protocolFact[Book[renderTopic]]
	Selection     protocolFact[Selection[renderTopic]]
	RenderRequest protocolFact[RenderRequest[renderTopic]]
	Report        machineProjection[Report]
	PageReport    machineProjection[PageReport]
	exactWriter   capabilityWrapper[exactWriter]
}

func TestManualDataFlowStructInventoryRatchet(t *testing.T) {
	t.Parallel()

	got := manualProductionStructNames(t)
	want := manualClassifiedStructNames(t)
	if !slices.Equal(got, want) {
		t.Fatalf("Manual production structs = %q, want classified %q", got, want)
	}
}

func manualProductionStructNames(t *testing.T) []string {
	t.Helper()

	entries, err := manualContractSources.ReadDir(".")
	if err != nil {
		t.Fatalf("manualContractSources.ReadDir(.) error = %v, want nil", err)
	}
	names := make([]string, 0)
	files := token.NewFileSet()
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		source, readErr := manualContractSources.ReadFile(entry.Name())
		if readErr != nil {
			t.Fatalf("manualContractSources.ReadFile(%q) error = %v, want nil", entry.Name(), readErr)
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

func manualClassifiedStructNames(t *testing.T) []string {
	t.Helper()

	source, err := manualContractSources.ReadFile("architecture_test.go")
	if err != nil {
		t.Fatalf("manualContractSources.ReadFile(architecture_test.go) error = %v, want nil", err)
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
			if specification.Name.Name != "manualContractInventory" {
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
	t.Fatal("manualContractInventory declarations found = 0, want 1")
	return nil
}

var (
	_ = manualContractInventory{}
	_ = manualContractInventory{}.exactWriter
)
