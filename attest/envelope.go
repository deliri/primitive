package attest

import (
	"errors"

	"github.com/deliri/primitive/v2026/core"
)

// Envelope is one detached structural attestation. Validate does not claim
// signer trust or signature validity.
type Envelope[D SigningDomain[D]] struct {
	Domain     D
	BodyLength core.ByteCount
	Signature  Signature
	Signer     core.Ed25519PublicKey
	BodySHA256 core.SHA256Digest
}

// envelopeWire fixes the canonical member order. It mirrors the signed frame,
// domain then signer then body length then body digest, and appends the
// detached signature last, so the wire projection reads as the layout that was
// actually signed. Reordering members changes the canonical bytes.
type envelopeWire struct {
	Domain     *domainToken           `json:"domain"`
	Signer     *core.Ed25519PublicKey `json:"signer"`
	BodyLength *core.ByteCount        `json:"body_length_bytes"`
	BodySHA256 *core.SHA256Digest     `json:"body_sha256"`
	Signature  *Signature             `json:"signature"`
}

// Validate proves structural envelope closure.
func (e Envelope[D]) Validate() error {
	if _, err := canonicalDomain(e.Domain); err != nil {
		return err
	}
	if err := e.Signer.Validate(); err != nil {
		return contractError(err)
	}
	if err := validateBodyLength(e.BodyLength); err != nil {
		return err
	}
	if err := e.BodySHA256.Validate(); err != nil {
		return contractError(err)
	}
	if err := e.Signature.Validate(); err != nil {
		return err
	}
	return nil
}

func (e Envelope[D]) wire() (envelopeWire, error) {
	if err := e.Validate(); err != nil {
		return envelopeWire{}, err
	}
	token, err := canonicalDomain(e.Domain)
	if err != nil {
		return envelopeWire{}, err
	}
	signer := e.Signer
	bodyLength := e.BodyLength
	bodySHA256 := e.BodySHA256
	signature := e.Signature
	return envelopeWire{
		Domain:     &token,
		Signer:     &signer,
		BodyLength: &bodyLength,
		BodySHA256: &bodySHA256,
		Signature:  &signature,
	}, nil
}

func (w envelopeWire) Validate() error {
	if w.Domain == nil || w.Signer == nil || w.BodyLength == nil ||
		w.BodySHA256 == nil || w.Signature == nil {
		return contractError(errors.New(envelopeWireMissingErrorText))
	}
	if err := w.Domain.Validate(); err != nil {
		return err
	}
	if err := w.Signer.Validate(); err != nil {
		return contractError(err)
	}
	if err := validateBodyLength(*w.BodyLength); err != nil {
		return err
	}
	if err := w.BodySHA256.Validate(); err != nil {
		return contractError(err)
	}
	return w.Signature.Validate()
}

// MarshalJSON emits the canonical envelope projection.
func (e Envelope[D]) MarshalJSON() ([]byte, error) {
	wire, err := e.wire()
	if err != nil {
		return nil, envelopeJSONError(err)
	}
	encoded, err := core.MarshalCanonicalJSONDocument(wire)
	if err != nil {
		return nil, envelopeJSONError(err)
	}
	if len(encoded) > EnvelopeCanonicalJSONMaximumBytes {
		return nil, envelopeJSONError(errors.New(envelopeBodyLengthErrorText))
	}
	return encoded, nil
}

// UnmarshalJSON accepts bounded strict JSON and preserves the receiver on
// every rejection. Harmless JSON whitespace, member order, and equivalent
// string escapes are accepted and normalize on the next marshal. The raw
// document, whitespace included, is bounded by EnvelopeJSONMaximumBytes.
func (e *Envelope[D]) UnmarshalJSON(data []byte) error {
	if e == nil {
		return envelopeJSONError(errors.New("nil envelope receiver"))
	}
	limits, err := envelopeJSONLimits()
	if err != nil {
		return envelopeJSONError(err)
	}
	wire, err := core.DecodeStrictJSON[envelopeWire](data, limits)
	if err != nil {
		return envelopeJSONError(err)
	}
	domain, err := parseCanonicalDomain[D](*wire.Domain)
	if err != nil {
		return envelopeJSONError(err)
	}
	candidate := Envelope[D]{
		Domain:     domain,
		Signer:     *wire.Signer,
		BodyLength: *wire.BodyLength,
		BodySHA256: *wire.BodySHA256,
		Signature:  *wire.Signature,
	}
	if err := candidate.Validate(); err != nil {
		return envelopeJSONError(err)
	}
	*e = candidate
	return nil
}

func envelopeJSONLimits() (core.StrictJSONLimits, error) {
	maximum, err := core.NewByteCount(EnvelopeJSONMaximumBytes)
	if err != nil {
		return core.StrictJSONLimits{}, err
	}
	return core.StrictJSONLimits{
		DocumentMaximumBytes: maximum,
		NestingDepthMaximum:  1,
		ObjectFieldMaximum:   5,
		ArrayItemMaximum:     1,
	}, nil
}
