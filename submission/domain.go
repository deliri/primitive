package submission

import (
	"encoding"
	"encoding/json"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/core"
)

const (
	// SigningDomainRequestV1Token is the exact namespace for a device's
	// evidence-submission request.
	SigningDomainRequestV1Token = "primitive-submission-request-2026-1"
	// SigningDomainGrantV1Token is the exact namespace for an authority's
	// evidence-submission grant.
	SigningDomainGrantV1Token = "primitive-submission-grant-2026-1"
	// SigningDomainCompletionV1Token is the exact namespace for a device's
	// provider-confirmed upload completion.
	SigningDomainCompletionV1Token = "primitive-submission-completion-2026-1"
)

// SigningDomain is the closed set of evidence-submission signature namespaces.
type SigningDomain uint8

const (
	SigningDomainUnknown SigningDomain = iota
	SigningDomainRequestV1
	SigningDomainGrantV1
	SigningDomainCompletionV1
	signingDomainLimit
)

func signingDomainTokens() [signingDomainLimit]string {
	return [...]string{
		SigningDomainUnknown:      "",
		SigningDomainRequestV1:    SigningDomainRequestV1Token,
		SigningDomainGrantV1:      SigningDomainGrantV1Token,
		SigningDomainCompletionV1: SigningDomainCompletionV1Token,
	}
}

// Validate rejects the unset domain and every unpublished domain.
func (d SigningDomain) Validate() error {
	if d <= SigningDomainUnknown || d >= signingDomainLimit || signingDomainTokens()[d] == "" {
		return contractError(core.ErrControlPlaneSigningDomain)
	}
	return nil
}

// IsValid reports whether d is one of the published namespaces.
func (d SigningDomain) IsValid() bool { return d.Validate() == nil }

// String returns the exact signed namespace, or empty text when invalid.
func (d SigningDomain) String() string {
	if d >= signingDomainLimit {
		return ""
	}
	return signingDomainTokens()[d]
}

// MarshalText emits the exact signed namespace.
func (d SigningDomain) MarshalText() ([]byte, error) {
	if err := d.Validate(); err != nil {
		return nil, err
	}
	return []byte(d.String()), nil
}

// ParseCanonicalText accepts only one exact published namespace.
func (SigningDomain) ParseCanonicalText(text []byte) (SigningDomain, error) {
	if len(text) > attest.SigningDomainMaximumBytes {
		return SigningDomainUnknown, contractError(core.ErrControlPlaneSigningDomain)
	}
	for domain := SigningDomainUnknown + 1; domain < signingDomainLimit; domain++ {
		if signingDomainTokens()[domain] == string(text) {
			return domain, nil
		}
	}
	return SigningDomainUnknown, contractError(core.ErrControlPlaneSigningDomain)
}

// ParseSigningDomain accepts one exact published namespace.
func ParseSigningDomain(value string) (SigningDomain, error) {
	return SigningDomainUnknown.ParseCanonicalText([]byte(value))
}

// MarshalJSON emits the same canonical namespace MarshalText signs.
func (d SigningDomain) MarshalJSON() ([]byte, error) {
	if err := d.Validate(); err != nil {
		return nil, jsonError(err)
	}
	encoded, err := core.MarshalCanonicalJSONString(d.String())
	if err != nil {
		return nil, jsonError(err)
	}
	return encoded, nil
}

// UnmarshalJSON accepts only a published namespace and preserves the receiver
// on every refusal.
func (d *SigningDomain) UnmarshalJSON(data []byte) error {
	if d == nil {
		return jsonError(core.ErrControlPlaneSigningDomain)
	}
	value, err := core.DecodeJSONStringToken(data)
	if err != nil {
		return jsonError(err)
	}
	parsed, err := ParseSigningDomain(value)
	if err != nil {
		return jsonError(err)
	}
	*d = parsed
	return nil
}

type signingDomainWitness[D attest.SigningDomain[D]] [0]D

var (
	_ core.Validatable            = SigningDomainUnknown
	_ core.ValidatedJSONMarshaler = SigningDomain(0)
	_ encoding.TextMarshaler      = SigningDomainUnknown
	_ json.Marshaler              = SigningDomainUnknown
	_ json.Unmarshaler            = (*SigningDomain)(nil)
	_                             = signingDomainWitness[SigningDomain]{}
)
