package lease

import (
	"encoding/json"
	"errors"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/core"
)

const (
	attestBodyLengthCanonicalMaximumBytes  = 7
	leaseBodyLengthCanonicalMaximumBytes   = 3
	leaseEnvelopeCanonicalJSONMaximumBytes = attest.EnvelopeCanonicalJSONMaximumBytes -
		(attest.SigningDomainMaximumBytes - len(decisionDomainToken)) -
		(attestBodyLengthCanonicalMaximumBytes - leaseBodyLengthCanonicalMaximumBytes)
	// DocumentCanonicalJSONMaximumBytes is the exact compact signed-document
	// maximum.
	DocumentCanonicalJSONMaximumBytes = len(`{"decision":,"attestation":}`) +
		DecisionCanonicalJSONMaximumBytes +
		leaseEnvelopeCanonicalJSONMaximumBytes
	documentJSONWhitespaceAllowance = 8 << 10
	// DocumentJSONMaximumBytes bounds an accepted signed document.
	DocumentJSONMaximumBytes = DocumentCanonicalJSONMaximumBytes +
		documentJSONWhitespaceAllowance
)

var (
	_ [attest.CanonicalBodyMaximumBytes - 1_000_000]struct{}
	_ [9_999_999 - attest.CanonicalBodyMaximumBytes]struct{}
	_ [999 - DecisionCanonicalJSONMaximumBytes]struct{}
	_ [DecisionCanonicalJSONMaximumBytes - 100]struct{}
)

// Document is an untrusted decision and detached structural attestation.
// Validate proves shape only, never authority or signature validity.
type Document struct {
	Decision    Decision                `json:"decision"`
	Attestation attest.Envelope[Domain] `json:"attestation"`
}

// Validate proves structural document closure.
func (d Document) Validate() error {
	if err := d.Decision.Validate(); err != nil {
		return contractError(err)
	}
	if err := d.Attestation.Validate(); err != nil {
		return contractError(err)
	}
	if d.Attestation.Domain != d.Decision.AttestationDomain() {
		return verificationError(errors.New("lease document attestation domain differs from its body"))
	}
	return nil
}

// MarshalJSON emits the exact document field order.
func (d Document) MarshalJSON() ([]byte, error) {
	type wire Document
	if err := d.Validate(); err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(wire(d))
	if err != nil || len(encoded) > DocumentCanonicalJSONMaximumBytes {
		return nil, jsonError(errors.New("lease document encoding exceeded its contract"), err)
	}
	return encoded, nil
}

// UnmarshalJSON accepts one bounded strict document without receiver mutation
// on rejection.
func (d *Document) UnmarshalJSON(data []byte) error {
	if d == nil {
		return jsonError(errors.New("document receiver is nil"))
	}
	type wire Document
	limits, err := (jsonStructureContract{
		maximumBytes: DocumentJSONMaximumBytes,
		depth:        5,
		fields:       6,
	}).limits()
	if err != nil {
		return err
	}
	decoded, err := core.DecodeStrictJSONStructure[wire](data, limits)
	if err != nil {
		return jsonError(err)
	}
	candidate := Document(decoded)
	if err := candidate.Validate(); err != nil {
		return jsonError(err)
	}
	*d = candidate
	return nil
}
