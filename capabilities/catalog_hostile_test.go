package capabilities

import (
	"errors"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestClosedEnumDomainsRejectEveryUnknownAndFutureValue(t *testing.T) {
	t.Parallel()

	for raw := 0; raw <= 255; raw++ {
		effect := Effect(raw)
		wantEffectValid := effect >= EffectFilesystem && effect <= EffectSignal
		if effect.IsValid() != wantEffectValid {
			t.Errorf("Effect(%d).IsValid() = %t, want %t", raw, effect.IsValid(), wantEffectValid)
		}
		if !wantEffectValid && effect.String() != core.UnknownEnumDiagnostic {
			t.Errorf("Effect(%d).String() = %q, want %q", raw, effect.String(), core.UnknownEnumDiagnostic)
		}

		target := RequirementTarget(raw)
		wantTargetValid := target >= RequirementTargetPackage && target <= RequirementTargetEffect
		if target.IsValid() != wantTargetValid {
			t.Errorf("RequirementTarget(%d).IsValid() = %t, want %t", raw, target.IsValid(), wantTargetValid)
		}
		if !wantTargetValid && target.String() != core.UnknownEnumDiagnostic {
			t.Errorf(
				"RequirementTarget(%d).String() = %q, want %q",
				raw,
				target.String(),
				core.UnknownEnumDiagnostic,
			)
		}

		scope := Scope(raw)
		wantScopeValid := scope >= ScopeProduction && scope <= ScopeTest
		if scope.IsValid() != wantScopeValid {
			t.Errorf("Scope(%d).IsValid() = %t, want %t", raw, scope.IsValid(), wantScopeValid)
		}
		if !wantScopeValid && scope.String() != core.UnknownEnumDiagnostic {
			t.Errorf("Scope(%d).String() = %q, want %q", raw, scope.String(), core.UnknownEnumDiagnostic)
		}
	}
}

func TestPurposeBoundariesAreExactAndBounded(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{name: "empty purpose is rejected", wantErr: true},
		{name: "one byte purpose is admitted", value: "x"},
		{name: "exact maximum purpose is admitted", value: strings.Repeat("x", purposeMaximumBytes)},
		{
			name:    "one byte above maximum purpose is rejected",
			value:   strings.Repeat("x", purposeMaximumBytes+1),
			wantErr: true,
		},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := newPurpose(testCase.value)
			if testCase.wantErr {
				if !errors.Is(gotErr, core.ErrCapabilitiesContract) || got != (Purpose{}) {
					t.Fatalf(
						"newPurpose(%d bytes) = (%d bytes, %v), want empty and errors.Is(..., %v)",
						len(testCase.value),
						len(got.String()),
						gotErr,
						core.ErrCapabilitiesContract,
					)
				}
				return
			}
			if gotErr != nil || got.String() != testCase.value {
				t.Fatalf(
					"newPurpose(%d bytes) = (%d bytes, %v), want identical value and nil",
					len(testCase.value),
					len(got.String()),
					gotErr,
				)
			}
		})
	}
}

func TestAllExhaustivelyResolvesEveryCompiledPackage(t *testing.T) {
	t.Parallel()

	catalog, err := All()
	if err != nil {
		t.Fatalf("All() error = %v, want nil", err)
	}
	gotCount := 0
	for capability := range catalog.Capabilities() {
		gotCount++
		if err := capability.Validate(); err != nil {
			t.Errorf("Capability(%v).Validate() error = %v, want nil", capability.Package, err)
			continue
		}
		gotPath, err := capability.ImportPath()
		if err != nil {
			t.Errorf("Capability(%v).ImportPath() error = %v, want nil", capability.Package, err)
			continue
		}
		wantPath, err := capability.Package.ImportPath()
		if err != nil {
			t.Errorf("PackageIdentity(%v).ImportPath() error = %v, want nil", capability.Package, err)
			continue
		}
		if gotPath != wantPath {
			t.Errorf("Capability(%v).ImportPath() = %q, want %q", capability.Package, gotPath, wantPath)
		}

		scopes := []Scope{ScopeProduction, ScopeTest}
		for _, scope := range scopes {
			got, gotErr := catalog.Resolve(ForPackage(scope, capability.Package))
			if scope == ScopeProduction && capability.Kind == core.PackageKindTestSupport {
				if !errors.Is(gotErr, core.ErrCapabilityUnavailable) || got != (Match{}) {
					t.Errorf(
						"Catalog.Resolve(%v, %v) = (%+v, %v), want zero and errors.Is(..., %v)",
						scope,
						capability.Package,
						got,
						gotErr,
						core.ErrCapabilityUnavailable,
					)
				}
				continue
			}
			if gotErr != nil || got.Capability != capability {
				t.Errorf(
					"Catalog.Resolve(%v, %v) = (%+v, %v), want capability %+v and nil",
					scope,
					capability.Package,
					got,
					gotErr,
					capability,
				)
			}
		}
	}
	if gotCount != core.PrimitivePackageCount {
		t.Fatalf("Catalog.Capabilities() count = %d, want %d", gotCount, core.PrimitivePackageCount)
	}
}

func TestResolveExhaustsTheRealWorldEffectDomain(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		effect    Effect
		wantOwner core.PackageIdentity
	}{
		{name: "filesystem resolves only to Filestore", effect: EffectFilesystem, wantOwner: core.PackageFilestore},
		{name: "process resolves only to Process", effect: EffectProcess, wantOwner: core.PackageProcess},
		{name: "transport resolves only to Exchange", effect: EffectTransport, wantOwner: core.PackageExchange},
		{name: "time resolves only to Temporal", effect: EffectTime, wantOwner: core.PackageTemporal},
		{name: "entropy resolves only to Keygen", effect: EffectEntropy, wantOwner: core.PackageKeygen},
		{name: "secret access resolves only to Secretstore", effect: EffectSecret, wantOwner: core.PackageSecretStore},
		{name: "host observation resolves only to Hostfacts", effect: EffectHost, wantOwner: core.PackageHostFacts},
		{name: "signals resolve only to Shutdown", effect: EffectSignal, wantOwner: core.PackageShutdown},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			for _, scope := range []Scope{ScopeProduction, ScopeTest} {
				got, gotErr := Resolve(ForEffect(scope, testCase.effect))
				if gotErr != nil {
					t.Fatalf("Resolve(%v, %v) error = %v, want nil", scope, testCase.effect, gotErr)
				}
				if got.Capability.Package != testCase.wantOwner {
					t.Fatalf(
						"Resolve(%v, %v) owner = %v, want %v",
						scope,
						testCase.effect,
						got.Capability.Package,
						testCase.wantOwner,
					)
				}
				if !got.Capability.Owns(testCase.effect) {
					t.Fatalf("Capability(%v).Owns(%v) = false, want true", got.Capability.Package, testCase.effect)
				}
			}
		})
	}
}

func TestCapabilityOwnershipConservesNeutralityAcrossTheCompleteCrossProduct(t *testing.T) {
	t.Parallel()

	catalog, err := All()
	if err != nil {
		t.Fatalf("All() error = %v, want nil", err)
	}
	gotOwned := 0
	gotNeutral := 0
	for capability := range catalog.Capabilities() {
		for effect := EffectFilesystem; effect < effectLimit; effect++ {
			if capability.Owns(effect) {
				gotOwned++
				continue
			}
			gotNeutral++
		}
	}
	wantOwned := int(effectLimit - EffectFilesystem)
	wantNeutral := core.PrimitivePackageCount*wantOwned - wantOwned
	if gotOwned != wantOwned || gotNeutral != wantNeutral {
		t.Fatalf(
			"complete package/effect ownership = (%d owned, %d neutral), want (%d owned, %d neutral)",
			gotOwned,
			gotNeutral,
			wantOwned,
			wantNeutral,
		)
	}
}

func TestResolutionMutationPairsChangeOnlyTheLoadBearingFact(t *testing.T) {
	t.Parallel()

	t.Run("changing only effect identity changes the resolved owner", func(t *testing.T) {
		t.Parallel()

		baseline, baselineErr := Resolve(ForEffect(ScopeProduction, EffectFilesystem))
		mutated, mutatedErr := Resolve(ForEffect(ScopeProduction, EffectProcess))
		if baselineErr != nil || mutatedErr != nil {
			t.Fatalf("Resolve(effect pair) errors = (%v, %v), want (nil, nil)", baselineErr, mutatedErr)
		}
		if baseline.Requirement.Scope != mutated.Requirement.Scope ||
			baseline.Requirement.Target != mutated.Requirement.Target {
			t.Fatalf(
				"effect pair stable facts = (%+v, %+v), want equal scope and target",
				baseline.Requirement,
				mutated.Requirement,
			)
		}
		if baseline.Requirement.Effect == mutated.Requirement.Effect ||
			baseline.Capability.Package == mutated.Capability.Package {
			t.Fatalf(
				"effect pair = (%v -> %v, %v -> %v), want changed effect and owner",
				baseline.Requirement.Effect,
				mutated.Requirement.Effect,
				baseline.Capability.Package,
				mutated.Capability.Package,
			)
		}
	})

	t.Run("changing only test support scope changes acceptance to refusal", func(t *testing.T) {
		t.Parallel()

		baselineRequirement := ForPackage(ScopeTest, core.PackageTestSerial)
		baseline, baselineErr := Resolve(baselineRequirement)
		mutatedRequirement := baselineRequirement
		mutatedRequirement.Scope = ScopeProduction
		mutated, mutatedErr := Resolve(mutatedRequirement)
		if baselineErr != nil || baseline.Capability.Package != core.PackageTestSerial {
			t.Fatalf("Resolve(test scope) = (%+v, %v), want Testserial and nil", baseline, baselineErr)
		}
		if !errors.Is(mutatedErr, core.ErrCapabilityUnavailable) || mutated != (Match{}) {
			t.Fatalf(
				"Resolve(production mutation) = (%+v, %v), want zero and errors.Is(..., %v)",
				mutated,
				mutatedErr,
				core.ErrCapabilityUnavailable,
			)
		}
		if baselineRequirement.Package != mutatedRequirement.Package ||
			baselineRequirement.Target != mutatedRequirement.Target ||
			baselineRequirement.Scope == mutatedRequirement.Scope {
			t.Fatalf(
				"scope mutation = (%+v, %+v), want only Scope to change",
				baselineRequirement,
				mutatedRequirement,
			)
		}
	})
}

func TestRequirementValidationRejectsEveryImpossibleUnionShape(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		requirement Requirement
	}{
		{name: "zero requirement has no scope or target"},
		{
			name:        "package target has no package",
			requirement: Requirement{Target: RequirementTargetPackage, Scope: ScopeProduction},
		},
		{
			name: "package target rejects future package identity",
			requirement: Requirement{
				Package: core.PackageIdentity(255),
				Target:  RequirementTargetPackage,
				Scope:   ScopeProduction,
			},
		},
		{
			name: "package target rejects an effect sibling",
			requirement: Requirement{
				Package: core.PackageFilestore,
				Effect:  EffectFilesystem,
				Target:  RequirementTargetPackage,
				Scope:   ScopeProduction,
			},
		},
		{
			name:        "effect target has no effect",
			requirement: Requirement{Target: RequirementTargetEffect, Scope: ScopeProduction},
		},
		{
			name: "effect target rejects future effect",
			requirement: Requirement{
				Effect: Effect(255),
				Target: RequirementTargetEffect,
				Scope:  ScopeProduction,
			},
		},
		{
			name: "effect target rejects a package sibling",
			requirement: Requirement{
				Package: core.PackageFilestore,
				Effect:  EffectFilesystem,
				Target:  RequirementTargetEffect,
				Scope:   ScopeProduction,
			},
		},
		{
			name:        "known package target rejects absent scope",
			requirement: ForPackage(ScopeUnknown, core.PackageFilestore),
		},
		{
			name:        "known effect target rejects absent scope",
			requirement: ForEffect(ScopeUnknown, EffectFilesystem),
		},
		{
			name:        "known package target rejects future scope",
			requirement: ForPackage(Scope(255), core.PackageFilestore),
		},
		{
			name:        "known effect target rejects future scope",
			requirement: ForEffect(Scope(255), EffectFilesystem),
		},
		{
			name: "future target is never treated as a package or effect",
			requirement: Requirement{
				Package: core.PackageFilestore,
				Target:  RequirementTarget(255),
				Scope:   ScopeProduction,
			},
		},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := Resolve(testCase.requirement)
			if !errors.Is(gotErr, core.ErrCapabilitiesContract) || got != (Match{}) {
				t.Fatalf(
					"Resolve(%+v) = (%+v, %v), want zero and errors.Is(..., %v)",
					testCase.requirement,
					got,
					gotErr,
					core.ErrCapabilitiesContract,
				)
			}
		})
	}
}

func TestMatchValidationRejectsContradictoryResolutionEvidence(t *testing.T) {
	t.Parallel()

	filesystem, err := Resolve(ForEffect(ScopeProduction, EffectFilesystem))
	if err != nil {
		t.Fatalf("Resolve(filesystem) error = %v, want nil", err)
	}
	process, err := Resolve(ForEffect(ScopeProduction, EffectProcess))
	if err != nil {
		t.Fatalf("Resolve(process) error = %v, want nil", err)
	}
	testSupport, err := Resolve(ForPackage(ScopeTest, core.PackageTestSerial))
	if err != nil {
		t.Fatalf("Resolve(testserial) error = %v, want nil", err)
	}

	tests := []struct {
		wantErr error
		name    string
		match   Match
	}{
		{
			name:  "zero match carries no requirement or capability",
			match: Match{},
		},
		{
			name:  "filesystem requirement cannot carry Process",
			match: Match{Requirement: filesystem.Requirement, Capability: process.Capability},
		},
		{
			name: "package requirement cannot carry another package",
			match: Match{
				Requirement: ForPackage(ScopeProduction, core.PackageFilestore),
				Capability:  process.Capability,
			},
		},
		{
			name: "production source cannot carry a test-support capability",
			match: Match{
				Requirement: ForPackage(ScopeProduction, core.PackageTestSerial),
				Capability:  testSupport.Capability,
			},
			wantErr: core.ErrCapabilityUnavailable,
		},
		{
			name: "purpose drift invalidates an otherwise matching capability",
			match: Match{
				Requirement: filesystem.Requirement,
				Capability: Capability{
					Package: core.PackageFilestore,
					Kind:    core.PackageKindProduction,
					Purpose: Purpose{value: "invented second description"},
				},
			},
		},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			wantErr := testCase.wantErr
			if wantErr == nil {
				wantErr = core.ErrCapabilitiesContract
			}
			gotErr := testCase.match.Validate()
			if !errors.Is(gotErr, wantErr) {
				t.Fatalf("Match.Validate() error = %v, want errors.Is(..., %v)", gotErr, wantErr)
			}
		})
	}
}

func TestCapabilityResolutionLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive exact effect resolves to its one compiled owner", func(t *testing.T) {
		t.Parallel()

		got, gotErr := Resolve(ForEffect(ScopeProduction, EffectEntropy))
		if gotErr != nil || got.Capability.Package != core.PackageKeygen {
			t.Fatalf(
				"Resolve(entropy) = (%+v, %v), want owner %v and nil",
				got,
				gotErr,
				core.PackageKeygen,
			)
		}
	})

	t.Run("negative production request cannot admit test-only capability", func(t *testing.T) {
		t.Parallel()

		got, gotErr := Resolve(ForPackage(ScopeProduction, core.PackageControlPlaneTest))
		if !errors.Is(gotErr, core.ErrCapabilityUnavailable) || got != (Match{}) {
			t.Fatalf(
				"Resolve(production controlplanetest) = (%+v, %v), want zero and errors.Is(..., %v)",
				got,
				gotErr,
				core.ErrCapabilityUnavailable,
			)
		}
	})

	t.Run("neutral unrelated package does not manufacture effect ownership", func(t *testing.T) {
		t.Parallel()

		got, gotErr := Resolve(ForPackage(ScopeProduction, core.PackageCurrency))
		if gotErr != nil {
			t.Fatalf("Resolve(currency) error = %v, want nil", gotErr)
		}
		if got.Capability.Owns(EffectFilesystem) {
			t.Fatalf("Currency.Owns(filesystem) = true, want false")
		}
	})
}
