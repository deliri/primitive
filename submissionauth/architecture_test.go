package submissionauth

import (
	"embed"
	"github.com/deliri/primitive/v2026/core"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"slices"
	"sort"
	"strings"
	"testing"
)

//go:embed *.go
var authTestSources embed.FS

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

	entries, err := fs.ReadDir(authTestSources, ".")
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
			fileSet, entry.Name(), authSource(t, entry.Name()), parser.SkipObjectResolution,
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
		token.NewFileSet(), "architecture_test.go", authSource(t, "architecture_test.go"), parser.SkipObjectResolution,
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
			specification, ok := raw.(*ast.TypeSpec)
			if !ok {
				continue
			}
			if specification.Name.Name != "submissionAuthContractInventory" {
				continue
			}
			structure, ok := specification.Type.(*ast.StructType)
			if !ok {
				t.Fatalf("inventory type = %T, want *ast.StructType", specification.Type)
			}
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

func authSource(t *testing.T, name string) []byte {
	t.Helper()
	data, err := authTestSources.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

type authIngressDoors struct {
	RequestJSON    func(*RequestDocument, []byte) error
	CompletionJSON func(*CompletionDocument, []byte) error
	ProjectionJSON func(CompletionProjection, []byte, core.StrictJSONLimits) error
}

var authIngress = authIngressDoors{
	RequestJSON:    (*RequestDocument).UnmarshalJSON,
	CompletionJSON: (*CompletionDocument).UnmarshalJSON,
	ProjectionJSON: CompletionProjection.ValidateJSONProjection,
}
var authIngressFuzz = struct {
	Request            func(*testing.F)
	Completion         func(*testing.F)
	Projection         func(*testing.F)
	SubmissionResponse func(*testing.F)
	CompletionResponse func(*testing.F)
}{
	FuzzCredentialedRequestJSONSemanticAndAuthorityClosure,
	FuzzCredentialedCompletionJSONSemanticAndAuthorityClosure,
	FuzzCredentialedCompletionProjectionValidateJSONProjectionOracle,
	FuzzSubmissionResponseAuthorityClosure,
	FuzzCompletionResponseAuthorityClosure,
}

func TestSubmissionAuthDecoderInventoryRatchet(t *testing.T) {
	_ = authIngress
	_ = authIngressFuzz
	t.Parallel()
	var got []string
	entries, err := fs.ReadDir(authTestSources, ".")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), entry.Name(), authSource(t, entry.Name()), parser.SkipObjectResolution)
		if err != nil {
			t.Fatal(err)
		}
		for _, declaration := range file.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok || function.Recv == nil {
				continue
			}
			if function.Name.Name != "UnmarshalJSON" && function.Name.Name != "ValidateJSONProjection" {
				continue
			}
			expression := function.Recv.List[0].Type
			if pointer, ok := expression.(*ast.StarExpr); ok {
				expression = pointer.X
			}
			receiver, ok := expression.(*ast.Ident)
			if !ok {
				t.Fatalf("decoder receiver = %T, want *ast.Ident", expression)
			}
			got = append(got, receiver.Name+"."+function.Name.Name)
		}
	}
	// These are symbol coordinates for the compiler-bound doors above, not protocol strings.
	want := []string{"CompletionDocument.UnmarshalJSON", "CompletionProjection.ValidateJSONProjection", "RequestDocument.UnmarshalJSON"}
	slices.Sort(got)
	if !slices.Equal(got, want) {
		t.Fatalf("external JSON doors=%v, want fuzz-bound %v", got, want)
	}
}
