package projectstandards

import (
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/capabilities"
	"github.com/deliri/primitive/v2026/core"
)

func TestPackageRoleExhaustsTheClosedArchitecturalDomain(t *testing.T) {
	t.Parallel()

	wantNames := map[core.PackageRole]string{
		core.PackageRoleValueContract:         "value_contract",
		core.PackageRoleDomainAgreement:       "domain_agreement",
		core.PackageRoleAuthenticationBinding: "authentication_binding",
		core.PackageRoleEffectCapability:      "effect_capability",
		core.PackageRoleWireProtocol:          "wire_protocol",
		core.PackageRoleOrchestration:         "orchestration",
	}
	for raw := 0; raw <= 255; raw++ {
		got := core.PackageRole(raw)
		wantName, wantValid := wantNames[got]
		gotErr := got.Validate()
		if wantValid {
			if gotErr != nil || !got.IsValid() || got.String() != wantName {
				t.Fatalf("PackageRole(%d) = (%q, %t, %v), want (%q, true, nil)", raw, got.String(), got.IsValid(), gotErr, wantName)
			}
			continue
		}
		if !errors.Is(gotErr, core.ErrPrimitiveContract) || got.IsValid() || got.String() != "" {
			t.Fatalf("PackageRole(%d) = (%q, %t, %v), want empty, false, and errors.Is(..., %v)", raw, got.String(), got.IsValid(), gotErr, core.ErrPrimitiveContract)
		}
	}
}

func TestPackageRoleJSONRejectsNonCanonicalAndPreservesTheReceiver(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		data string
	}{
		{name: "unknown role spelling is rejected", data: `"provider"`},
		{name: "empty role spelling is rejected", data: `""`},
		{name: "case changed role is rejected", data: `"Domain_Agreement"`},
		{name: "hyphenated alias is rejected", data: `"effect-capability"`},
		{name: "camel case alias is rejected", data: `"effectCapability"`},
		{name: "uppercase alias is rejected", data: `"EFFECT_CAPABILITY"`},
		{name: "numeric role is rejected", data: `2`},
		{name: "boolean role is rejected", data: `true`},
		{name: "null role is rejected", data: `null`},
		{name: "array role is rejected", data: `[]`},
		{name: "object role is rejected", data: `{}`},
		{name: "trailing JSON is rejected", data: `"domain_agreement" true`},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			got := core.PackageRoleWireProtocol
			gotErr := got.UnmarshalJSON([]byte(testCase.data))
			if !errors.Is(gotErr, core.ErrJSONContract) || got != core.PackageRoleWireProtocol {
				t.Fatalf("PackageRole.UnmarshalJSON(%q) = (%v, %v), want preserved %v and errors.Is(..., %v)", testCase.data, got, gotErr, core.PackageRoleWireProtocol, core.ErrJSONContract)
			}
		})
	}
}

func TestPackageKnowledgeRequiresOneAuthoredPrimaryRole(t *testing.T) {
	t.Parallel()

	catalog := fixtureCatalog(t)
	knowledge := catalog.Packages[0].Package.Knowledge
	knowledge.AuthorRole = core.PackageRoleUnknown
	gotErr := knowledge.Validate()
	if !errors.Is(gotErr, core.ErrProjectStandardsContract) {
		t.Fatalf("PackageKnowledge.Validate(missing role) error = %v, want errors.Is(..., %v)", gotErr, core.ErrProjectStandardsContract)
	}

	knowledge.AuthorRole = core.PackageRoleEffectCapability
	if gotErr := knowledge.Validate(); !errors.Is(gotErr, core.ErrProjectStandardsConflict) {
		t.Fatalf("PackageKnowledge.Validate(effect role without ownership) error = %v, want errors.Is(..., %v)", gotErr, core.ErrProjectStandardsConflict)
	}

	filesystem, err := capabilities.IdentityForEffect(capabilities.EffectFilesystem)
	if err != nil {
		t.Fatalf("capabilities.IdentityForEffect(filesystem) error = %v, want nil", err)
	}
	knowledge.AuthorRole = core.PackageRoleDomainAgreement
	knowledge.AuthorCapabilityOwnership = []CapabilityOwnership{{Capability: filesystem, Role: CapabilityRolePrimitiveImplementation}}
	if gotErr := knowledge.Validate(); !errors.Is(gotErr, core.ErrProjectStandardsConflict) {
		t.Fatalf("PackageKnowledge.Validate(domain role with ownership) error = %v, want errors.Is(..., %v)", gotErr, core.ErrProjectStandardsConflict)
	}
}

func TestPackageRoleCapabilityOwnershipExhaustsTheCompleteCrossProduct(t *testing.T) {
	t.Parallel()

	filesystem, err := capabilities.IdentityForEffect(capabilities.EffectFilesystem)
	if err != nil {
		t.Fatalf("capabilities.IdentityForEffect(filesystem) error = %v, want nil", err)
	}
	owned := []CapabilityOwnership{{Capability: filesystem, Role: CapabilityRolePrimitiveImplementation}}
	roles := []core.PackageRole{
		core.PackageRoleValueContract,
		core.PackageRoleDomainAgreement,
		core.PackageRoleAuthenticationBinding,
		core.PackageRoleEffectCapability,
		core.PackageRoleWireProtocol,
		core.PackageRoleOrchestration,
	}
	for _, role := range roles {
		t.Run(role.String()+" without capability ownership", func(t *testing.T) {
			t.Parallel()

			gotErr := validatePackageRoleOwnership(role, nil)
			if role == core.PackageRoleEffectCapability {
				if !errors.Is(gotErr, core.ErrProjectStandardsConflict) {
					t.Fatalf("validatePackageRoleOwnership(%v, absent) error = %v, want errors.Is(..., %v)", role, gotErr, core.ErrProjectStandardsConflict)
				}
				return
			}
			if gotErr != nil {
				t.Fatalf("validatePackageRoleOwnership(%v, absent) error = %v, want nil", role, gotErr)
			}
		})
		t.Run(role.String()+" with filesystem ownership", func(t *testing.T) {
			t.Parallel()

			gotErr := validatePackageRoleOwnership(role, owned)
			if role != core.PackageRoleEffectCapability {
				if !errors.Is(gotErr, core.ErrProjectStandardsConflict) {
					t.Fatalf("validatePackageRoleOwnership(%v, filesystem) error = %v, want errors.Is(..., %v)", role, gotErr, core.ErrProjectStandardsConflict)
				}
				return
			}
			if gotErr != nil {
				t.Fatalf("validatePackageRoleOwnership(%v, filesystem) error = %v, want nil", role, gotErr)
			}
		})
	}
}
