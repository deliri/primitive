package receipt

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"reflect"
	"slices"
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

func TestReceiptExternalIngressFuzzInventoryMatchesProduction(t *testing.T) {
	t.Parallel()

	gotJSON, err := receiptExportedJSONReceiverNames()
	if err != nil {
		t.Fatalf("receiptExportedJSONReceiverNames() error = %v, want nil", err)
	}
	var wantJSON []string
	for door := receiptJSONDoorUnknown + 1; door < receiptJSONDoorLimit; door++ {
		name := door.receiverName()
		if name == "" {
			t.Fatalf("receipt JSON fuzz door %d has no compiler-visible receiver", door)
		}
		wantJSON = append(wantJSON, name)
	}
	slices.Sort(wantJSON)
	if !slices.Equal(gotJSON, wantJSON) {
		t.Fatalf("public JSON receivers = %v, fuzz inventory = %v", gotJSON, wantJSON)
	}

	gotText := []string{
		"ParseAccountIdentity", "ParseObjectIdentity",
		"ParseReceiptID", "ParseSubmissionIdentity",
	}
	var wantText []string
	for door := receiptTextDoorUnknown + 1; door < receiptTextDoorLimit; door++ {
		name := door.functionName()
		if name == "" {
			t.Fatalf("receipt text fuzz door %d has no compiler-visible function", door)
		}
		wantText = append(wantText, name)
	}
	slices.Sort(wantText)
	if !slices.Equal(gotText, wantText) {
		t.Fatalf("public text parsers = %v, fuzz inventory = %v", gotText, wantText)
	}
}

func receiptExportedJSONReceiverNames() ([]string, error) {
	files, err := os.ReadDir(".")
	if err != nil {
		return nil, err
	}
	var names []string
	fileSet := token.NewFileSet()
	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".go") ||
			strings.HasSuffix(file.Name(), "_test.go") {
			continue
		}
		parsed, parseErr := parser.ParseFile(fileSet, file.Name(), nil, parser.SkipObjectResolution)
		if parseErr != nil {
			return nil, parseErr
		}
		for _, declaration := range parsed.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok || function.Name.Name != "UnmarshalJSON" || function.Recv == nil ||
				len(function.Recv.List) != 1 {
				continue
			}
			pointer, ok := function.Recv.List[0].Type.(*ast.StarExpr)
			if !ok {
				continue
			}
			receiver, ok := pointer.X.(*ast.Ident)
			if ok && receiver.IsExported() {
				names = append(names, receiver.Name)
			}
		}
	}
	slices.Sort(names)
	return names, nil
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
		for field := range inventory.Fields() {
			found = found || gotName == receiptProductionStructName(field.Name)
		}
		if !found {
			t.Errorf("production struct %q has no compiler-visible data-flow role", gotName)
		}
	}
}
