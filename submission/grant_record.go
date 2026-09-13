package submission

import (
	"errors"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/core"
)

// GrantRecordJSONMaximumBytes bounds one retained agreement, not total evidence.
const GrantRecordJSONMaximumBytes = 4096

// GrantRecord retains the authority's signed agreement, never its spendable
// upload capability. It is the durable input to completion verification after
// restart. Validate proves nominal shape; VerifyCompletion authenticates it.
type GrantRecord struct {
	Payload     GrantPayload                   `json:"payload"`
	Attestation attest.Envelope[SigningDomain] `json:"attestation"`
}

type grantRecordWire GrantRecord

func (r GrantRecord) Validate() error {
	if err := errors.Join(r.Payload.Validate(), r.Attestation.Validate()); err != nil {
		return contractError(err)
	}
	if r.Attestation.Domain != r.Payload.AttestationDomain() {
		return bindingError()
	}
	return nil
}

// Record removes the bearer at the issuer's persistence boundary, after binding
// the exact issued capability to the signed agreement.
func (p GrantProjection) Record() (GrantRecord, error) {
	if err := p.Validate(); err != nil {
		return GrantRecord{}, err
	}
	return GrantRecord{Payload: p.Payload, Attestation: p.Attestation}, nil
}

// Record closes the received capability binding before discarding its bearer.
func (d GrantDocument) Record() (GrantRecord, error) {
	if err := d.Validate(); err != nil {
		return GrantRecord{}, err
	}
	return GrantRecord{Payload: d.Payload, Attestation: d.Attestation}, nil
}

func (r GrantRecord) MarshalJSON() ([]byte, error) {
	if err := r.Validate(); err != nil {
		return nil, jsonError(err)
	}
	return core.MarshalCanonicalJSONDocument(grantRecordWire(r))
}

func (r *GrantRecord) UnmarshalJSON(data []byte) error {
	if r == nil || len(data) > GrantRecordJSONMaximumBytes {
		return jsonError(core.ErrControlPlaneContract)
	}
	wire, err := decodeStrict[grantRecordWire](data)
	if err != nil {
		return err
	}
	candidate := GrantRecord(wire)
	if err := candidate.Validate(); err != nil {
		return jsonError(err)
	}
	*r = candidate
	return nil
}
