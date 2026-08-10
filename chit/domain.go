package chit

import (
	"encoding"
	"encoding/json"
	"errors"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/core"
)

const (
	SigningDomainChitV1Token    = "primitive-chit-2026-1"
	SigningDomainCatalogV1Token = "primitive-chit-catalog-2026-1"
	SigningDomainQueryV1Token   = "primitive-chit-query-2026-1"
)

// SigningDomain separates one immutable chit from a catalog observation.
type SigningDomain uint8

const (
	SigningDomainUnknown SigningDomain = iota
	SigningDomainChitV1
	SigningDomainCatalogV1
	SigningDomainQueryV1
	signingDomainLimit
)

func signingDomainTokens() [signingDomainLimit]string {
	return [...]string{"", SigningDomainChitV1Token, SigningDomainCatalogV1Token, SigningDomainQueryV1Token}
}

func (d SigningDomain) Validate() error {
	if d <= SigningDomainUnknown || d >= signingDomainLimit || signingDomainTokens()[d] == "" {
		return contractError(errors.New("chit signing domain is invalid"))
	}
	return nil
}

// IsValid reports whether d names one published chit signing namespace.
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
		return SigningDomainUnknown, contractError(errors.New("chit signing domain is too long"))
	}
	for domain := SigningDomainUnknown + 1; domain < signingDomainLimit; domain++ {
		if signingDomainTokens()[domain] == string(text) {
			return domain, nil
		}
	}
	return SigningDomainUnknown, contractError(errors.New("chit signing domain is unsupported"))
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
		return jsonError(errors.New("nil chit signing domain receiver"))
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
