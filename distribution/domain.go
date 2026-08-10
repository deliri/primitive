package distribution

import (
	"encoding/json"
	"errors"

	"github.com/deliri/primitive/v2026/core"
)

const signingDomainNonCanonicalDiagnostic = "distribution signing domain is not canonical"

const (
	SigningDomainPublicationRequestV1Token    = "primitive-distribution-publication-request-2026-1"
	SigningDomainPublicationGrantV1Token      = "primitive-distribution-publication-grant-2026-1"
	SigningDomainPublicationCompletionV1Token = "primitive-distribution-publication-completion-2026-1"
	SigningDomainUpdateRequestV1Token         = "primitive-distribution-update-request-2026-1"
	SigningDomainUpdateResponseV1Token        = "primitive-distribution-update-response-2026-1"
	SigningDomainUpgradeRequestV1Token        = "primitive-distribution-upgrade-request-2026-1"
	SigningDomainUpgradeGrantV1Token          = "primitive-distribution-upgrade-grant-2026-1"
)

// SigningDomain is the closed set of distribution signature namespaces.
type SigningDomain uint8

const (
	SigningDomainUnknown SigningDomain = iota
	SigningDomainPublicationRequestV1
	SigningDomainPublicationGrantV1
	SigningDomainPublicationCompletionV1
	SigningDomainUpdateRequestV1
	SigningDomainUpdateResponseV1
	SigningDomainUpgradeRequestV1
	SigningDomainUpgradeGrantV1
	signingDomainLimit
)

func signingDomainTokens() [signingDomainLimit]string {
	return [...]string{
		SigningDomainUnknown:                 "",
		SigningDomainPublicationRequestV1:    SigningDomainPublicationRequestV1Token,
		SigningDomainPublicationGrantV1:      SigningDomainPublicationGrantV1Token,
		SigningDomainPublicationCompletionV1: SigningDomainPublicationCompletionV1Token,
		SigningDomainUpdateRequestV1:         SigningDomainUpdateRequestV1Token,
		SigningDomainUpdateResponseV1:        SigningDomainUpdateResponseV1Token,
		SigningDomainUpgradeRequestV1:        SigningDomainUpgradeRequestV1Token,
		SigningDomainUpgradeGrantV1:          SigningDomainUpgradeGrantV1Token,
	}
}

func (d SigningDomain) Validate() error {
	if d <= SigningDomainUnknown || d >= signingDomainLimit || signingDomainTokens()[d] == "" {
		return contractError(errors.New("distribution signing domain is outside the closed domain"))
	}
	return nil
}

func (d SigningDomain) IsValid() bool { return d.Validate() == nil }

func (d SigningDomain) String() string {
	if !d.IsValid() {
		return ""
	}
	return signingDomainTokens()[d]
}

func ParseSigningDomain(value string) (SigningDomain, error) {
	for candidate := SigningDomainUnknown + 1; candidate < signingDomainLimit; candidate++ {
		if candidate.String() == value {
			return candidate, nil
		}
	}
	return SigningDomainUnknown, contractError(errors.New("distribution signing domain is unsupported"))
}

func (d SigningDomain) MarshalText() ([]byte, error) {
	if err := d.Validate(); err != nil {
		return nil, err
	}
	return []byte(d.String()), nil
}

func (SigningDomain) ParseCanonicalText(text []byte) (SigningDomain, error) {
	parsed, err := ParseSigningDomain(string(text))
	if err != nil {
		return SigningDomainUnknown, err
	}
	canonical, _ := parsed.MarshalText()
	if string(canonical) != string(text) {
		return SigningDomainUnknown, contractError(errors.New(signingDomainNonCanonicalDiagnostic))
	}
	return parsed, nil
}

func (d SigningDomain) MarshalJSON() ([]byte, error) {
	if err := d.Validate(); err != nil {
		return nil, jsonError(err)
	}
	return core.MarshalCanonicalJSONString(d.String())
}

func (d *SigningDomain) UnmarshalJSON(data []byte) error {
	if d == nil {
		return jsonError(errors.New("distribution signing domain receiver is nil"))
	}
	value, err := core.DecodeJSONStringToken(data)
	if err != nil {
		return jsonError(err)
	}
	parsed, err := ParseSigningDomain(value)
	if err != nil {
		return jsonError(err)
	}
	canonical, marshalErr := json.Marshal(value)
	if marshalErr != nil || string(canonical) != string(data) {
		return jsonError(errors.New(signingDomainNonCanonicalDiagnostic), marshalErr)
	}
	*d = parsed
	return nil
}

var (
	_ core.Validatable            = SigningDomainUnknown
	_ core.ValidatedJSONMarshaler = SigningDomain(0)
	_ json.Unmarshaler            = (*SigningDomain)(nil)
)
