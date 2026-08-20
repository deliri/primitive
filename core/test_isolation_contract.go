package core

import "errors"

const (
	// TestIsolationCorePackagePath is the exact Core import recognized by the
	// pinned test-isolation analyzer.
	TestIsolationCorePackagePath = PrimitivePackagePathPrefix + "core"
	// TestIsolationDeclarationPackagePath is the exact declaration package
	// import recognized by the pinned test-isolation analyzer.
	TestIsolationDeclarationPackagePath = PrimitivePackagePathPrefix + "testserial"
	// TestIsolationDeclarationFunctionName is the exact declaration function
	// recognized by the testserial package and its pinned analyzer.
	TestIsolationDeclarationFunctionName = "Declare"
	// TestIsolationDeclarationTypeName is the exact aggregate declaration type
	// recognized by the pinned test-isolation analyzer.
	TestIsolationDeclarationTypeName = "TestIsolationDeclaration"
	// TestIsolationDeclarationHazardFieldName is the aggregate hazard field.
	TestIsolationDeclarationHazardFieldName = "Hazard"
	// TestIsolationDeclarationScopeFieldName is the aggregate scope field.
	TestIsolationDeclarationScopeFieldName = "Scope"
)

// TestIsolationHazard identifies a generic shared-state hazard that prevents a
// Go test from safely running in parallel.
type TestIsolationHazard uint8

const (
	// TestIsolationHazardUnknown is the invalid zero hazard.
	TestIsolationHazardUnknown TestIsolationHazard = iota
	// TestIsolationHazardProcessEnvironment identifies process environment mutation.
	TestIsolationHazardProcessEnvironment
	// TestIsolationHazardProcessWorkingDirectory identifies working-directory mutation.
	TestIsolationHazardProcessWorkingDirectory
	// TestIsolationHazardProcessSignal identifies process signal-handler mutation.
	TestIsolationHazardProcessSignal
	// TestIsolationHazardProcessOutput identifies process output redirection.
	TestIsolationHazardProcessOutput
	// TestIsolationHazardProcessLogger identifies process-global logger mutation.
	TestIsolationHazardProcessLogger
	// TestIsolationHazardGlobalRegistry identifies a mutable package-global registry.
	TestIsolationHazardGlobalRegistry
	// TestIsolationHazardRuntimeAllocation identifies process-wide allocation observation.
	TestIsolationHazardRuntimeAllocation
	// TestIsolationHazardSiblingOrder identifies deliberately ordered sibling subtests.
	TestIsolationHazardSiblingOrder
	testIsolationHazardLimit
)

func testIsolationHazardDiagnostics() [testIsolationHazardLimit]string {
	return [...]string{
		TestIsolationHazardProcessEnvironment:      "process-environment",
		TestIsolationHazardProcessWorkingDirectory: "process-working-directory",
		TestIsolationHazardProcessSignal:           "process-signal",
		TestIsolationHazardProcessOutput:           "process-output",
		TestIsolationHazardProcessLogger:           "process-logger",
		TestIsolationHazardGlobalRegistry:          "global-registry",
		TestIsolationHazardRuntimeAllocation:       "runtime-allocation",
		TestIsolationHazardSiblingOrder:            "sibling-order",
	}
}

// IsValid reports whether h belongs to the closed hazard domain.
func (h TestIsolationHazard) IsValid() bool {
	return h > TestIsolationHazardUnknown && h < testIsolationHazardLimit &&
		testIsolationHazardDiagnostics()[h] != ""
}

// OffWireEnum declares TestIsolationHazard as analyzer policy rather than a
// wire encoding.
func (TestIsolationHazard) OffWireEnum() {}

// String returns the compiler-owned diagnostic label for h.
func (h TestIsolationHazard) String() string {
	if !h.IsValid() {
		return UnknownEnumDiagnostic
	}
	return testIsolationHazardDiagnostics()[h]
}

// Validate rejects hazards outside the closed domain.
func (h TestIsolationHazard) Validate() error {
	if !h.IsValid() {
		return testIsolationContractError("test isolation hazard is outside the admitted domain")
	}
	return nil
}

// GoIdentifier returns the exact Core constant identifier recognized by the
// pinned test-isolation analyzer.
func (h TestIsolationHazard) GoIdentifier() string {
	if !h.IsValid() {
		return ""
	}
	return testIsolationHazardGoIdentifiers()[h]
}

func testIsolationHazardGoIdentifiers() [testIsolationHazardLimit]string {
	return [...]string{
		TestIsolationHazardProcessEnvironment:      "TestIsolationHazardProcessEnvironment",
		TestIsolationHazardProcessWorkingDirectory: "TestIsolationHazardProcessWorkingDirectory",
		TestIsolationHazardProcessSignal:           "TestIsolationHazardProcessSignal",
		TestIsolationHazardProcessOutput:           "TestIsolationHazardProcessOutput",
		TestIsolationHazardProcessLogger:           "TestIsolationHazardProcessLogger",
		TestIsolationHazardGlobalRegistry:          "TestIsolationHazardGlobalRegistry",
		TestIsolationHazardRuntimeAllocation:       "TestIsolationHazardRuntimeAllocation",
		TestIsolationHazardSiblingOrder:            "TestIsolationHazardSiblingOrder",
	}
}

// parseTestIsolationHazardGoIdentifier resolves only an exact Core-owned
// hazard constant identifier.
func parseTestIsolationHazardGoIdentifier(
	identifier string,
) (TestIsolationHazard, error) {
	// GoIdentifier answers the empty string for every value outside the closed
	// domain. Without this gate, a hazard added to the enum but not to the
	// switch would make the empty identifier resolve to that hazard.
	if identifier != "" {
		for hazard := TestIsolationHazardProcessEnvironment; hazard < testIsolationHazardLimit; hazard++ {
			if hazard.GoIdentifier() == identifier {
				return hazard, nil
			}
		}
	}
	return TestIsolationHazardUnknown, testIsolationContractError(
		"test isolation hazard Go identifier is not admitted",
	)
}

// TestIsolationScope identifies the concurrency boundary protected by a test
// isolation declaration.
type TestIsolationScope uint8

const (
	// TestIsolationScopeUnknown is the invalid zero scope.
	TestIsolationScopeUnknown TestIsolationScope = iota
	// TestIsolationScopeSiblingTable protects ordering among sibling subtests.
	TestIsolationScopeSiblingTable
	// TestIsolationScopePackageProcess protects process-global state shared by package tests.
	TestIsolationScopePackageProcess
	testIsolationScopeLimit
)

func testIsolationScopeDiagnostics() [testIsolationScopeLimit]string {
	return [...]string{
		TestIsolationScopeSiblingTable:   "sibling-table",
		TestIsolationScopePackageProcess: "package-process",
	}
}

// IsValid reports whether s belongs to the closed scope domain.
func (s TestIsolationScope) IsValid() bool {
	return s > TestIsolationScopeUnknown && s < testIsolationScopeLimit &&
		testIsolationScopeDiagnostics()[s] != ""
}

// OffWireEnum declares TestIsolationScope as analyzer policy rather than a
// wire encoding.
func (TestIsolationScope) OffWireEnum() {}

// String returns the compiler-owned diagnostic label for s.
func (s TestIsolationScope) String() string {
	if !s.IsValid() {
		return UnknownEnumDiagnostic
	}
	return testIsolationScopeDiagnostics()[s]
}

// Validate rejects scopes outside the closed domain.
func (s TestIsolationScope) Validate() error {
	if !s.IsValid() {
		return testIsolationContractError("test isolation scope is outside the admitted domain")
	}
	return nil
}

// GoIdentifier returns the exact Core constant identifier recognized by the
// pinned test-isolation analyzer.
func (s TestIsolationScope) GoIdentifier() string {
	if !s.IsValid() {
		return ""
	}
	return testIsolationScopeGoIdentifiers()[s]
}

func testIsolationScopeGoIdentifiers() [testIsolationScopeLimit]string {
	return [...]string{
		TestIsolationScopeSiblingTable:   "TestIsolationScopeSiblingTable",
		TestIsolationScopePackageProcess: "TestIsolationScopePackageProcess",
	}
}

// parseTestIsolationScopeGoIdentifier resolves only an exact Core-owned scope
// constant identifier.
func parseTestIsolationScopeGoIdentifier(
	identifier string,
) (TestIsolationScope, error) {
	// The empty identifier is rejected for the same reason as the hazard
	// domain: it is what GoIdentifier answers outside the closed domain.
	if identifier != "" {
		for scope := TestIsolationScopeSiblingTable; scope < testIsolationScopeLimit; scope++ {
			if scope.GoIdentifier() == identifier {
				return scope, nil
			}
		}
	}
	return TestIsolationScopeUnknown, testIsolationContractError(
		"test isolation scope Go identifier is not admitted",
	)
}

// TestIsolationDeclaration binds one generic hazard to its exact enforced
// concurrency scope.
type TestIsolationDeclaration struct {
	// Hazard identifies the shared-state behavior under test.
	Hazard TestIsolationHazard
	// Scope identifies the boundary that must remain non-parallel.
	Scope TestIsolationScope
}

// Validate rejects incomplete declarations and mismatched hazard/scope pairs.
func (d TestIsolationDeclaration) Validate() error {
	if err := d.Hazard.Validate(); err != nil {
		return err
	}
	if err := d.Scope.Validate(); err != nil {
		return err
	}
	if d.Hazard == TestIsolationHazardSiblingOrder {
		if d.Scope != TestIsolationScopeSiblingTable {
			return testIsolationContractError("sibling-order hazard requires sibling-table scope")
		}
		return nil
	}
	if d.Scope != TestIsolationScopePackageProcess {
		return testIsolationContractError("process-global hazard requires package-process scope")
	}
	return nil
}

func testIsolationContractError(message string) error {
	return errors.Join(ErrTestIsolationContract, errors.New(message))
}
