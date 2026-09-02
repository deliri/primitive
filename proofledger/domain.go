package proofledger

import (
	"encoding"
	json "encoding/json/v2"
	"errors"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/core"
)

const ReceiptSigningDomainV1Token = "primitive-proof-ledger-receipt-2026-1"

type ReceiptSigningDomain uint8

const (
	ReceiptSigningDomainUnknown ReceiptSigningDomain = iota
	ReceiptSigningDomainV1
	receiptSigningDomainLimit
)

func (d ReceiptSigningDomain) Validate() error {
	if d != ReceiptSigningDomainV1 {
		return contractError(errors.New("proof ledger receipt signing domain is invalid"))
	}
	return nil
}

func (d ReceiptSigningDomain) IsValid() bool { return d.Validate() == nil }

func (d ReceiptSigningDomain) String() string {
	if d == ReceiptSigningDomainV1 {
		return ReceiptSigningDomainV1Token
	}
	return ""
}

func (d ReceiptSigningDomain) MarshalText() ([]byte, error) {
	if err := d.Validate(); err != nil {
		return nil, err
	}
	return []byte(d.String()), nil
}

func (ReceiptSigningDomain) ParseCanonicalText(text []byte) (ReceiptSigningDomain, error) {
	if len(text) > attest.SigningDomainMaximumBytes || string(text) != ReceiptSigningDomainV1Token {
		return ReceiptSigningDomainUnknown, contractError(errors.New("proof ledger receipt signing domain text is unsupported"))
	}
	return ReceiptSigningDomainV1, nil
}

func (d ReceiptSigningDomain) MarshalJSON() ([]byte, error) {
	if err := d.Validate(); err != nil {
		return nil, jsonError(err)
	}
	return core.MarshalCanonicalJSONString(d.String())
}

func (d *ReceiptSigningDomain) UnmarshalJSON(data []byte) error {
	if d == nil {
		return jsonError(errors.New("nil proof ledger receipt signing domain receiver"))
	}
	value, err := core.DecodeJSONStringToken(data)
	if err != nil {
		return jsonError(err)
	}
	parsed, err := ReceiptSigningDomainUnknown.ParseCanonicalText([]byte(value))
	if err != nil {
		return jsonError(err)
	}
	*d = parsed
	return nil
}

type receiptSigningDomainWitness[D attest.SigningDomain[D]] [0]D

var (
	_ core.ValidatedJSONMarshaler = ReceiptSigningDomain(0)
	_ encoding.TextMarshaler      = ReceiptSigningDomain(0)
	_ json.Unmarshaler            = (*ReceiptSigningDomain)(nil)
	_                             = receiptSigningDomainWitness[ReceiptSigningDomain]{}
)
