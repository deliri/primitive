package submissionauth

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
	authProtocolFact[T any]      struct{}
	authCapabilityWrapper[T any] struct{}
)

// submissionAuthContractInventory classifies every production struct by its
// exact role at the installation-authentication boundary.
type submissionAuthContractInventory struct {
	RequestDocument                 authProtocolFact[RequestDocument]
	RequestAssembly                 authProtocolFact[RequestAssembly]
	Verification                    authProtocolFact[Verification]
	Verified                        authCapabilityWrapper[Verified]
	CompletionDocument              authProtocolFact[CompletionDocument]
	CompletionProjection            authProtocolFact[CompletionProjection]
	CompletionAssembly              authProtocolFact[CompletionAssembly]
	CompletionProjectionAssembly    authProtocolFact[CompletionProjectionAssembly]
	CompletionVerification          authProtocolFact[CompletionVerification]
	VerifiedCompletion              authCapabilityWrapper[VerifiedCompletion]
	CompletionReconciliationRequest authProtocolFact[CompletionReconciliationRequest]
	ReconciledCompletion            authCapabilityWrapper[ReconciledCompletion]
	SubmissionResponseIssuance      authProtocolFact[SubmissionResponseIssuance]
	SubmissionResponseVerification  authProtocolFact[SubmissionResponseVerification]
	CompletionResponseIssuance      authProtocolFact[CompletionResponseIssuance]
	CompletionResponseVerification  authProtocolFact[CompletionResponseVerification]
}

func TestSubmissionAuthDataFlowStructInventoryRatchet(t *testing.T) {
	t.Parallel()

	got := submissionAuthProductionStructNames(t)
	want := submissionAuthClassifiedStructNames(t)
	if !slices.Equal(got, want) {
		t.Fatalf("Submissionauth production structs = %q, want classified %q", got, want)
	}
}

func submissionAuthProductionStructNames(t *testing.T) []string {
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

func submissionAuthClassifiedStructNames(t *testing.T) []string {
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
			if specification.Name.Name != "submissionAuthContractInventory" {
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
	t.Fatal("submissionAuthContractInventory declarations found = 0, want 1")
	return nil
}

var _ = submissionAuthContractInventory{}
