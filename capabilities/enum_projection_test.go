package capabilities

import (
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestEnumJSONProjectionRejectsCompleteInvalidByteDomains(t *testing.T) {
	t.Parallel()
	for raw := range 256 {
		operation := Operation(raw)
		disposition := StandardSymbolDisposition(raw)
		identity := Identity{effect: Effect(raw)}
		cases := []struct {
			name     string
			valid    bool
			validate func() error
			isValid  func() bool
			text     func() string
			encode   func() ([]byte, error)
		}{
			{"operation", raw < int(operationLimit), operation.Validate, operation.IsValid, operation.String, operation.MarshalJSON},
			{"disposition", raw >= int(StandardSymbolPure) && raw <= int(StandardSymbolUnresolved), disposition.Validate, disposition.IsValid, disposition.String, disposition.MarshalJSON},
			{"identity", raw >= int(EffectFilesystem) && raw < int(effectLimit), identity.Validate, identity.IsValid, identity.String, identity.MarshalJSON},
		}
		for _, tc := range cases {
			err := tc.validate()
			encoded, encodeErr := tc.encode()
			if tc.isValid() != tc.valid || (err == nil) != tc.valid {
				t.Fatalf("%s byte %d admission = (%t,%v), want %t", tc.name, raw, tc.isValid(), err, tc.valid)
			}
			if !tc.valid {
				if !errors.Is(err, core.ErrCapabilitiesContract) || !errors.Is(encodeErr, core.ErrCapabilitiesContract) || len(encoded) != 0 || tc.text() != core.UnknownEnumDiagnostic {
					t.Fatalf("%s byte %d invalid projection = (%q,%q,%v,%v)", tc.name, raw, tc.text(), encoded, err, encodeErr)
				}
				continue
			}
			if encodeErr != nil {
				t.Fatal(encodeErr)
			}
			decoded, err := core.DecodeJSONStringToken(encoded)
			if err != nil || decoded != tc.text() {
				t.Fatalf("%s byte %d JSON = (%q,%v), want %q", tc.name, raw, decoded, err, tc.text())
			}
		}
		effect, err := identity.Effect()
		if raw >= int(EffectFilesystem) && raw < int(effectLimit) {
			if err != nil || effect != Effect(raw) {
				t.Fatalf("identity projection = (%v,%v), want %d", effect, err, raw)
			}
		} else if !errors.Is(err, core.ErrCapabilitiesContract) || effect != EffectUnknown {
			t.Fatalf("invalid identity exposed effect %v / %v", effect, err)
		}
	}
}

func TestRequirementEnumTextMatchesOwnedNames(t *testing.T) {
	t.Parallel()
	for raw := range 256 {
		target := RequirementTarget(raw)
		scope := Scope(raw)
		targetNames := [requirementTargetLimit]string{RequirementTargetPackage: requirementTargetPackageName, RequirementTargetEffect: requirementTargetEffectName}
		scopeNames := [scopeLimit]string{ScopeProduction: scopeProductionName, ScopeTest: scopeTestName}
		wantTarget, wantScope := core.UnknownEnumDiagnostic, core.UnknownEnumDiagnostic
		if raw > int(RequirementTargetUnknown) && raw < int(requirementTargetLimit) {
			wantTarget = targetNames[target]
		}
		if raw > int(ScopeUnknown) && raw < int(scopeLimit) {
			wantScope = scopeNames[scope]
		}
		if target.String() != wantTarget || scope.String() != wantScope {
			t.Fatalf("byte %d text = (%q,%q), want (%q,%q)", raw, target, scope, wantTarget, wantScope)
		}
	}
}
