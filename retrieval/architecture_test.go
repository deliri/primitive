package retrieval

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
var retrievalContractSources embed.FS

type (
	protocolFact[T any]         struct{}
	sealedWireProjection[T any] struct{}
	capabilityWrapper[T any]    struct{}
)

type retrievalContractInventory struct {
	Selection            protocolFact[Selection]
	RequestPayload       protocolFact[RequestPayload]
	RequestDocument      protocolFact[RequestDocument]
	RequestIssuance      protocolFact[RequestIssuance]
	RequestCommitment    protocolFact[RequestCommitment]
	GrantPayload         protocolFact[GrantPayload]
	GrantDocument        protocolFact[GrantDocument]
	GrantProjection      protocolFact[GrantProjection]
	GrantIssuance        protocolFact[GrantIssuance]
	GrantExpectation     protocolFact[GrantExpectation]
	GrantContinuation    protocolFact[GrantContinuation]
	selectionExpectation protocolFact[selectionExpectation]
	DownloadCallRequest  protocolFact[DownloadCallRequest]
	FileDownloadRequest  protocolFact[FileDownloadRequest]
	VerifiedGrant        capabilityWrapper[VerifiedGrant]
	grantProjectionWire  sealedWireProjection[grantProjectionWire]
}

func TestRetrievalDataFlowStructInventoryRatchet(t *testing.T) {
	t.Parallel()

	got := retrievalProductionStructNames(t)
	want := retrievalClassifiedStructNames(t)
	if !slices.Equal(got, want) {
		t.Fatalf("Retrieval production structs = %q, want classified %q", got, want)
	}
}

func retrievalProductionStructNames(t *testing.T) []string {
	t.Helper()

	entries, err := retrievalContractSources.ReadDir(".")
	if err != nil {
		t.Fatalf("retrievalContractSources.ReadDir(.) error = %v, want nil", err)
	}
	names := make([]string, 0)
	files := token.NewFileSet()
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		source, readErr := retrievalContractSources.ReadFile(entry.Name())
		if readErr != nil {
			t.Fatalf("retrievalContractSources.ReadFile(%q) error = %v, want nil", entry.Name(), readErr)
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

func retrievalClassifiedStructNames(t *testing.T) []string {
	t.Helper()

	source, err := retrievalContractSources.ReadFile("architecture_test.go")
	if err != nil {
		t.Fatalf("retrievalContractSources.ReadFile(architecture_test.go) error = %v, want nil", err)
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
			if specification.Name.Name != "retrievalContractInventory" {
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
	t.Fatal("retrievalContractInventory declarations found = 0, want 1")
	return nil
}

var (
	_ = retrievalContractInventory{}
	_ = retrievalContractInventory{}.grantProjectionWire
	_ = retrievalContractInventory{}.selectionExpectation
)
