package capabilities

import "github.com/deliri/primitive/v2026/core"

const (
	requirementTargetPackageName = "package"
	requirementTargetEffectName  = "effect"
	scopeProductionName          = "production"
	scopeTestName                = "test"
)

// RequirementTarget identifies which closed domain a requirement addresses.
type RequirementTarget uint8

const (
	// RequirementTargetUnknown is outside the admitted target domain.
	RequirementTargetUnknown RequirementTarget = iota
	// RequirementTargetPackage requests one exact Primitive package capability.
	RequirementTargetPackage
	// RequirementTargetEffect requests the owner of one real-world effect.
	RequirementTargetEffect
	requirementTargetLimit
)

// Validate rejects values outside the closed requirement-target domain.
func (t RequirementTarget) Validate() error {
	if t <= RequirementTargetUnknown || t >= requirementTargetLimit {
		return contractError("requirement target is outside the admitted domain")
	}
	return nil
}

// IsValid reports whether t belongs to the closed target domain.
func (t RequirementTarget) IsValid() bool { return t.Validate() == nil }

// OffWireEnum marks RequirementTarget as a compiler-only enum.
func (RequirementTarget) OffWireEnum() {}

// String returns the stable doctrine identity of a valid target.
func (t RequirementTarget) String() string {
	if !t.IsValid() {
		return core.UnknownEnumDiagnostic
	}
	if t == RequirementTargetPackage {
		return requirementTargetPackageName
	}
	return requirementTargetEffectName
}

// Scope identifies whether the requesting source is production or test code.
type Scope uint8

const (
	// ScopeUnknown is outside the admitted source scope domain.
	ScopeUnknown Scope = iota
	// ScopeProduction identifies production source.
	ScopeProduction
	// ScopeTest identifies test source.
	ScopeTest
	scopeLimit
)

// Validate rejects values outside the closed source-scope domain.
func (s Scope) Validate() error {
	if s <= ScopeUnknown || s >= scopeLimit {
		return contractError("requirement scope is outside the admitted domain")
	}
	return nil
}

// IsValid reports whether s belongs to the closed source-scope domain.
func (s Scope) IsValid() bool { return s.Validate() == nil }

// OffWireEnum marks Scope as a compiler-only enum.
func (Scope) OffWireEnum() {}

// String returns the stable doctrine identity of a valid scope.
func (s Scope) String() string {
	if !s.IsValid() {
		return core.UnknownEnumDiagnostic
	}
	if s == ScopeProduction {
		return scopeProductionName
	}
	return scopeTestName
}

// Requirement is a closed union: Target selects exactly one of Package or
// Effect, and the inactive field must remain zero.
type Requirement struct {
	Package core.PackageIdentity
	Effect  Effect
	Target  RequirementTarget
	Scope   Scope
}

// ForPackage constructs an exact package requirement.
func ForPackage(scope Scope, pkg core.PackageIdentity) Requirement {
	return Requirement{Package: pkg, Target: RequirementTargetPackage, Scope: scope}
}

// ForEffect constructs an exact real-world effect requirement.
func ForEffect(scope Scope, effect Effect) Requirement {
	return Requirement{Effect: effect, Target: RequirementTargetEffect, Scope: scope}
}

// Validate rejects unknown, mixed, or incomplete requirements.
func (r Requirement) Validate() error {
	if err := r.Scope.Validate(); err != nil {
		return err
	}
	if err := r.Target.Validate(); err != nil {
		return err
	}
	switch r.Target {
	case RequirementTargetPackage:
		if r.Effect != EffectUnknown {
			return contractError("package requirement also names an effect")
		}
		if err := r.Package.Validate(); err != nil {
			return contractError("package requirement names an invalid package")
		}
		return nil
	case RequirementTargetEffect:
		if r.Package != core.PackageUnknown {
			return contractError("effect requirement also names a package")
		}
		return r.Effect.Validate()
	default:
		return contractError("validated requirement target has no validation path")
	}
}

// Match binds one validated requirement to exactly one compiled capability.
type Match struct {
	Capability  Capability
	Requirement Requirement
}

// Validate proves that the match satisfies the requirement and source scope.
func (m Match) Validate() error {
	if err := m.Requirement.Validate(); err != nil {
		return err
	}
	if err := m.Capability.Validate(); err != nil {
		return err
	}
	if m.Requirement.Scope == ScopeProduction && m.Capability.Kind != core.PackageKindProduction {
		return unavailableError("test-support capability is unavailable to production source")
	}
	if m.Requirement.Target == RequirementTargetPackage &&
		m.Requirement.Package != m.Capability.Package {
		return contractError("package match names the wrong capability")
	}
	if m.Requirement.Target == RequirementTargetEffect &&
		!m.Capability.Owns(m.Requirement.Effect) {
		return contractError("effect match names the wrong capability")
	}
	return nil
}

// Resolve matches requirement against Primitive's complete compiled catalog.
func Resolve(requirement Requirement) (Match, error) {
	catalog, err := All()
	if err != nil {
		return Match{}, err
	}
	return catalog.Resolve(requirement)
}

// Resolve matches requirement against c or returns a typed rejection.
func (c Catalog) Resolve(requirement Requirement) (Match, error) {
	if err := c.Validate(); err != nil {
		return Match{}, err
	}
	if err := requirement.Validate(); err != nil {
		return Match{}, err
	}
	identity, err := requirementIdentity(requirement)
	if err != nil {
		return Match{}, err
	}
	capability, err := capabilityFor(identity)
	if err != nil {
		return Match{}, err
	}
	match := Match{Requirement: requirement, Capability: capability}
	if err := match.Validate(); err != nil {
		return Match{}, err
	}
	return match, nil
}

func requirementIdentity(requirement Requirement) (core.PackageIdentity, error) {
	if requirement.Target == RequirementTargetPackage {
		return requirement.Package, nil
	}
	if requirement.Target == RequirementTargetEffect {
		return effectOwner(requirement.Effect)
	}
	return core.PackageUnknown, contractError("validated requirement has no resolution path")
}

var (
	_ core.OffWireEnum = RequirementTargetUnknown
	_ core.OffWireEnum = ScopeUnknown
)
