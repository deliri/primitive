package core

import "errors"

// PackageRole is the one primary architectural responsibility of a package.
// Supporting traits remain orthogonal generated facts and never replace it.
type PackageRole uint8

const (
	// PackageRoleUnknown is the invalid zero package role.
	PackageRoleUnknown PackageRole = iota
	// PackageRoleValueContract owns validated nominal values and shared scalar contracts.
	PackageRoleValueContract
	// PackageRoleDomainAgreement owns one bidirectional typed agreement shared by independently deployed peers.
	PackageRoleDomainAgreement
	// PackageRoleAuthenticationBinding turns untrusted documents into verified nominal values.
	PackageRoleAuthenticationBinding
	// PackageRoleEffectCapability is the single owner of one or more real-world effects.
	PackageRoleEffectCapability
	// PackageRoleWireProtocol transports bounded typed documents without owning their meaning.
	PackageRoleWireProtocol
	// PackageRoleOrchestration composes typed policy and capabilities without becoming their owner.
	PackageRoleOrchestration
	packageRoleLimit
)

// Validate rejects the zero and every value outside the closed role domain.
func (r PackageRole) Validate() error {
	if r <= PackageRoleUnknown || r >= packageRoleLimit {
		return architectureContractError("package role is outside the admitted domain")
	}
	return nil
}

// IsValid reports whether r belongs to the closed role domain.
func (r PackageRole) IsValid() bool { return r.Validate() == nil }

// String returns the canonical role token or empty text for an invalid role.
func (r PackageRole) String() string {
	if !r.IsValid() {
		return ""
	}
	return packageRoleTexts()[r]
}

// MarshalJSON emits the canonical role token.
func (r PackageRole) MarshalJSON() ([]byte, error) {
	if err := r.Validate(); err != nil {
		return nil, errors.Join(ErrJSONContract, err)
	}
	return MarshalCanonicalJSONString(r.String())
}

// UnmarshalJSON admits only a canonical token from the closed role domain.
func (r *PackageRole) UnmarshalJSON(data []byte) error {
	if r == nil {
		return errors.Join(ErrJSONContract, architectureContractError("nil package role receiver"))
	}
	value, err := DecodeJSONStringToken(data)
	if err != nil {
		return err
	}
	for candidate := PackageRoleValueContract; candidate < packageRoleLimit; candidate++ {
		if candidate.String() == value {
			*r = candidate
			return nil
		}
	}
	return errors.Join(ErrJSONContract, architectureContractError("package role text is not admitted"))
}

func packageRoleTexts() [packageRoleLimit]string {
	return [...]string{
		PackageRoleValueContract:         "value_contract",
		PackageRoleDomainAgreement:       "domain_agreement",
		PackageRoleAuthenticationBinding: "authentication_binding",
		PackageRoleEffectCapability:      "effect_capability",
		PackageRoleWireProtocol:          "wire_protocol",
		PackageRoleOrchestration:         "orchestration",
	}
}
