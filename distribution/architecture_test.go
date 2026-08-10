package distribution

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

type (
	protocolFact[T any]      struct{}
	executionIngress[T any]  struct{}
	capabilityWrapper[T any] struct{}
	internalFlow[T any]      struct{}
)

type distributionContractInventory struct {
	RequestCommitment                          protocolFact[RequestCommitment]
	requestCommitmentWire                      internalFlow[requestCommitmentWire]
	PublicationRequestPayload                  protocolFact[PublicationRequestPayload]
	PublicationRequestDocument                 protocolFact[PublicationRequestDocument]
	PublicationRequestIssuance                 executionIngress[PublicationRequestIssuance]
	PublicationRequestVerification             executionIngress[PublicationRequestVerification]
	VerifiedPublicationRequest                 capabilityWrapper[VerifiedPublicationRequest]
	PublicationSource                          executionIngress[PublicationSource]
	PublicationPlanRequest                     executionIngress[PublicationPlanRequest]
	PublicationGrantPayload                    protocolFact[PublicationGrantPayload]
	PublicationGrantDocument                   capabilityWrapper[PublicationGrantDocument]
	PublicationGrantProjection                 capabilityWrapper[PublicationGrantProjection]
	PublicationGrantIssuance                   executionIngress[PublicationGrantIssuance]
	PublicationGrantExpectation                executionIngress[PublicationGrantExpectation]
	VerifiedPublicationGrant                   capabilityWrapper[VerifiedPublicationGrant]
	publicationGrantDocumentWire               internalFlow[publicationGrantDocumentWire]
	publicationGrantProjectionWire             internalFlow[publicationGrantProjectionWire]
	PublicationCompletionPayload               protocolFact[PublicationCompletionPayload]
	PublicationCompletionDocument              protocolFact[PublicationCompletionDocument]
	PublicationCompletionProjection            capabilityWrapper[PublicationCompletionProjection]
	PublicationCompletionIssuance              executionIngress[PublicationCompletionIssuance]
	PublicationCompletionExpectation           executionIngress[PublicationCompletionExpectation]
	VerifiedPublicationCompletion              capabilityWrapper[VerifiedPublicationCompletion]
	publicationCompletionProjectionPayload     internalFlow[publicationCompletionProjectionPayload]
	publicationCompletionProjectionPayloadWire internalFlow[publicationCompletionProjectionPayloadWire]
	publicationCompletionProjectionWire        internalFlow[publicationCompletionProjectionWire]
	UpdateRequestPayload                       protocolFact[UpdateRequestPayload]
	UpdateRequestDocument                      protocolFact[UpdateRequestDocument]
	UpdateRequestIssuance                      executionIngress[UpdateRequestIssuance]
	UpdateRequestVerification                  executionIngress[UpdateRequestVerification]
	VerifiedUpdateRequest                      capabilityWrapper[VerifiedUpdateRequest]
	UpdateResponsePayload                      protocolFact[UpdateResponsePayload]
	UpdateResponseDocument                     protocolFact[UpdateResponseDocument]
	UpdateResponseIssuance                     executionIngress[UpdateResponseIssuance]
	UpdateResponseVerification                 executionIngress[UpdateResponseVerification]
	VerifiedUpdateResponse                     capabilityWrapper[VerifiedUpdateResponse]
	UpgradeRequestPayload                      protocolFact[UpgradeRequestPayload]
	UpgradeRequestDocument                     protocolFact[UpgradeRequestDocument]
	UpgradeRequestIssuance                     executionIngress[UpgradeRequestIssuance]
	UpgradeRequestVerification                 executionIngress[UpgradeRequestVerification]
	VerifiedUpgradeRequest                     capabilityWrapper[VerifiedUpgradeRequest]
	UpgradeGrantPayload                        protocolFact[UpgradeGrantPayload]
	UpgradeGrantDocument                       capabilityWrapper[UpgradeGrantDocument]
	UpgradeGrantProjection                     capabilityWrapper[UpgradeGrantProjection]
	UpgradeGrantIssuance                       executionIngress[UpgradeGrantIssuance]
	UpgradeGrantExpectation                    executionIngress[UpgradeGrantExpectation]
	VerifiedUpgradeGrant                       capabilityWrapper[VerifiedUpgradeGrant]
	UpgradeStageRequest                        executionIngress[UpgradeStageRequest]
	upgradeGrantDocumentWire                   internalFlow[upgradeGrantDocumentWire]
	upgradeGrantProjectionWire                 internalFlow[upgradeGrantProjectionWire]
}

var (
	_ distributionContractInventory
	_ = distributionContractInventory{}.requestCommitmentWire
	_ = distributionContractInventory{}.publicationGrantDocumentWire
	_ = distributionContractInventory{}.publicationGrantProjectionWire
	_ = distributionContractInventory{}.publicationCompletionProjectionPayload
	_ = distributionContractInventory{}.publicationCompletionProjectionPayloadWire
	_ = distributionContractInventory{}.publicationCompletionProjectionWire
	_ = distributionContractInventory{}.upgradeGrantDocumentWire
	_ = distributionContractInventory{}.upgradeGrantProjectionWire
)

func TestProductionStructsHaveCompilerVisibleDataFlowRoles(t *testing.T) {
	t.Parallel()

	got, err := distributionProductionStructNames()
	if err != nil {
		t.Fatalf("distributionProductionStructNames() error = %v, want nil", err)
	}
	want, err := distributionInventoryStructNames()
	if err != nil {
		t.Fatalf("distributionInventoryStructNames() error = %v, want nil", err)
	}
	if !slices.Equal(got, want) {
		t.Fatalf("production struct inventory = %v, want exact compiler-visible roles %v", got, want)
	}
}

func TestPublicOperationsAreExactDistributionIntent(t *testing.T) {
	t.Parallel()

	got, err := distributionExportedFunctionNames()
	if err != nil {
		t.Fatalf("distributionExportedFunctionNames() error = %v, want nil", err)
	}
	want := []string{
		"CommitRequest",
		"IssuePublicationCompletion",
		"IssuePublicationGrant",
		"IssuePublicationRequest",
		"IssueUpdateRequest",
		"IssueUpdateResponse",
		"IssueUpgradeGrant",
		"IssueUpgradeRequest",
		"ParseSigningDomain",
		"PreparePublicationPlan",
		"PrepareUpgradeStage",
		"VerifyPublicationCompletion",
		"VerifyPublicationGrant",
		"VerifyPublicationRequest",
		"VerifyUpdateRequest",
		"VerifyUpdateResponse",
		"VerifyUpgradeGrant",
		"VerifyUpgradeRequest",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("exported top-level functions = %v, want %v", got, want)
	}
}

func distributionProductionStructNames() ([]string, error) {
	files, err := distributionProductionFiles()
	if err != nil {
		return nil, err
	}
	set := token.NewFileSet()
	var names []string
	for _, name := range files {
		file, err := parser.ParseFile(set, name, nil, parser.SkipObjectResolution)
		if err != nil {
			return nil, err
		}
		for _, declaration := range file.Decls {
			generic, ok := declaration.(*ast.GenDecl)
			if !ok || generic.Tok != token.TYPE {
				continue
			}
			for _, raw := range generic.Specs {
				specification := raw.(*ast.TypeSpec)
				if _, ok := specification.Type.(*ast.StructType); ok {
					names = append(names, specification.Name.Name)
				}
			}
		}
	}
	slices.Sort(names)
	return names, nil
}

func distributionInventoryStructNames() ([]string, error) {
	set := token.NewFileSet()
	file, err := parser.ParseFile(set, "architecture_test.go", nil, parser.SkipObjectResolution)
	if err != nil {
		return nil, err
	}
	for _, declaration := range file.Decls {
		generic, ok := declaration.(*ast.GenDecl)
		if !ok || generic.Tok != token.TYPE {
			continue
		}
		for _, raw := range generic.Specs {
			specification := raw.(*ast.TypeSpec)
			if specification.Name.Name != "distributionContractInventory" {
				continue
			}
			structure := specification.Type.(*ast.StructType)
			var names []string
			for _, field := range structure.Fields.List {
				for _, name := range field.Names {
					names = append(names, name.Name)
				}
			}
			slices.Sort(names)
			return names, nil
		}
	}
	return nil, core.ErrDistributionContract
}

func distributionExportedFunctionNames() ([]string, error) {
	files, err := distributionProductionFiles()
	if err != nil {
		return nil, err
	}
	set := token.NewFileSet()
	var names []string
	for _, name := range files {
		file, err := parser.ParseFile(set, name, nil, parser.SkipObjectResolution)
		if err != nil {
			return nil, err
		}
		for _, declaration := range file.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if ok && function.Recv == nil && function.Name.IsExported() {
				names = append(names, function.Name.Name)
			}
		}
	}
	slices.Sort(names)
	return names, nil
}

func distributionProductionFiles() ([]string, error) {
	entries, err := os.ReadDir(".")
	if err != nil {
		return nil, err
	}
	var names []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".go") &&
			!strings.HasSuffix(entry.Name(), "_test.go") {
			names = append(names, entry.Name())
		}
	}
	slices.Sort(names)
	return names, nil
}
