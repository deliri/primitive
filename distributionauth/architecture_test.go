package distributionauth

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
var contractSources embed.FS

type (
	protocolFact[T any]      struct{}
	capabilityWrapper[T any] struct{}
)

type contractInventory struct {
	MaterialResponseIssuance                  protocolFact[MaterialResponseIssuance]
	MaterialResponseVerification              protocolFact[MaterialResponseVerification]
	PublicationRequestDocument                protocolFact[PublicationRequestDocument]
	PublicationRequestAssembly                protocolFact[PublicationRequestAssembly]
	PublicationVerification                   protocolFact[PublicationVerification]
	VerifiedPublication                       capabilityWrapper[VerifiedPublication]
	PublicationCompletionDocument             protocolFact[PublicationCompletionDocument]
	PublicationCompletionProjection           protocolFact[PublicationCompletionProjection]
	PublicationCompletionAssembly             protocolFact[PublicationCompletionAssembly]
	PublicationCompletionProjectionAssembly   protocolFact[PublicationCompletionProjectionAssembly]
	PublicationCompletionVerification         protocolFact[PublicationCompletionVerification]
	VerifiedPublicationCompletion             capabilityWrapper[VerifiedPublicationCompletion]
	UpdateRequestDocument                     protocolFact[UpdateRequestDocument]
	UpdateRequestAssembly                     protocolFact[UpdateRequestAssembly]
	UpdateVerification                        protocolFact[UpdateVerification]
	VerifiedUpdate                            capabilityWrapper[VerifiedUpdate]
	UpgradeRequestDocument                    protocolFact[UpgradeRequestDocument]
	UpgradeRequestAssembly                    protocolFact[UpgradeRequestAssembly]
	UpgradeVerification                       protocolFact[UpgradeVerification]
	VerifiedUpgrade                           capabilityWrapper[VerifiedUpgrade]
	PublicationResponseIssuance               protocolFact[PublicationResponseIssuance]
	PublicationResponseVerification           protocolFact[PublicationResponseVerification]
	PublicationCompletionResponseIssuance     protocolFact[PublicationCompletionResponseIssuance]
	PublicationCompletionResponseVerification protocolFact[PublicationCompletionResponseVerification]
	UpdateResponseIssuance                    protocolFact[UpdateResponseIssuance]
	UpdateResponseVerification                protocolFact[UpdateResponseVerification]
	UpgradeResponseIssuance                   protocolFact[UpgradeResponseIssuance]
	UpgradeResponseVerification               protocolFact[UpgradeResponseVerification]
}

func TestDistributionAuthDataFlowStructInventoryRatchet(t *testing.T) {
	t.Parallel()

	got := productionStructNames(t)
	want := classifiedStructNames(t)
	if !slices.Equal(got, want) {
		t.Fatalf("Distributionauth production structs = %q, want classified %q", got, want)
	}
}

func productionStructNames(t *testing.T) []string {
	t.Helper()

	entries, err := contractSources.ReadDir(".")
	if err != nil {
		t.Fatalf("contractSources.ReadDir(.) error = %v, want nil", err)
	}
	names := make([]string, 0)
	files := token.NewFileSet()
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		source, err := contractSources.ReadFile(entry.Name())
		if err != nil {
			t.Fatalf("contractSources.ReadFile(%q) error = %v, want nil", entry.Name(), err)
		}
		file, err := parser.ParseFile(files, entry.Name(), source, parser.SkipObjectResolution)
		if err != nil {
			t.Fatalf("parser.ParseFile(%q) error = %v, want nil", entry.Name(), err)
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

func classifiedStructNames(t *testing.T) []string {
	t.Helper()

	source, err := contractSources.ReadFile("architecture_test.go")
	if err != nil {
		t.Fatalf("contractSources.ReadFile(architecture_test.go) error = %v, want nil", err)
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
			if specification.Name.Name != "contractInventory" {
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
	t.Fatal("contractInventory declarations found = 0, want 1")
	return nil
}

var _ = contractInventory{}
