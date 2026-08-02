package lease

import (
	"errors"

	"github.com/deliri/primitive/v2026/core"
)

const decisionDomainToken = "primitive-lease-decision-v1"

// Domain is the closed Attest domain for Lease decisions.
type Domain uint8

const (
	DomainUnknown Domain = iota
	// DomainDecisionV1 separates Lease decisions from every other attestation.
	DomainDecisionV1
	domainLimit
)

func domainDiagnostics() [domainLimit]string {
	return [...]string{
		DomainDecisionV1: decisionDomainToken,
	}
}

func (d Domain) Validate() error {
	if !d.IsValid() {
		return contractError(errors.New("lease signing domain is outside the closed domain"))
	}
	return nil
}

func (d Domain) IsValid() bool {
	return d > DomainUnknown && d < domainLimit && domainDiagnostics()[d] != ""
}

// OffWireEnum declares that Attest owns Domain's canonical text projection;
// Domain itself is never a direct JSON enum.
func (Domain) OffWireEnum() {}

func (d Domain) String() string {
	if !d.IsValid() {
		return unknownDiagnostic
	}
	return domainDiagnostics()[d]
}

// MarshalText emits the exact Attest domain.
func (d Domain) MarshalText() ([]byte, error) {
	if err := d.Validate(); err != nil {
		return nil, err
	}
	return []byte(domainDiagnostics()[d]), nil
}

// ParseCanonicalText reconstructs one exact Attest domain.
func (Domain) ParseCanonicalText(text []byte) (Domain, error) {
	if string(text) != decisionDomainToken {
		return DomainUnknown, contractError(errors.New("lease signing domain text is unsupported"))
	}
	return DomainDecisionV1, nil
}

var (
	_ core.Validatable = DomainUnknown
	_ core.OffWireEnum = DomainUnknown
)
