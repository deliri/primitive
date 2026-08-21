package payment

import (
	"encoding"
	json "encoding/json/v2"
	"errors"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/core"
)

const (
	// SigningDomainReceiptV1Token separates an immutable payment receipt.
	SigningDomainReceiptV1Token = "primitive-payment-receipt-2026-1"
	// SigningDomainCatalogV1Token separates an observed payment catalog.
	SigningDomainCatalogV1Token = "primitive-payment-catalog-2026-1"
	// SigningDomainQueryV1Token separates an installed device's catalog request.
	SigningDomainQueryV1Token = "primitive-payment-query-2026-1"
)

// SigningDomain closes the two payment authority statement namespaces.
type SigningDomain uint8

const (
	// SigningDomainUnknown is the invalid zero signing domain.
	SigningDomainUnknown SigningDomain = iota
	// SigningDomainReceiptV1 authenticates an immutable payment receipt.
	SigningDomainReceiptV1
	// SigningDomainCatalogV1 authenticates one bounded payment catalog page.
	SigningDomainCatalogV1
	// SigningDomainQueryV1 authenticates one installed device's catalog request.
	SigningDomainQueryV1
	signingDomainLimit
)

func signingDomainTokens() [signingDomainLimit]string {
	return [...]string{"", SigningDomainReceiptV1Token, SigningDomainCatalogV1Token, SigningDomainQueryV1Token}
}

// Validate rejects signing domains outside the closed domain.
func (d SigningDomain) Validate() error {
	if d <= SigningDomainUnknown || d >= signingDomainLimit || signingDomainTokens()[d] == "" {
		return contractError(errors.New("payment signing domain is invalid"))
	}
	return nil
}

// IsValid reports whether d names one published payment signing namespace.
func (d SigningDomain) IsValid() bool { return d.Validate() == nil }

// String returns the canonical token or empty text for an invalid domain.
func (d SigningDomain) String() string {
	if d >= signingDomainLimit {
		return ""
	}
	return signingDomainTokens()[d]
}

// MarshalText emits the canonical signing token.
func (d SigningDomain) MarshalText() ([]byte, error) {
	if err := d.Validate(); err != nil {
		return nil, err
	}
	return []byte(d.String()), nil
}

// ParseCanonicalText accepts one exact payment signing token.
func (SigningDomain) ParseCanonicalText(text []byte) (SigningDomain, error) {
	if len(text) > attest.SigningDomainMaximumBytes {
		return SigningDomainUnknown, contractError(errors.New("payment signing domain is too long"))
	}
	for domain := SigningDomainUnknown + 1; domain < signingDomainLimit; domain++ {
		if domain.String() == string(text) {
			return domain, nil
		}
	}
	return SigningDomainUnknown, contractError(errors.New("payment signing domain is unsupported"))
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
		return jsonError(errors.New("nil payment signing domain receiver"))
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
