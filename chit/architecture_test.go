package chit

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
var chitContractSources embed.FS

type (
	protocolFact[T any]      struct{}
	streamingState[T any]    struct{}
	capabilityWrapper[T any] struct{}
)

type chitContractInventory struct {
	EntryName           protocolFact[EntryName]
	ChitID              protocolFact[ChitID]
	CollectionID        protocolFact[CollectionID]
	Version             protocolFact[Version]
	Payload             protocolFact[Payload]
	Document            protocolFact[Document]
	Issuance            protocolFact[Issuance]
	Expectation         protocolFact[Expectation]
	Verification        protocolFact[Verification]
	Verified            capabilityWrapper[Verified]
	ObjectCount         protocolFact[ObjectCount]
	EntrySequence       protocolFact[EntrySequence]
	ManifestEntry       protocolFact[ManifestEntry]
	ManifestAddition    protocolFact[ManifestAddition]
	ManifestDigest      protocolFact[ManifestDigest]
	ManifestSummary     protocolFact[ManifestSummary]
	ManifestAccumulator streamingState[ManifestAccumulator]
	CatalogEntry        protocolFact[CatalogEntry]
	Cursor              protocolFact[Cursor]
	Continuation        protocolFact[Continuation]
	CatalogPayload      protocolFact[CatalogPayload]
	CatalogDocument     protocolFact[CatalogDocument]
	CatalogIssuance     protocolFact[CatalogIssuance]
	CatalogVerification protocolFact[CatalogVerification]
	Selection           protocolFact[Selection]
	Position            protocolFact[Position]
	Query               protocolFact[Query]
	QueryRequest        protocolFact[QueryRequest]
}

func TestChitDataFlowStructInventoryRatchet(t *testing.T) {
	t.Parallel()

	got := chitProductionStructNames(t)
	want := chitClassifiedStructNames(t)
	if !slices.Equal(got, want) {
		t.Fatalf("Chit production structs = %q, want classified %q", got, want)
	}
}

func chitProductionStructNames(t *testing.T) []string {
	t.Helper()

	entries, err := chitContractSources.ReadDir(".")
	if err != nil {
		t.Fatalf("chitContractSources.ReadDir(.) error = %v, want nil", err)
	}
	names := make([]string, 0)
	files := token.NewFileSet()
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		source, readErr := chitContractSources.ReadFile(entry.Name())
		if readErr != nil {
			t.Fatalf("chitContractSources.ReadFile(%q) error = %v, want nil", entry.Name(), readErr)
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

func chitClassifiedStructNames(t *testing.T) []string {
	t.Helper()

	source, err := chitContractSources.ReadFile("architecture_test.go")
	if err != nil {
		t.Fatalf("chitContractSources.ReadFile(architecture_test.go) error = %v, want nil", err)
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
			if specification.Name.Name != "chitContractInventory" {
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
	t.Fatal("chitContractInventory declarations found = 0, want 1")
	return nil
}

var _ = chitContractInventory{}
