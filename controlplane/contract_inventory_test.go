package controlplane

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"slices"
	"strings"
	"testing"
)

type (
	controlplaneProtocolFact[T any]   struct{}
	controlplaneAuthorityInput[T any] struct{}
	controlplaneVerifiedProof[T any]  struct{}
	controlplaneIssuance[T any]       struct{}
	controlplaneInternalFlow[T any]   struct{}
	controlplaneTypedFailure[T any]   struct{}
	controlplaneProductionStructName  string
)

// controlplaneContractInventory classifies every production struct that moves,
// authenticates, signs, or projects control-plane data. The field name is the
// production type name, so the AST comparison below makes omissions fail.
type controlplaneContractInventory struct {
	RegistrationRequest               controlplaneProtocolFact[RegistrationRequest]
	RegistrationIdentity              controlplaneProtocolFact[RegistrationIdentity]
	InstallationCertificateBody       controlplaneProtocolFact[InstallationCertificateBody]
	InstallationCertificateDocument   controlplaneProtocolFact[InstallationCertificateDocument]
	RegistrationPayload               controlplaneProtocolFact[RegistrationPayload]
	RegistrationDocument              controlplaneProtocolFact[RegistrationDocument]
	ResponseHeader                    controlplaneProtocolFact[ResponseHeader]
	ResponseExpectation               controlplaneAuthorityInput[ResponseExpectation]
	CheckInPayload                    controlplaneProtocolFact[CheckInPayload]
	CheckInRequest                    controlplaneProtocolFact[CheckInRequest]
	CheckInResponsePayload            controlplaneProtocolFact[CheckInResponsePayload]
	CheckInResponseDocument           controlplaneProtocolFact[CheckInResponseDocument]
	UsageWatermark                    controlplaneProtocolFact[UsageWatermark]
	WorkUnitCount                     controlplaneProtocolFact[WorkUnitCount]
	OutcomeCount                      controlplaneProtocolFact[OutcomeCount]
	UsageWindow                       controlplaneProtocolFact[UsageWindow]
	ResponseCommitment                controlplaneProtocolFact[ResponseCommitment]
	RegistrationVerification          controlplaneAuthorityInput[RegistrationVerification]
	RegistrationAuthorityVerification controlplaneAuthorityInput[RegistrationAuthorityVerification]
	RegistrationCertificateIssuance   controlplaneIssuance[RegistrationCertificateIssuance]
	CheckInVerification               controlplaneAuthorityInput[CheckInVerification]
	CheckInCommitRequest              controlplaneAuthorityInput[CheckInCommitRequest]
	CheckInResponsePreparation        controlplaneIssuance[CheckInResponsePreparation]
	CheckInResponseVerification       controlplaneAuthorityInput[CheckInResponseVerification]
	ResponseIssuance                  controlplaneIssuance[ResponseIssuance[RegistrationDocument]]
	UpgradeRequiredIssuance           controlplaneIssuance[UpgradeRequiredIssuance]
	ResponseProjection                controlplaneIssuance[ResponseProjection[RegistrationDocument]]
	ResponseDocument                  controlplaneProtocolFact[ResponseDocument[RegistrationDocument, *RegistrationDocument]]
	ResponseVerification              controlplaneAuthorityInput[ResponseVerification[RegistrationDocument, *RegistrationDocument]]
	VerifiedRegistration              controlplaneVerifiedProof[VerifiedRegistration]
	VerifiedRegistrationAuthority     controlplaneVerifiedProof[VerifiedRegistrationAuthority]
	VerifiedInstallationCertificate   controlplaneVerifiedProof[VerifiedInstallationCertificate]
	VerifiedCheckIn                   controlplaneVerifiedProof[VerifiedCheckIn]
	VerifiedCheckInCommit             controlplaneVerifiedProof[VerifiedCheckInCommit]
	VerifiedCheckInResponse           controlplaneVerifiedProof[VerifiedCheckInResponse]
	VerifiedResponse                  controlplaneVerifiedProof[VerifiedResponse[RegistrationDocument, *RegistrationDocument]]
	checkInBinding                    controlplaneInternalFlow[checkInBinding]
	checkInDocumentValidation         controlplaneInternalFlow[checkInDocumentValidation]
	usageWindowBoundsWire             controlplaneInternalFlow[usageWindowBoundsWire]
	usageWindowWire                   controlplaneInternalFlow[usageWindowWire]
	usageDigestRequest                controlplaneInternalFlow[usageDigestRequest]
	responseDocumentWire              controlplaneInternalFlow[responseDocumentWire]
	responseBindingError              controlplaneTypedFailure[responseBindingError]
}

// TestControlplaneProductionStructsHaveCompilerVisibleDataFlowRoles makes a
// newly introduced carrier fail until its role in the real pipeline is named.
func TestControlplaneProductionStructsHaveCompilerVisibleDataFlowRoles(t *testing.T) {
	t.Parallel()

	_ = controlplaneContractInventory{}
	_ = controlplaneContractInventory{}.checkInBinding
	_ = controlplaneContractInventory{}.checkInDocumentValidation
	_ = controlplaneContractInventory{}.usageWindowBoundsWire
	_ = controlplaneContractInventory{}.usageWindowWire
	_ = controlplaneContractInventory{}.usageDigestRequest
	_ = controlplaneContractInventory{}.responseDocumentWire
	_ = controlplaneContractInventory{}.responseBindingError
	gotProduction, err := controlplaneProductionStructNames()
	if err != nil {
		t.Fatalf("controlplaneProductionStructNames() error = %v, want nil", err)
	}
	wantClassified := controlplaneClassifiedStructNames(t)
	for _, got := range gotProduction {
		if !slices.Contains(wantClassified, got) {
			t.Errorf("production struct %q has no compiler-visible data-flow role", got)
		}
	}
	for _, want := range wantClassified {
		if !slices.Contains(gotProduction, want) {
			t.Errorf("classified struct %q does not exist in production", want)
		}
	}
}

func controlplaneProductionStructNames() ([]controlplaneProductionStructName, error) {
	entries, err := os.ReadDir(".")
	if err != nil {
		return nil, err
	}
	files := token.NewFileSet()
	var names []controlplaneProductionStructName
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") ||
			strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		file, parseErr := parser.ParseFile(files, entry.Name(), nil, parser.SkipObjectResolution)
		if parseErr != nil {
			return nil, parseErr
		}
		for _, declaration := range file.Decls {
			generic, ok := declaration.(*ast.GenDecl)
			if !ok || generic.Tok != token.TYPE {
				continue
			}
			for _, raw := range generic.Specs {
				spec, ok := raw.(*ast.TypeSpec)
				if !ok {
					continue
				}
				if _, ok := spec.Type.(*ast.StructType); ok {
					names = append(names, controlplaneProductionStructName(spec.Name.Name))
				}
			}
		}
	}
	return names, nil
}

func controlplaneClassifiedStructNames(t *testing.T) []controlplaneProductionStructName {
	t.Helper()

	file, err := parser.ParseFile(token.NewFileSet(), "contract_inventory_test.go", nil, parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("parser.ParseFile() error = %v, want nil", err)
	}
	var names []controlplaneProductionStructName
	for _, declaration := range file.Decls {
		generic, ok := declaration.(*ast.GenDecl)
		if !ok || generic.Tok != token.TYPE {
			continue
		}
		for _, raw := range generic.Specs {
			spec, ok := raw.(*ast.TypeSpec)
			if !ok || spec.Name.Name != "controlplaneContractInventory" {
				continue
			}
			structure, ok := spec.Type.(*ast.StructType)
			if !ok {
				continue
			}
			for _, field := range structure.Fields.List {
				for _, name := range field.Names {
					names = append(names, controlplaneProductionStructName(name.Name))
				}
			}
		}
	}
	if len(names) == 0 {
		t.Fatalf("controlplaneContractInventory classified structs = %d, want at least one", len(names))
	}
	return names
}
