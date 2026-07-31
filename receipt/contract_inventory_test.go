package receipt

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"reflect"
	"strings"
	"testing"
)

type (
	receiptProtocolFact[T any]     struct{}
	receiptInternalFlow[T any]     struct{}
	receiptRequestCarrier[T any]   struct{}
	receiptSealedProjection[T any] struct{}
	receiptFailureDetail[T any]    struct{}
	receiptProductionStructName    string
)

// receiptContractInventory classifies every production struct by its role.
type receiptContractInventory struct {
	scopeMismatch           receiptFailureDetail[scopeMismatch]
	watermarkConflict       receiptFailureDetail[watermarkConflict]
	lifecycleIdentity       receiptInternalFlow[lifecycleIdentity]
	AccountIdentity         receiptProtocolFact[AccountIdentity]
	OfferingIdentity        receiptProtocolFact[OfferingIdentity]
	SubmissionIdentity      receiptProtocolFact[SubmissionIdentity]
	ObjectIdentity          receiptProtocolFact[ObjectIdentity]
	ReceiptID               receiptProtocolFact[ReceiptID]
	Generation              receiptProtocolFact[Generation]
	jsonStructureContract   receiptInternalFlow[jsonStructureContract]
	EvidenceBody            receiptProtocolFact[EvidenceBody]
	Header                  receiptProtocolFact[Header]
	EvidencePayload         receiptProtocolFact[EvidencePayload]
	EvidenceDocument        receiptProtocolFact[EvidenceDocument]
	IssueEvidenceRequest    receiptRequestCarrier[IssueEvidenceRequest]
	EvidenceExpectation     receiptRequestCarrier[EvidenceExpectation]
	VerifyEvidenceRequest   receiptRequestCarrier[VerifyEvidenceRequest]
	VerifiedEvidence        receiptSealedProjection[VerifiedEvidence]
	evidenceBodyWire        receiptInternalFlow[evidenceBodyWire]
	headerWire              receiptInternalFlow[headerWire]
	payloadWire             receiptInternalFlow[payloadWire]
	documentWire            receiptInternalFlow[documentWire]
	CursorDigest            receiptProtocolFact[CursorDigest]
	ChainHash               receiptProtocolFact[ChainHash]
	Scope                   receiptProtocolFact[Scope]
	scopeWire               receiptInternalFlow[scopeWire]
	Watermark               receiptProtocolFact[Watermark]
	watermarkWire           receiptInternalFlow[watermarkWire]
	WatermarkRequest        receiptRequestCarrier[WatermarkRequest]
	AdvanceWatermarkRequest receiptRequestCarrier[AdvanceWatermarkRequest]
	AdvanceResult           receiptSealedProjection[AdvanceResult]
}

func TestReceiptProductionStructsHaveCompilerVisibleDataFlowRoles(t *testing.T) {
	t.Parallel()

	_ = receiptContractInventory{}.scopeMismatch
	_ = receiptContractInventory{}.watermarkConflict
	_ = receiptContractInventory{}.lifecycleIdentity
	_ = receiptContractInventory{}.jsonStructureContract
	_ = receiptContractInventory{}.evidenceBodyWire
	_ = receiptContractInventory{}.headerWire
	_ = receiptContractInventory{}.payloadWire
	_ = receiptContractInventory{}.documentWire
	_ = receiptContractInventory{}.scopeWire
	_ = receiptContractInventory{}.watermarkWire
	inventory := reflect.TypeFor[receiptContractInventory]()
	maximum := inventory.NumField()
	files, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("os.ReadDir() error = %v, want nil", err)
	}
	got := make([]receiptProductionStructName, maximum)
	var count int
	fileSet := token.NewFileSet()
	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".go") ||
			strings.HasSuffix(file.Name(), "_test.go") {
			continue
		}
		parsed, parseErr := parser.ParseFile(fileSet, file.Name(), nil, parser.SkipObjectResolution)
		if parseErr != nil {
			t.Fatalf("parser.ParseFile(%q) error = %v, want nil", file.Name(), parseErr)
		}
		for _, declaration := range parsed.Decls {
			generic, ok := declaration.(*ast.GenDecl)
			if !ok || generic.Tok != token.TYPE {
				continue
			}
			for _, raw := range generic.Specs {
				spec := raw.(*ast.TypeSpec)
				if _, ok := spec.Type.(*ast.StructType); !ok {
					continue
				}
				if count >= len(got) {
					t.Fatalf("production struct count exceeded ratchet %d", len(got))
				}
				got[count] = receiptProductionStructName(spec.Name.Name)
				count++
			}
		}
	}
	if count != maximum {
		t.Fatalf("production struct count = %d, want %d", count, maximum)
	}
	for _, gotName := range got {
		var found bool
		for index := range inventory.NumField() {
			found = found || gotName == receiptProductionStructName(inventory.Field(index).Name)
		}
		if !found {
			t.Errorf("production struct %q has no compiler-visible data-flow role", gotName)
		}
	}
}
