package retrieval

import (
	"encoding"
	"encoding/json"
	"errors"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/core"
)

// #nosec G101 -- these are public signature-domain namespace labels emitted in
// every request and grant, not credentials or bearer material.
const (
	SigningDomainRequestV1Token = "primitive-retrieval-request-2026-1"
	SigningDomainGrantV1Token   = "primitive-retrieval-grant-2026-1"
)

type SigningDomain uint8

const (
	SigningDomainUnknown SigningDomain = iota
	SigningDomainRequestV1
	SigningDomainGrantV1
	signingDomainLimit
)

func signingDomainTokens() [signingDomainLimit]string {
	return [...]string{"", SigningDomainRequestV1Token, SigningDomainGrantV1Token}
}

func (d SigningDomain) Validate() error {
	if d <= SigningDomainUnknown || d >= signingDomainLimit || signingDomainTokens()[d] == "" {
		return contractError(errors.New("retrieval signing domain is invalid"))
	}
	return nil
}

// IsValid reports whether d names one published retrieval signing namespace.
func (d SigningDomain) IsValid() bool { return d.Validate() == nil }

func (d SigningDomain) String() string {
	if d >= signingDomainLimit {
		return ""
	}
	return signingDomainTokens()[d]
}

func (d SigningDomain) MarshalText() ([]byte, error) {
	if err := d.Validate(); err != nil {
		return nil, err
	}
	return []byte(d.String()), nil
}

func (SigningDomain) ParseCanonicalText(text []byte) (SigningDomain, error) {
	if len(text) > attest.SigningDomainMaximumBytes {
		return SigningDomainUnknown, contractError(errors.New("retrieval signing domain is too long"))
	}
	for domain := SigningDomainUnknown + 1; domain < signingDomainLimit; domain++ {
		if domain.String() == string(text) {
			return domain, nil
		}
	}
	return SigningDomainUnknown, contractError(errors.New("retrieval signing domain is unsupported"))
}

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

func (d *SigningDomain) UnmarshalJSON(data []byte) error {
	if d == nil {
		return jsonError(errors.New("nil retrieval signing domain receiver"))
	}
	value, err := core.DecodeJSONStringToken(data)
	if err != nil {
		return jsonError(err)
	}
	parsed, err := SigningDomainUnknown.ParseCanonicalText([]byte(value))
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
