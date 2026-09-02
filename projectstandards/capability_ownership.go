package projectstandards

import (
	"errors"

	"github.com/deliri/primitive/v2026/capabilities"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gomodule"
)

const (
	capabilityRoleInvalidDiagnostic      = "project standards capability role is invalid"
	projectRelationshipInvalidDiagnostic = "project standards project relationship is invalid"
)

// CapabilityRole states why a package is allowed to implement a real-world
// capability directly instead of reaching it through Primitive.
type CapabilityRole uint8

const (
	CapabilityRoleUnknown CapabilityRole = iota
	CapabilityRolePrimitiveImplementation
	CapabilityRoleDeclaredAdapter
	capabilityRoleLimit
)

func capabilityRoleLabels() []string {
	return []string{"", "primitive_implementation", "declared_adapter"}
}

func (r CapabilityRole) Validate() error {
	return validateEnum(uint8(r), capabilityRoleLabels(), capabilityRoleInvalidDiagnostic)
}

func (r CapabilityRole) IsValid() bool  { return r.Validate() == nil }
func (r CapabilityRole) String() string { return enumString(uint8(r), capabilityRoleLabels()) }
func (r CapabilityRole) MarshalJSON() ([]byte, error) {
	return marshalEnum(uint8(r), capabilityRoleLabels(), capabilityRoleInvalidDiagnostic)
}
func (r *CapabilityRole) UnmarshalJSON(data []byte) error {
	if r == nil {
		return jsonError(errors.New("nil project standards capability role receiver"))
	}
	value, err := unmarshalEnum(data, capabilityRoleLabels(), capabilityRoleInvalidDiagnostic)
	if err == nil {
		*r = CapabilityRole(value)
	}
	return err
}

// CapabilityOwnership is hand-authored policy declaring one real-world
// capability that the package implements directly.
type CapabilityOwnership struct {
	Capability capabilities.Identity `json:"capability"`
	Role       CapabilityRole        `json:"role"`
}

func (o CapabilityOwnership) Validate() error {
	return contractJoin(o.Capability.Validate(), o.Role.Validate())
}

// ProjectRelationship identifies how one Go module relates to Primitive's
// real-world capability boundary.
type ProjectRelationship uint8

const (
	ProjectRelationshipUnknown ProjectRelationship = iota
	ProjectRelationshipPrimitive
	ProjectRelationshipProduct
	ProjectRelationshipDeclaredAdapter
	projectRelationshipLimit
)

func projectRelationshipLabels() []string {
	return []string{"", "primitive", "product", "declared_adapter"}
}

func (r ProjectRelationship) Validate() error {
	return validateEnum(uint8(r), projectRelationshipLabels(), projectRelationshipInvalidDiagnostic)
}

func (r ProjectRelationship) IsValid() bool { return r.Validate() == nil }
func (r ProjectRelationship) String() string {
	return enumString(uint8(r), projectRelationshipLabels())
}
func (r ProjectRelationship) MarshalJSON() ([]byte, error) {
	return marshalEnum(uint8(r), projectRelationshipLabels(), projectRelationshipInvalidDiagnostic)
}
func (r *ProjectRelationship) UnmarshalJSON(data []byte) error {
	if r == nil {
		return jsonError(errors.New("nil project standards project relationship receiver"))
	}
	value, err := unmarshalEnum(data, projectRelationshipLabels(), projectRelationshipInvalidDiagnostic)
	if err == nil {
		*r = ProjectRelationship(value)
	}
	return err
}

// ResolveProjectRelationship derives project posture from the validated Go
// module identity and authored ownership roles. Repository paths and package
// name allowlists are deliberately not inputs.
func ResolveProjectRelationship(module gomodule.Path, ownership []CapabilityOwnership) (ProjectRelationship, error) {
	if err := module.Validate(); err != nil {
		return ProjectRelationshipUnknown, contractError(err)
	}
	if err := validateCapabilityOwnership(ownership); err != nil {
		return ProjectRelationshipUnknown, err
	}
	if module.String() == core.PrimitiveModulePath {
		for _, declared := range ownership {
			if declared.Role != CapabilityRolePrimitiveImplementation {
				return ProjectRelationshipUnknown, conflictError(errors.New("primitive module declares a non-Primitive capability role"))
			}
		}
		return ProjectRelationshipPrimitive, nil
	}
	if len(ownership) == 0 {
		return ProjectRelationshipProduct, nil
	}
	for _, declared := range ownership {
		if declared.Role != CapabilityRoleDeclaredAdapter {
			return ProjectRelationshipUnknown, conflictError(errors.New("external module declares a Primitive implementation role"))
		}
	}
	return ProjectRelationshipDeclaredAdapter, nil
}

// ResolvePackageRelationship additionally binds Primitive implementation
// declarations to the canonical package identity that owns the capability.
func ResolvePackageRelationship(module gomodule.Path, packagePath SourcePath, ownership []CapabilityOwnership) (ProjectRelationship, error) {
	if err := packagePath.Validate(); err != nil {
		return ProjectRelationshipUnknown, err
	}
	relationship, err := ResolveProjectRelationship(module, ownership)
	if err != nil || relationship != ProjectRelationshipPrimitive {
		return relationship, err
	}
	for _, declared := range ownership {
		effect, effectErr := declared.Capability.Effect()
		if effectErr != nil {
			return ProjectRelationshipUnknown, effectErr
		}
		match, matchErr := capabilities.Resolve(capabilities.ForEffect(capabilities.ScopeProduction, effect))
		if matchErr != nil {
			return ProjectRelationshipUnknown, matchErr
		}
		if packagePath.String() != match.Capability.Package.String() {
			return ProjectRelationshipUnknown, conflictError(errors.New("primitive capability ownership names a different package"))
		}
	}
	return relationship, nil
}

// PackageOwnsCapability reports whether validated authored policy assigns the
// capability to the current package.
func PackageOwnsCapability(values []CapabilityOwnership, capability capabilities.Identity) (bool, error) {
	if err := validateCapabilityOwnership(values); err != nil {
		return false, err
	}
	if err := capability.Validate(); err != nil {
		return false, err
	}
	return ownsCapability(values, capability), nil
}

func validateCapabilityOwnership(values []CapabilityOwnership) error {
	if len(values) > capabilities.IdentityCount {
		return contractError(errors.New("project standards capability ownership exceeds its bound"))
	}
	for index := range values {
		if err := values[index].Validate(); err != nil {
			return err
		}
		for previous := range index {
			if values[previous].Capability == values[index].Capability {
				return conflictError(errors.New("project standards capability ownership is duplicated"))
			}
		}
	}
	return nil
}

func ownsCapability(values []CapabilityOwnership, capability capabilities.Identity) bool {
	for _, declared := range values {
		if declared.Capability == capability {
			return true
		}
	}
	return false
}
