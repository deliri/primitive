package controlplane

import (
	"encoding"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/core"
)

// These are the exact canonical domain texts. A signature is only meaningful
// inside one of them, so they are spelled once here and never rebuilt from
// parts. Two documents that shared a domain could have one's signature
// presented as the other's.
// #nosec G101 -- these are public namespace labels, not credentials. A domain
// text is emitted verbatim in every signed document and both ends must spell it
// identically for a signature to verify, so it is the opposite of a secret:
// nothing is authorized by knowing one.
const (
	SigningDomainInstallationCertificateV1Token = "ogs-control-installation-certificate-2026-1"
	SigningDomainRegistrationV1Token            = "ogs-control-registration-2026-1"
	SigningDomainCheckInV1Token                 = "ogs-control-check-in-2026-1"
	SigningDomainCheckInResponseV1Token         = "ogs-control-check-in-response-2026-1"
)

// SigningDomain is the closed set of namespaces a control-plane document may
// be signed under.
//
// Closed rather than open on purpose. An unrecognised domain is refused rather
// than carried, because a verifier that accepted an unknown domain would be
// verifying that a signature is valid for something it cannot name.
type SigningDomain uint8

const (
	// SigningDomainUnknown is the unset domain and never signs anything.
	SigningDomainUnknown SigningDomain = iota
	// SigningDomainInstallationCertificateV1 signs one installation's
	// certificate body.
	SigningDomainInstallationCertificateV1
	// SigningDomainRegistrationV1 signs a complete registration response.
	SigningDomainRegistrationV1
	// SigningDomainCheckInV1 signs a check-in request.
	SigningDomainCheckInV1
	// SigningDomainCheckInResponseV1 signs a complete check-in response. The
	// request and the response are separate namespaces so a signature over one
	// can never be presented as a signature over the other.
	SigningDomainCheckInResponseV1
	signingDomainLimit
)

func signingDomainTokens() [signingDomainLimit]string {
	return [...]string{
		SigningDomainUnknown:                   "",
		SigningDomainInstallationCertificateV1: SigningDomainInstallationCertificateV1Token,
		SigningDomainRegistrationV1:            SigningDomainRegistrationV1Token,
		SigningDomainCheckInV1:                 SigningDomainCheckInV1Token,
		SigningDomainCheckInResponseV1:         SigningDomainCheckInResponseV1Token,
	}
}

// Validate rejects the unset domain and every domain outside the closed set.
func (d SigningDomain) Validate() error {
	if d <= SigningDomainUnknown || d >= signingDomainLimit || signingDomainTokens()[d] == "" {
		return signingDomainError()
	}
	return nil
}

// IsValid reports whether d is one of the closed set's domains.
func (d SigningDomain) IsValid() bool { return d.Validate() == nil }

// String returns the canonical domain text, or empty text when unset.
func (d SigningDomain) String() string {
	if d >= signingDomainLimit {
		return ""
	}
	return signingDomainTokens()[d]
}

// MarshalText emits the canonical domain text and refuses an unset domain.
func (d SigningDomain) MarshalText() ([]byte, error) {
	if err := d.Validate(); err != nil {
		return nil, err
	}
	return []byte(d.String()), nil
}

// ParseCanonicalText reconstructs the domain whose canonical text is the
// supplied bytes.
//
// This is the self-referential half of the Attest contract: the same type both
// renders the text a signature covers and decides which text it will accept
// back. A separate parser could drift from the renderer and let a signature
// verify under a domain the signer never used.
func (SigningDomain) ParseCanonicalText(text []byte) (SigningDomain, error) {
	if len(text) > attest.SigningDomainMaximumBytes {
		return SigningDomainUnknown, signingDomainError()
	}
	tokens := signingDomainTokens()
	for domain := SigningDomainUnknown + 1; domain < signingDomainLimit; domain++ {
		if tokens[domain] != "" && tokens[domain] == string(text) {
			return domain, nil
		}
	}
	return SigningDomainUnknown, signingDomainError()
}

// ParseSigningDomain accepts one exact canonical domain text.
func ParseSigningDomain(value string) (SigningDomain, error) {
	return SigningDomainUnknown.ParseCanonicalText([]byte(value))
}

// MarshalJSON emits the canonical domain text and refuses an unset domain.
//
// It emits exactly what MarshalText emits. The domain names the namespace a
// signature covers, so a document that carried one spelling and verified under
// another would be verifying a signature it cannot name.
func (d SigningDomain) MarshalJSON() ([]byte, error) {
	if err := d.Validate(); err != nil {
		return nil, jsonError(err)
	}
	encoded, err := core.MarshalCanonicalJSONString(d.String())
	if err != nil {
		return nil, jsonError(signingDomainError(err))
	}
	return encoded, nil
}

// UnmarshalJSON accepts only a domain inside the closed set and leaves d
// unchanged on every rejection.
func (d *SigningDomain) UnmarshalJSON(data []byte) error {
	if d == nil {
		return jsonError(signingDomainError())
	}
	token, err := core.DecodeJSONStringToken(data)
	if err != nil {
		return jsonError(signingDomainError(err))
	}
	parsed, err := ParseSigningDomain(token)
	if err != nil {
		return jsonError(err)
	}
	*d = parsed
	return nil
}

// signingDomainWitness makes the Attest contract a compile-time obligation.
// The constraint embeds comparable, so it cannot be asserted as an interface
// value; instantiating this function is the assertion.
func signingDomainWitness[D attest.SigningDomain[D]]() {}

var (
	_ core.Validatable       = SigningDomainUnknown
	_ encoding.TextMarshaler = SigningDomainUnknown

	_ = signingDomainWitness[SigningDomain]
)
