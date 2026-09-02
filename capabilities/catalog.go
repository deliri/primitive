// Package capabilities exposes Primitive's compiler-owned package and
// real-world effect catalog. It derives package facts from core's architecture
// catalog; Markdown and runtime registration are not authority surfaces.
package capabilities

import (
	"errors"
	"iter"

	"github.com/deliri/primitive/v2026/core"
)

const purposeMaximumBytes = 4096

// Purpose is a bounded human explanation attached to a compiler-owned package
// identity. It helps a caller choose; it is never the matching key.
type Purpose struct {
	value string
}

// newPurpose admits compiler-owned descriptive text. Purpose is human guidance
// only; package identity and effect enums remain the matching authority.
func newPurpose(value string) (Purpose, error) {
	purpose := Purpose{value: value}
	if err := purpose.Validate(); err != nil {
		return Purpose{}, err
	}
	return purpose, nil
}

// Validate rejects absent or unbounded purpose text.
func (p Purpose) Validate() error {
	if len(p.value) == 0 || len(p.value) > purposeMaximumBytes {
		return contractError("capability purpose is absent or exceeds its bound")
	}
	return nil
}

// String returns the admitted descriptive text or an empty string when invalid.
func (p Purpose) String() string {
	if p.Validate() != nil {
		return ""
	}
	return p.value
}

// Capability describes one actual Primitive package.
type Capability struct {
	Purpose Purpose
	Package core.PackageIdentity
	Kind    core.PackageKind
	Role    core.PackageRole
}

// Description returns the bounded human guidance for this capability.
func (c Capability) Description() Purpose { return c.Purpose }

// Validate binds the capability to the authoritative Primitive architecture.
func (c Capability) Validate() error {
	if err := c.Package.Validate(); err != nil {
		return errors.Join(core.ErrCapabilitiesContract, err)
	}
	if err := c.Kind.Validate(); err != nil {
		return errors.Join(core.ErrCapabilitiesContract, err)
	}
	if err := c.Purpose.Validate(); err != nil {
		return err
	}
	want, err := capabilityFor(c.Package)
	if err != nil {
		return err
	}
	if c != want {
		return contractError("capability contradicts the Primitive architecture")
	}
	return nil
}

// ImportPath returns the exact import path that supplies this capability.
func (c Capability) ImportPath() (string, error) {
	if err := c.Validate(); err != nil {
		return "", err
	}
	return c.Package.ImportPath()
}

// Owns reports whether this package is the canonical product-facing owner of
// effect. It returns false for invalid effects and unrelated capabilities.
func (c Capability) Owns(effect Effect) bool {
	owner, err := effectOwner(effect)
	return err == nil && owner == c.Package
}

// Catalog is the complete validated view of Primitive's compiled architecture.
type Catalog struct {
	architecture core.ArchitectureCatalog
}

// All returns every Primitive capability from the compiler-owned architecture.
func All() (Catalog, error) {
	catalog := Catalog{architecture: core.PrimitiveArchitecture()}
	if err := catalog.Validate(); err != nil {
		return Catalog{}, err
	}
	return catalog, nil
}

// Validate proves that the underlying architecture and every derived
// capability are internally consistent.
func (c Catalog) Validate() error {
	if err := c.architecture.Validate(); err != nil {
		return errors.Join(core.ErrCapabilitiesContract, err)
	}
	count := 0
	var roleCounts [core.PrimitivePackageCount]uint16
	for contract := range c.architecture.Packages() {
		capability, err := capabilityFromContract(contract)
		if err != nil {
			return err
		}
		if err := capability.Validate(); err != nil {
			return err
		}
		roleCounts[int(contract.Role-core.PackageRoleValueContract)]++
		count++
	}
	if count != core.PrimitivePackageCount {
		return contractError("capability count contradicts the Primitive architecture")
	}
	for _, roleCount := range roleCounts[:int(core.PackageRoleOrchestration-core.PackageRoleValueContract)+1] {
		if roleCount == 0 {
			return contractError("Primitive architecture omits an admitted package role")
		}
	}
	return validateEffectOwners(c)
}

// Capabilities yields each package capability in deterministic architecture
// order without constructing a second mutable collection.
func (c Catalog) Capabilities() iter.Seq[Capability] {
	return func(yield func(Capability) bool) {
		for contract := range c.architecture.Packages() {
			capability, err := capabilityFromContract(contract)
			if err != nil || !yield(capability) {
				return
			}
		}
	}
}

func capabilityFor(identity core.PackageIdentity) (Capability, error) {
	architecture := core.PrimitiveArchitecture()
	if err := architecture.Validate(); err != nil {
		return Capability{}, errors.Join(core.ErrCapabilitiesContract, err)
	}
	contract, found := architecture.Lookup(identity)
	if !found {
		return Capability{}, unavailableError("package is not present in the Primitive architecture")
	}
	return capabilityFromContract(contract)
}

func capabilityFromContract(contract core.PackageContract) (Capability, error) {
	if err := contract.Validate(); err != nil {
		return Capability{}, errors.Join(core.ErrCapabilitiesContract, err)
	}
	purpose, err := contract.Identity.Purpose()
	if err != nil {
		return Capability{}, errors.Join(core.ErrCapabilitiesContract, err)
	}
	admittedPurpose, err := newPurpose(purpose)
	if err != nil {
		return Capability{}, err
	}
	capability := Capability{Package: contract.Identity, Kind: contract.Kind, Role: contract.Role, Purpose: admittedPurpose}
	return capability, nil
}

func validateEffectOwners(c Catalog) error {
	for effect := EffectFilesystem; effect < effectLimit; effect++ {
		owner, err := effectOwner(effect)
		if err != nil {
			return err
		}
		contract, found := c.architecture.Lookup(owner)
		if !found || contract.Kind != core.PackageKindProduction || contract.Role != core.PackageRoleEffectCapability {
			return contractError("real-world effect lacks one production capability owner")
		}
	}
	return nil
}

func contractError(message string) error {
	return errors.Join(core.ErrCapabilitiesContract, errors.New(message))
}

func unavailableError(message string) error {
	return errors.Join(core.ErrCapabilityUnavailable, errors.New(message))
}
