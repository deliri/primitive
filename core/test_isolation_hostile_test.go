package core

import (
	"errors"
	"math"
	"testing"
)

type isolationValidationCase[T interface{ Validate() error }] struct {
	wantErr error
	value   T
	name    string
}

func TestIsolationHazardAndScopeExhaustClosedDomains(t *testing.T) {
	t.Parallel()

	hazardCases := []isolationValidationCase[TestIsolationHazard]{
		{name: "zero hazard rejected", value: TestIsolationHazardUnknown, wantErr: ErrTestIsolationContract},
		{name: "process environment admitted", value: TestIsolationHazardProcessEnvironment},
		{name: "process working directory admitted", value: TestIsolationHazardProcessWorkingDirectory},
		{name: "process signal admitted", value: TestIsolationHazardProcessSignal},
		{name: "process output admitted", value: TestIsolationHazardProcessOutput},
		{name: "process logger admitted", value: TestIsolationHazardProcessLogger},
		{name: "global registry admitted", value: TestIsolationHazardGlobalRegistry},
		{name: "runtime allocation admitted", value: TestIsolationHazardRuntimeAllocation},
		{name: "sibling order admitted", value: TestIsolationHazardSiblingOrder},
		{name: "private hazard limit rejected", value: testIsolationHazardLimit, wantErr: ErrTestIsolationContract},
		{name: "maximum hazard backing value rejected", value: TestIsolationHazard(math.MaxUint8), wantErr: ErrTestIsolationContract},
	}
	runIsolationValidationCases(t, hazardCases)

	scopeCases := []isolationValidationCase[TestIsolationScope]{
		{name: "zero scope rejected", value: TestIsolationScopeUnknown, wantErr: ErrTestIsolationContract},
		{name: "sibling table admitted", value: TestIsolationScopeSiblingTable},
		{name: "package process admitted", value: TestIsolationScopePackageProcess},
		{name: "private scope limit rejected", value: testIsolationScopeLimit, wantErr: ErrTestIsolationContract},
		{name: "maximum scope backing value rejected", value: TestIsolationScope(math.MaxUint8), wantErr: ErrTestIsolationContract},
	}
	runIsolationValidationCases(t, scopeCases)
}

func runIsolationValidationCases[T interface{ Validate() error }](
	t *testing.T,
	cases []isolationValidationCase[T],
) {
	t.Helper()

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			gotErr := tc.value.Validate()
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("Validate() error = %v, want %v", gotErr, tc.wantErr)
			}
			if tc.wantErr != nil && !errors.Is(gotErr, ErrPrimitiveContract) {
				t.Fatalf("Validate() error = %v, want parent %v", gotErr, ErrPrimitiveContract)
			}
		})
	}
}

func TestIsolationDeclarationCombinationMatrix(t *testing.T) {
	t.Parallel()

	cases := []struct {
		wantErr     error
		name        string
		declaration TestIsolationDeclaration
	}{
		{
			name: "process environment requires package process",
			declaration: TestIsolationDeclaration{
				Hazard: TestIsolationHazardProcessEnvironment,
				Scope:  TestIsolationScopePackageProcess,
			},
		},
		{
			name: "process working directory requires package process",
			declaration: TestIsolationDeclaration{
				Hazard: TestIsolationHazardProcessWorkingDirectory,
				Scope:  TestIsolationScopePackageProcess,
			},
		},
		{
			name: "process signal requires package process",
			declaration: TestIsolationDeclaration{
				Hazard: TestIsolationHazardProcessSignal,
				Scope:  TestIsolationScopePackageProcess,
			},
		},
		{
			name: "process output requires package process",
			declaration: TestIsolationDeclaration{
				Hazard: TestIsolationHazardProcessOutput,
				Scope:  TestIsolationScopePackageProcess,
			},
		},
		{
			name: "process logger requires package process",
			declaration: TestIsolationDeclaration{
				Hazard: TestIsolationHazardProcessLogger,
				Scope:  TestIsolationScopePackageProcess,
			},
		},
		{
			name: "global registry requires package process",
			declaration: TestIsolationDeclaration{
				Hazard: TestIsolationHazardGlobalRegistry,
				Scope:  TestIsolationScopePackageProcess,
			},
		},
		{
			name: "runtime allocation requires package process",
			declaration: TestIsolationDeclaration{
				Hazard: TestIsolationHazardRuntimeAllocation,
				Scope:  TestIsolationScopePackageProcess,
			},
		},
		{
			name: "sibling order requires sibling table",
			declaration: TestIsolationDeclaration{
				Hazard: TestIsolationHazardSiblingOrder,
				Scope:  TestIsolationScopeSiblingTable,
			},
		},
		{
			name:        "zero declaration rejected",
			declaration: TestIsolationDeclaration{},
			wantErr:     ErrTestIsolationContract,
		},
		{
			name: "zero hazard rejected",
			declaration: TestIsolationDeclaration{
				Scope: TestIsolationScopePackageProcess,
			},
			wantErr: ErrTestIsolationContract,
		},
		{
			name: "zero scope rejected",
			declaration: TestIsolationDeclaration{
				Hazard: TestIsolationHazardProcessEnvironment,
			},
			wantErr: ErrTestIsolationContract,
		},
		{
			name: "process hazard cannot claim sibling table",
			declaration: TestIsolationDeclaration{
				Hazard: TestIsolationHazardProcessEnvironment,
				Scope:  TestIsolationScopeSiblingTable,
			},
			wantErr: ErrTestIsolationContract,
		},
		{
			name: "sibling order cannot claim package process",
			declaration: TestIsolationDeclaration{
				Hazard: TestIsolationHazardSiblingOrder,
				Scope:  TestIsolationScopePackageProcess,
			},
			wantErr: ErrTestIsolationContract,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			gotErr := tc.declaration.Validate()
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("TestIsolationDeclaration.Validate() error = %v, want %v", gotErr, tc.wantErr)
			}
			if tc.wantErr != nil && !errors.Is(gotErr, ErrPrimitiveContract) {
				t.Fatalf(
					"TestIsolationDeclaration.Validate() error = %v, want parent %v",
					gotErr,
					ErrPrimitiveContract,
				)
			}
		})
	}
}

func TestIsolationAnalyzerIdentifiersExhaustClosedDomainsAndRejectLookalikes(t *testing.T) {
	t.Parallel()

	for hazard := TestIsolationHazardProcessEnvironment; hazard < testIsolationHazardLimit; hazard++ {
		identifier := hazard.GoIdentifier()
		if identifier == "" {
			t.Fatalf("TestIsolationHazard(%d).GoIdentifier() = empty, want exact identifier", hazard)
		}
		got, gotErr := parseTestIsolationHazardGoIdentifier(identifier)
		if gotErr != nil || got != hazard {
			t.Fatalf(
				"ParseTestIsolationHazardGoIdentifier(%q) = (%d, %v), want (%d, nil)",
				identifier,
				got,
				gotErr,
				hazard,
			)
		}
	}
	for scope := TestIsolationScopeSiblingTable; scope < testIsolationScopeLimit; scope++ {
		identifier := scope.GoIdentifier()
		if identifier == "" {
			t.Fatalf("TestIsolationScope(%d).GoIdentifier() = empty, want exact identifier", scope)
		}
		got, gotErr := parseTestIsolationScopeGoIdentifier(identifier)
		if gotErr != nil || got != scope {
			t.Fatalf(
				"ParseTestIsolationScopeGoIdentifier(%q) = (%d, %v), want (%d, nil)",
				identifier,
				got,
				gotErr,
				scope,
			)
		}
	}
	// The loops above prove every admitted value projects a non-empty
	// identifier. These rows prove the other half: every value outside the
	// closed domain projects the empty identifier. Without both halves an enum
	// member added without a switch case would project "" and, before the
	// explicit empty-identifier gate, parse straight back to itself.
	invalidHazards := []struct {
		name   string
		hazard TestIsolationHazard
	}{
		{name: "unknown zero hazard", hazard: TestIsolationHazardUnknown},
		{name: "exact hazard limit", hazard: testIsolationHazardLimit},
		{name: "one past hazard limit", hazard: testIsolationHazardLimit + 1},
		{name: "maximum hazard value", hazard: TestIsolationHazard(math.MaxUint8)},
	}
	for _, tc := range invalidHazards {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := tc.hazard.GoIdentifier(); got != "" {
				t.Fatalf("TestIsolationHazard(%d).GoIdentifier() = %q, want empty", tc.hazard, got)
			}
			if err := tc.hazard.Validate(); !errors.Is(err, ErrTestIsolationContract) {
				t.Fatalf("TestIsolationHazard(%d).Validate() = %v, want %v", tc.hazard, err, ErrTestIsolationContract)
			}
		})
	}
	invalidScopes := []struct {
		name  string
		scope TestIsolationScope
	}{
		{name: "unknown zero scope", scope: TestIsolationScopeUnknown},
		{name: "exact scope limit", scope: testIsolationScopeLimit},
		{name: "one past scope limit", scope: testIsolationScopeLimit + 1},
		{name: "maximum scope value", scope: TestIsolationScope(math.MaxUint8)},
	}
	for _, tc := range invalidScopes {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := tc.scope.GoIdentifier(); got != "" {
				t.Fatalf("TestIsolationScope(%d).GoIdentifier() = %q, want empty", tc.scope, got)
			}
			if err := tc.scope.Validate(); !errors.Is(err, ErrTestIsolationContract) {
				t.Fatalf("TestIsolationScope(%d).Validate() = %v, want %v", tc.scope, err, ErrTestIsolationContract)
			}
		})
	}
	hazardLookalikes := []struct {
		name       string
		identifier string
	}{
		{name: "empty identifier"},
		{name: "unknown zero identifier", identifier: "TestIsolationHazardUnknown"},
		{name: "valid identifier with suffix", identifier: TestIsolationHazardProcessEnvironment.GoIdentifier() + "Extra"},
		{name: "local prefix before valid identifier", identifier: "Local" + TestIsolationHazardProcessEnvironment.GoIdentifier()},
		{name: "scope identifier in hazard position", identifier: TestIsolationScopePackageProcess.GoIdentifier()},
	}
	for _, tc := range hazardLookalikes {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := parseTestIsolationHazardGoIdentifier(tc.identifier)
			if got != TestIsolationHazardUnknown ||
				!errors.Is(gotErr, ErrTestIsolationContract) ||
				!errors.Is(gotErr, ErrPrimitiveContract) {
				t.Fatalf(
					"ParseTestIsolationHazardGoIdentifier(%q) = (%d, %v), want (%d, %v/%v)",
					tc.identifier,
					got,
					gotErr,
					TestIsolationHazardUnknown,
					ErrTestIsolationContract,
					ErrPrimitiveContract,
				)
			}
		})
	}
	scopeLookalikes := []struct {
		name       string
		identifier string
	}{
		{name: "empty scope identifier"},
		{name: "unknown zero scope identifier", identifier: "TestIsolationScopeUnknown"},
		{name: "valid scope identifier with suffix", identifier: TestIsolationScopePackageProcess.GoIdentifier() + "Extra"},
		{name: "local prefix before valid scope identifier", identifier: "Local" + TestIsolationScopePackageProcess.GoIdentifier()},
		{name: "hazard identifier in scope position", identifier: TestIsolationHazardProcessEnvironment.GoIdentifier()},
	}
	for _, tc := range scopeLookalikes {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := parseTestIsolationScopeGoIdentifier(tc.identifier)
			if got != TestIsolationScopeUnknown ||
				!errors.Is(gotErr, ErrTestIsolationContract) ||
				!errors.Is(gotErr, ErrPrimitiveContract) {
				t.Fatalf(
					"ParseTestIsolationScopeGoIdentifier(%q) = (%d, %v), want (%d, %v/%v)",
					tc.identifier,
					got,
					gotErr,
					TestIsolationScopeUnknown,
					ErrTestIsolationContract,
					ErrPrimitiveContract,
				)
			}
		})
	}
}

func TestIsolationAnalyzerNamingContractUsesCanonicalPrimitivePaths(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		got  string
		want string
	}{
		{name: "Core package path", got: TestIsolationCorePackagePath, want: PrimitivePackagePathPrefix + "core"},
		{name: "declaration package path", got: TestIsolationDeclarationPackagePath, want: PrimitivePackagePathPrefix + PackageTestSerial.String()},
		{name: "declaration function", got: TestIsolationDeclarationFunctionName, want: "Declare"},
		{name: "declaration type", got: TestIsolationDeclarationTypeName, want: "TestIsolationDeclaration"},
		{name: "hazard field", got: TestIsolationDeclarationHazardFieldName, want: "Hazard"},
		{name: "scope field", got: TestIsolationDeclarationScopeFieldName, want: "Scope"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if tc.got != tc.want {
				t.Fatalf("test-isolation analyzer contract = %q, want %q", tc.got, tc.want)
			}
		})
	}
}

func TestDepthTwoErrorIdentitiesPreserveCallerDecisions(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		produced   ErrorIdentity
		wantMatch  ErrorIdentity
		wantReject ErrorIdentity
	}{
		{
			name:       "indeterminate activation remains separate from cleanup",
			produced:   ErrFilestoreActivationIndeterminate,
			wantMatch:  ErrFilestoreActivation,
			wantReject: ErrFilestoreCleanup,
		},
		{
			name:       "keygen entropy remains separate from input contract",
			produced:   ErrKeygenEntropy,
			wantMatch:  ErrKeygenContract,
			wantReject: ErrCurrencyContract,
		},
		{
			name:       "test isolation remains separate from runtime packages",
			produced:   ErrTestIsolationContract,
			wantMatch:  ErrPrimitiveContract,
			wantReject: ErrKeygenContract,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if !errors.Is(tc.produced, tc.wantMatch) {
				t.Fatalf("errors.Is(%v, %v) = false, want true", tc.produced, tc.wantMatch)
			}
			if errors.Is(tc.produced, tc.wantReject) {
				t.Fatalf("errors.Is(%v, %v) = true, want false", tc.produced, tc.wantReject)
			}
		})
	}
}
