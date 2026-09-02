package standard

import (
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/capabilities"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gomodule"
)

func TestResolveProjectRelationshipRejectsOwnershipContradictions(t *testing.T) {
	t.Parallel()

	primitive, err := gomodule.ParsePath(core.PrimitiveModulePath)
	if err != nil {
		t.Fatalf("gomodule.ParsePath(Primitive) error = %v, want nil", err)
	}
	product, err := gomodule.ParsePath("example.com/product")
	if err != nil {
		t.Fatalf("gomodule.ParsePath(product) error = %v, want nil", err)
	}
	filesystem, err := capabilities.IdentityForEffect(capabilities.EffectFilesystem)
	if err != nil {
		t.Fatalf("capabilities.IdentityForEffect(filesystem) error = %v, want nil", err)
	}
	transport, err := capabilities.IdentityForEffect(capabilities.EffectTransport)
	if err != nil {
		t.Fatalf("capabilities.IdentityForEffect(transport) error = %v, want nil", err)
	}
	primitiveFilesystem := CapabilityOwnership{Capability: filesystem, Role: CapabilityRolePrimitiveImplementation}
	adapterFilesystem := CapabilityOwnership{Capability: filesystem, Role: CapabilityRoleDeclaredAdapter}
	adapterTransport := CapabilityOwnership{Capability: transport, Role: CapabilityRoleDeclaredAdapter}
	owned, ownershipErr := PackageOwnsCapability([]CapabilityOwnership{adapterFilesystem, adapterTransport}, transport)
	if ownershipErr != nil || !owned {
		t.Fatalf("PackageOwnsCapability(transport) = (%t, %v), want (true, nil)", owned, ownershipErr)
	}
	notOwned, ownershipErr := PackageOwnsCapability([]CapabilityOwnership{adapterFilesystem}, transport)
	if ownershipErr != nil || notOwned {
		t.Fatalf("PackageOwnsCapability(absent transport) = (%t, %v), want (false, nil)", notOwned, ownershipErr)
	}
	invalidOwned, ownershipErr := PackageOwnsCapability([]CapabilityOwnership{adapterFilesystem}, capabilities.Identity{})
	if !errors.Is(ownershipErr, core.ErrCapabilitiesContract) || invalidOwned {
		t.Fatalf("PackageOwnsCapability(unknown) = (%t, %v), want (false, errors.Is(..., %v))", invalidOwned, ownershipErr, core.ErrCapabilitiesContract)
	}
	cases := []struct {
		wantErr   error
		name      string
		module    gomodule.Path
		ownership []CapabilityOwnership
		want      ProjectRelationship
	}{
		{name: "Primitive module without effect-owning package remains Primitive", module: primitive, want: ProjectRelationshipPrimitive},
		{name: "Primitive implementation role is admitted from module identity", module: primitive, ownership: []CapabilityOwnership{primitiveFilesystem}, want: ProjectRelationshipPrimitive},
		{name: "product without ownership remains a consumer", module: product, want: ProjectRelationshipProduct},
		{name: "external declared adapter is distinguished from a product", module: product, ownership: []CapabilityOwnership{adapterFilesystem}, want: ProjectRelationshipDeclaredAdapter},
		{name: "external multi-capability adapter retains adapter relationship", module: product, ownership: []CapabilityOwnership{adapterFilesystem, adapterTransport}, want: ProjectRelationshipDeclaredAdapter},
		{name: "Primitive cannot claim an external adapter role", module: primitive, ownership: []CapabilityOwnership{adapterFilesystem}, wantErr: core.ErrStandardConflict},
		{name: "product cannot impersonate a Primitive implementation", module: product, ownership: []CapabilityOwnership{primitiveFilesystem}, wantErr: core.ErrStandardConflict},
		{name: "duplicated capability ownership is a contradiction", module: product, ownership: []CapabilityOwnership{adapterFilesystem, adapterFilesystem}, wantErr: core.ErrStandardConflict},
		{name: "unknown ownership role is rejected", module: product, ownership: []CapabilityOwnership{{Capability: filesystem}}, wantErr: core.ErrStandardContract},
		{name: "absent module identity is rejected", ownership: []CapabilityOwnership{adapterFilesystem}, wantErr: core.ErrStandardContract},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := ResolveProjectRelationship(testCase.module, testCase.ownership)
			if testCase.wantErr != nil {
				if !errors.Is(gotErr, testCase.wantErr) || got != ProjectRelationshipUnknown {
					t.Fatalf("ResolveProjectRelationship() = (%v, %v), want unknown and errors.Is(..., %v)", got, gotErr, testCase.wantErr)
				}
				return
			}
			if gotErr != nil || got != testCase.want {
				t.Fatalf("ResolveProjectRelationship() = (%v, %v), want (%v, nil)", got, gotErr, testCase.want)
			}
		})
	}
}

func TestPackageFileCatalogValidateOwnershipPreservesOrthogonalSiteEvidence(t *testing.T) {
	t.Parallel()

	module, err := gomodule.ParsePath(core.PrimitiveModulePath)
	if err != nil {
		t.Fatalf("gomodule.ParsePath(Primitive) error = %v, want nil", err)
	}
	filesystem, err := capabilities.IdentityForEffect(capabilities.EffectFilesystem)
	if err != nil {
		t.Fatalf("capabilities.IdentityForEffect(filesystem) error = %v, want nil", err)
	}
	ownership := []CapabilityOwnership{{Capability: filesystem, Role: CapabilityRolePrimitiveImplementation}}
	packagePath := fixturePath(t, "filestore")
	file := SourceFile{
		Path: fixturePath(t, "filestore/mixed.go"), Package: packagePath, Imports: &SourceFileImports{},
		Language: SourceLanguageGo, Kind: SourceFileKindProduction,
		Effects: SourceFileEffects{
			Capabilities:   []PrimitiveCapabilityUse{{Package: core.PackageFilestore}, {Package: core.PackageTemporal}},
			Implementation: []SourceEffectSite{sourceEffectSiteFixture(t, &PrimitiveCapabilityUse{Package: core.PackageFilestore}, 1)},
			Mediated:       []SourceEffectSite{sourceEffectSiteFixture(t, &PrimitiveCapabilityUse{Package: core.PackageTemporal}, 2)},
			Unresolved:     unresolvedEffectSites(t, 1),
			Posture:        PrimitiveEffectUnresolved,
		},
	}
	architecture, err := DerivePackageArchitecture([]SourceFile{file})
	if err != nil {
		t.Fatalf("DerivePackageArchitecture(mixed file) error = %v, want nil", err)
	}
	catalog := PackageFileCatalog{Package: packagePath, Files: []SourceFile{file}, Architecture: &architecture}
	if gotErr := catalog.ValidateOwnership(module, ownership); gotErr != nil {
		t.Fatalf("PackageFileCatalog.ValidateOwnership(mixed file) error = %v, want nil", gotErr)
	}

	direct := catalog
	direct.Files = []SourceFile{file}
	direct.Files[0].Effects.Capabilities = append(direct.Files[0].Effects.Capabilities, PrimitiveCapabilityUse{Package: core.PackageExchange})
	direct.Files[0].Effects.Direct = []SourceEffectSite{sourceEffectSiteFixture(t, &PrimitiveCapabilityUse{Package: core.PackageExchange}, 3)}
	direct.Files[0].Effects.Posture = PrimitiveEffectDirectObserved
	directArchitecture, err := DerivePackageArchitecture(direct.Files)
	if err != nil {
		t.Fatalf("DerivePackageArchitecture(direct mutation) error = %v, want nil", err)
	}
	direct.Architecture = &directArchitecture
	if gotErr := direct.ValidateOwnership(module, ownership); !errors.Is(gotErr, core.ErrStandardConflict) {
		t.Fatalf("PackageFileCatalog.ValidateOwnership(unowned direct mutation) error = %v, want errors.Is(..., %v)", gotErr, core.ErrStandardConflict)
	}

	misclassified := catalog
	misclassified.Files = []SourceFile{file}
	misclassified.Files[0].Effects.Direct = append(misclassified.Files[0].Effects.Direct, misclassified.Files[0].Effects.Implementation...)
	misclassified.Files[0].Effects.Implementation = nil
	misclassified.Files[0].Effects.Posture = PrimitiveEffectDirectObserved
	misclassifiedArchitecture, err := DerivePackageArchitecture(misclassified.Files)
	if err != nil {
		t.Fatalf("DerivePackageArchitecture(owned direct mutation) error = %v, want nil", err)
	}
	misclassified.Architecture = &misclassifiedArchitecture
	if gotErr := misclassified.ValidateOwnership(module, ownership); !errors.Is(gotErr, core.ErrStandardConflict) {
		t.Fatalf("PackageFileCatalog.ValidateOwnership(owned direct mutation) error = %v, want errors.Is(..., %v)", gotErr, core.ErrStandardConflict)
	}

	missing := catalog
	missing.Files = []SourceFile{file}
	if gotErr := missing.ValidateOwnership(module, nil); !errors.Is(gotErr, core.ErrStandardConflict) {
		t.Fatalf("PackageFileCatalog.ValidateOwnership(unowned implementation) error = %v, want errors.Is(..., %v)", gotErr, core.ErrStandardConflict)
	}
}
