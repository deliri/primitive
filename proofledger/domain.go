package proofledger

import (
	"encoding"
	json "encoding/json/v2"
	"errors"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/core"
)

const AppendReceiptSigningDomainV1Token = "primitive-proof-ledger-append-receipt-2026-1"

type AppendReceiptSigningDomain uint8

const (
	AppendReceiptSigningDomainUnknown AppendReceiptSigningDomain = iota
	AppendReceiptSigningDomainV1
	appendReceiptSigningDomainLimit
)

func (d AppendReceiptSigningDomain) Validate() error {
	if d != AppendReceiptSigningDomainV1 {
		return contractError(errors.New("proof ledger append receipt signing domain is invalid"))
	}
	return nil
}

func (d AppendReceiptSigningDomain) IsValid() bool { return d.Validate() == nil }

func (d AppendReceiptSigningDomain) String() string {
	if d == AppendReceiptSigningDomainV1 {
		return AppendReceiptSigningDomainV1Token
	}
	return ""
}

func (d AppendReceiptSigningDomain) MarshalText() ([]byte, error) {
	if err := d.Validate(); err != nil {
		return nil, err
	}
	return []byte(d.String()), nil
}

func (AppendReceiptSigningDomain) ParseCanonicalText(text []byte) (AppendReceiptSigningDomain, error) {
	if len(text) > attest.SigningDomainMaximumBytes || string(text) != AppendReceiptSigningDomainV1Token {
		return AppendReceiptSigningDomainUnknown, contractError(errors.New("proof ledger append receipt signing domain text is unsupported"))
	}
	return AppendReceiptSigningDomainV1, nil
}

func (d AppendReceiptSigningDomain) MarshalJSON() ([]byte, error) {
	if err := d.Validate(); err != nil {
		return nil, jsonError(err)
	}
	return core.MarshalCanonicalJSONString(d.String())
}

func (d *AppendReceiptSigningDomain) UnmarshalJSON(data []byte) error {
	if d == nil {
		return jsonError(errors.New("nil proof ledger append receipt signing domain receiver"))
	}
	value, err := core.DecodeJSONStringToken(data)
	if err != nil {
		return jsonError(err)
	}
	parsed, err := AppendReceiptSigningDomainUnknown.ParseCanonicalText([]byte(value))
	if err != nil {
		return jsonError(err)
	}
	*d = parsed
	return nil
}

type appendReceiptSigningDomainWitness[D attest.SigningDomain[D]] [0]D

var (
	_ core.ValidatedJSONMarshaler = AppendReceiptSigningDomain(0)
	_ encoding.TextMarshaler      = AppendReceiptSigningDomain(0)
	_ json.Unmarshaler            = (*AppendReceiptSigningDomain)(nil)
	_                             = appendReceiptSigningDomainWitness[AppendReceiptSigningDomain]{}
)
