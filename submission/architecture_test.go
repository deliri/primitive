package submission

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"
)

type (
	protocolFact[T any]         struct{}
	sealedWireProjection[T any] struct{}
	capabilityWrapper[T any]    struct{}
)

// submissionContractInventory classifies every production struct by its exact
// role at the evidence-agreement boundary. Field names deliberately equal the
// compiler-owned type names so an added carrier cannot evade review.
type submissionContractInventory struct {
	Declaration         protocolFact[Declaration]
	RequestPayload      protocolFact[RequestPayload]
	RequestDocument     protocolFact[RequestDocument]
	RequestIssuance     protocolFact[RequestIssuance]
	RequestVerification protocolFact[RequestVerification]
	RequestCommitment   protocolFact[RequestCommitment]
	AuthorizationNonce  protocolFact[AuthorizationNonce]
	GrantPayload        protocolFact[GrantPayload]
	GrantDocument       protocolFact[GrantDocument]
	GrantProjection     protocolFact[GrantProjection]
	GrantIssuance       protocolFact[GrantIssuance]
	GrantExpectation    protocolFact[GrantExpectation]
	VerifiedRequest     capabilityWrapper[VerifiedRequest]
	VerifiedGrant       capabilityWrapper[VerifiedGrant]
	grantDocumentWire   sealedWireProjection[grantDocumentWire]
	grantProjectionWire sealedWireProjection[grantProjectionWire]
}

func TestSubmissionDataFlowStructInventoryRatchet(t *testing.T) {
	t.Parallel()

	got := submissionProductionStructNames(t)
	want := submissionClassifiedStructNames(t)
	if !slices.Equal(got, want) {
		t.Fatalf("Submission production structs = %q, want classified %q", got, want)
	}
}

func submissionProductionStructNames(t *testing.T) []string {
	t.Helper()

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("os.ReadDir(.) error = %v, want nil", err)
	}
	names := make([]string, 0)
	fileSet := token.NewFileSet()
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") ||
			strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		file, parseErr := parser.ParseFile(
			fileSet, filepath.Clean(entry.Name()), nil, parser.SkipObjectResolution,
		)
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

func submissionClassifiedStructNames(t *testing.T) []string {
	t.Helper()

	file, err := parser.ParseFile(
		token.NewFileSet(), "architecture_test.go", nil, parser.SkipObjectResolution,
	)
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
			if specification.Name.Name != "submissionContractInventory" {
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
	t.Fatal("submissionContractInventory declarations found = 0, want 1")
	return nil
}

var (
	_ = submissionContractInventory{}
	_ = submissionContractInventory{}.grantDocumentWire
	_ = submissionContractInventory{}.grantProjectionWire
)
