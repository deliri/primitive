package runnercontrol

import (
	"bytes"
	"crypto"
	json "encoding/json/v2"
	"errors"
	"io"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/core"
)

type schedulingCapabilityWire SchedulingCapability
type memberCapabilityWire MemberCapability
type experimentCapabilityWire ExperimentCapability

func (c SchedulingCapability) AttestationDomain() CapabilitySigningDomain {
	return CapabilitySigningDomainSchedulingV1
}

func (c MemberCapability) AttestationDomain() CapabilitySigningDomain {
	return CapabilitySigningDomainMemberV1
}

func (c ExperimentCapability) AttestationDomain() CapabilitySigningDomain {
	return CapabilitySigningDomainExperimentV1
}

func (c SchedulingCapability) WriteCanonical(destination io.Writer) error {
	encoded, err := c.MarshalJSON()
	return writeCapabilityCanonical(destination, encoded, err)
}

func (c MemberCapability) WriteCanonical(destination io.Writer) error {
	encoded, err := c.MarshalJSON()
	return writeCapabilityCanonical(destination, encoded, err)
}

func (c ExperimentCapability) WriteCanonical(destination io.Writer) error {
	encoded, err := c.MarshalJSON()
	return writeCapabilityCanonical(destination, encoded, err)
}

func writeCapabilityCanonical(destination io.Writer, encoded []byte, encodeErr error) error {
	if destination == nil {
		return core.ErrPrimitiveContract
	}
	if encodeErr != nil {
		return encodeErr
	}
	written, err := destination.Write(encoded)
	if err != nil {
		return errors.Join(core.ErrPrimitiveContract, err)
	}
	if written != len(encoded) {
		return errors.Join(core.ErrPrimitiveContract, io.ErrShortWrite)
	}
	return nil
}

func (c SchedulingCapability) MarshalJSON() ([]byte, error) {
	return marshalCapabilityPayload(c, schedulingCapabilityWire(c))
}

func (c MemberCapability) MarshalJSON() ([]byte, error) {
	return marshalCapabilityPayload(c, memberCapabilityWire(c))
}

func (c ExperimentCapability) MarshalJSON() ([]byte, error) {
	return marshalCapabilityPayload(c, experimentCapabilityWire(c))
}

func marshalCapabilityPayload[T core.Validatable, W any](value T, wire W) ([]byte, error) {
	if err := value.Validate(); err != nil {
		return nil, errors.Join(core.ErrJSONContract, err)
	}
	encoded, err := core.MarshalCanonicalJSONDocument(wire)
	if err != nil || len(encoded) > core.JSONDocumentMaximumBytes {
		return nil, errors.Join(core.ErrJSONContract, core.ErrPrimitiveContract, err)
	}
	return encoded, nil
}

func (c *SchedulingCapability) UnmarshalJSON(data []byte) error {
	if c == nil {
		return errors.Join(core.ErrJSONContract, core.ErrPrimitiveContract)
	}
	wire, err := core.DecodeStrictJSONStructure[schedulingCapabilityWire](data, core.DefaultStrictJSONLimits())
	if err != nil {
		return errors.Join(core.ErrPrimitiveContract, err)
	}
	candidate := SchedulingCapability(wire)
	if err := candidate.Validate(); err != nil {
		return errors.Join(core.ErrJSONContract, err)
	}
	*c = candidate
	return nil
}

func (c *MemberCapability) UnmarshalJSON(data []byte) error {
	if c == nil {
		return errors.Join(core.ErrJSONContract, core.ErrPrimitiveContract)
	}
	wire, err := core.DecodeStrictJSONStructure[memberCapabilityWire](data, core.DefaultStrictJSONLimits())
	if err != nil {
		return errors.Join(core.ErrPrimitiveContract, err)
	}
	candidate := MemberCapability(wire)
	if err := candidate.Validate(); err != nil {
		return errors.Join(core.ErrJSONContract, err)
	}
	*c = candidate
	return nil
}

func (c *ExperimentCapability) UnmarshalJSON(data []byte) error {
	if c == nil {
		return errors.Join(core.ErrJSONContract, core.ErrPrimitiveContract)
	}
	wire, err := core.DecodeStrictJSONStructure[experimentCapabilityWire](data, core.DefaultStrictJSONLimits())
	if err != nil {
		return errors.Join(core.ErrPrimitiveContract, err)
	}
	candidate := ExperimentCapability(wire)
	if err := candidate.Validate(); err != nil {
		return errors.Join(core.ErrJSONContract, err)
	}
	*c = candidate
	return nil
}

type SchedulingCapabilityDocument struct {
	Payload     SchedulingCapability                     `json:"payload"`
	Attestation attest.Envelope[CapabilitySigningDomain] `json:"attestation"`
}

type MemberCapabilityDocument struct {
	Payload     MemberCapability                         `json:"payload"`
	Attestation attest.Envelope[CapabilitySigningDomain] `json:"attestation"`
}

type ExperimentCapabilityDocument struct {
	Payload     ExperimentCapability                     `json:"payload"`
	Attestation attest.Envelope[CapabilitySigningDomain] `json:"attestation"`
}

func (d SchedulingCapabilityDocument) Validate() error {
	return validateCapabilityDocument(d.Payload, d.Attestation)
}

func (d MemberCapabilityDocument) Validate() error {
	return validateCapabilityDocument(d.Payload, d.Attestation)
}

func (d ExperimentCapabilityDocument) Validate() error {
	return validateCapabilityDocument(d.Payload, d.Attestation)
}

func validateCapabilityDocument[B attest.CanonicalBody[CapabilitySigningDomain]](body B, envelope attest.Envelope[CapabilitySigningDomain]) error {
	if err := errors.Join(body.Validate(), envelope.Validate()); err != nil {
		return err
	}
	if envelope.Domain != body.AttestationDomain() {
		return core.ErrPrimitiveContract
	}
	return nil
}

type schedulingCapabilityDocumentWire SchedulingCapabilityDocument
type memberCapabilityDocumentWire MemberCapabilityDocument
type experimentCapabilityDocumentWire ExperimentCapabilityDocument

func (d SchedulingCapabilityDocument) MarshalJSON() ([]byte, error) {
	return marshalCapabilityDocument(d, schedulingCapabilityDocumentWire(d))
}

func (d MemberCapabilityDocument) MarshalJSON() ([]byte, error) {
	return marshalCapabilityDocument(d, memberCapabilityDocumentWire(d))
}

func (d ExperimentCapabilityDocument) MarshalJSON() ([]byte, error) {
	return marshalCapabilityDocument(d, experimentCapabilityDocumentWire(d))
}

func marshalCapabilityDocument[T core.Validatable, W any](value T, wire W) ([]byte, error) {
	if err := value.Validate(); err != nil {
		return nil, errors.Join(core.ErrJSONContract, err)
	}
	encoded, err := core.MarshalCanonicalJSONDocument(wire)
	if err != nil || len(encoded) > core.JSONDocumentMaximumBytes {
		return nil, errors.Join(core.ErrJSONContract, core.ErrPrimitiveContract, err)
	}
	return encoded, nil
}

func (d *SchedulingCapabilityDocument) UnmarshalJSON(data []byte) error {
	if d == nil {
		return errors.Join(core.ErrJSONContract, core.ErrPrimitiveContract)
	}
	wire, err := core.DecodeStrictJSONStructure[schedulingCapabilityDocumentWire](data, core.DefaultStrictJSONLimits())
	if err != nil {
		return errors.Join(core.ErrPrimitiveContract, err)
	}
	candidate := SchedulingCapabilityDocument(wire)
	if err := candidate.Validate(); err != nil {
		return errors.Join(core.ErrJSONContract, err)
	}
	*d = candidate
	return nil
}

func (d *MemberCapabilityDocument) UnmarshalJSON(data []byte) error {
	if d == nil {
		return errors.Join(core.ErrJSONContract, core.ErrPrimitiveContract)
	}
	wire, err := core.DecodeStrictJSONStructure[memberCapabilityDocumentWire](data, core.DefaultStrictJSONLimits())
	if err != nil {
		return errors.Join(core.ErrPrimitiveContract, err)
	}
	candidate := MemberCapabilityDocument(wire)
	if err := candidate.Validate(); err != nil {
		return errors.Join(core.ErrJSONContract, err)
	}
	*d = candidate
	return nil
}

func (d *ExperimentCapabilityDocument) UnmarshalJSON(data []byte) error {
	if d == nil {
		return errors.Join(core.ErrJSONContract, core.ErrPrimitiveContract)
	}
	wire, err := core.DecodeStrictJSONStructure[experimentCapabilityDocumentWire](data, core.DefaultStrictJSONLimits())
	if err != nil {
		return errors.Join(core.ErrPrimitiveContract, err)
	}
	candidate := ExperimentCapabilityDocument(wire)
	if err := candidate.Validate(); err != nil {
		return errors.Join(core.ErrJSONContract, err)
	}
	*d = candidate
	return nil
}

func (d SchedulingCapabilityDocument) Digest() (core.SHA256Digest, error) {
	return capabilityDocumentDigest(d)
}

func (d MemberCapabilityDocument) Digest() (core.SHA256Digest, error) {
	return capabilityDocumentDigest(d)
}

func (d ExperimentCapabilityDocument) Digest() (core.SHA256Digest, error) {
	return capabilityDocumentDigest(d)
}

func capabilityDocumentDigest[D interface{ MarshalJSON() ([]byte, error) }](document D) (core.SHA256Digest, error) {
	encoded, err := document.MarshalJSON()
	if err != nil {
		return core.SHA256Digest{}, err
	}
	return core.SHA256Of(encoded), nil
}

func IssueSchedulingCapability(payload SchedulingCapability, signer crypto.Signer) (SchedulingCapabilityDocument, error) {
	envelope, err := attest.Sign(attest.SignRequest[CapabilitySigningDomain]{Body: payload, Signer: signer})
	if err != nil {
		return SchedulingCapabilityDocument{}, err
	}
	document := SchedulingCapabilityDocument{Payload: payload, Attestation: envelope}
	return document, document.Validate()
}

func IssueMemberCapability(payload MemberCapability, signer crypto.Signer) (MemberCapabilityDocument, error) {
	envelope, err := attest.Sign(attest.SignRequest[CapabilitySigningDomain]{Body: payload, Signer: signer})
	if err != nil {
		return MemberCapabilityDocument{}, err
	}
	document := MemberCapabilityDocument{Payload: payload, Attestation: envelope}
	return document, document.Validate()
}

func IssueExperimentCapability(payload ExperimentCapability, signer crypto.Signer) (ExperimentCapabilityDocument, error) {
	envelope, err := attest.Sign(attest.SignRequest[CapabilitySigningDomain]{Body: payload, Signer: signer})
	if err != nil {
		return ExperimentCapabilityDocument{}, err
	}
	document := ExperimentCapabilityDocument{Payload: payload, Attestation: envelope}
	return document, document.Validate()
}

func VerifySchedulingCapability(document SchedulingCapabilityDocument, trusted attest.TrustedKeys) error {
	return verifyCapabilityDocument(document.Payload, document.Attestation, trusted)
}

func VerifyMemberCapability(document MemberCapabilityDocument, trusted attest.TrustedKeys) error {
	return verifyCapabilityDocument(document.Payload, document.Attestation, trusted)
}

func VerifyExperimentCapability(document ExperimentCapabilityDocument, trusted attest.TrustedKeys) error {
	return verifyCapabilityDocument(document.Payload, document.Attestation, trusted)
}

func verifyCapabilityDocument[B attest.CanonicalBody[CapabilitySigningDomain]](body B, envelope attest.Envelope[CapabilitySigningDomain], trusted attest.TrustedKeys) error {
	if err := validateCapabilityDocument(body, envelope); err != nil {
		return err
	}
	proof, err := attest.Verify(attest.VerifyRequest[CapabilitySigningDomain]{Body: body, Envelope: envelope, TrustedKeys: trusted})
	if err != nil {
		return err
	}
	return proof.Validate()
}

type schedulingClaimWire SchedulingClaim

func (c SchedulingClaim) MarshalJSON() ([]byte, error) {
	if err := c.Validate(); err != nil {
		return nil, errors.Join(core.ErrJSONContract, err)
	}
	encoded, err := core.MarshalCanonicalJSONDocument(schedulingClaimWire(c))
	if err != nil || len(encoded) > core.JSONDocumentMaximumBytes {
		return nil, errors.Join(core.ErrJSONContract, core.ErrPrimitiveContract, err)
	}
	return encoded, nil
}

func (c *SchedulingClaim) UnmarshalJSON(data []byte) error {
	if c == nil {
		return errors.Join(core.ErrJSONContract, core.ErrPrimitiveContract)
	}
	wire, err := core.DecodeStrictJSONStructure[schedulingClaimWire](data, core.DefaultStrictJSONLimits())
	if err != nil {
		return errors.Join(core.ErrPrimitiveContract, err)
	}
	candidate := SchedulingClaim(wire)
	if err := candidate.Validate(); err != nil {
		return errors.Join(core.ErrJSONContract, err)
	}
	*c = candidate
	return nil
}

func VerifySchedulingClaim(claim SchedulingClaim, trusted attest.TrustedKeys) error {
	if err := claim.Validate(); err != nil {
		return err
	}
	if err := VerifySchedulingCapability(claim.Capability, trusted); err != nil {
		return err
	}
	for index := range claim.Members {
		if err := VerifyMemberCapability(claim.Members[index], trusted); err != nil {
			return err
		}
	}
	for index := range claim.Direct {
		if err := VerifyExperimentCapability(claim.Direct[index], trusted); err != nil {
			return err
		}
	}
	return nil
}

type SchedulingClaimRecord struct {
	Canonical []byte
	Claim     SchedulingClaim
	Bytes     core.ByteLength
	Digest    core.SHA256Digest
}

func NewSchedulingClaimRecord(claim SchedulingClaim) (SchedulingClaimRecord, error) {
	canonical, err := claim.MarshalJSON()
	if err != nil {
		return SchedulingClaimRecord{}, err
	}
	extent, err := core.NewByteLength(uint64(len(canonical)))
	if err != nil {
		return SchedulingClaimRecord{}, err
	}
	record := SchedulingClaimRecord{Claim: claim, Canonical: canonical, Digest: core.SHA256Of(canonical), Bytes: extent}
	return record, record.Validate()
}

func (r SchedulingClaimRecord) Validate() error {
	if err := errors.Join(r.Claim.Validate(), r.Digest.Validate(), r.Bytes.Validate()); err != nil {
		return err
	}
	canonical, err := r.Claim.MarshalJSON()
	if err != nil || !bytes.Equal(canonical, r.Canonical) || r.Digest != core.SHA256Of(r.Canonical) || r.Bytes.Uint64() != uint64(len(r.Canonical)) {
		return errors.Join(core.ErrPrimitiveContract, err)
	}
	return nil
}

var (
	_ core.ValidatedJSONMarshaler = SchedulingCapability{}
	_ json.Unmarshaler            = (*SchedulingCapability)(nil)
	_ core.ValidatedJSONMarshaler = MemberCapability{}
	_ json.Unmarshaler            = (*MemberCapability)(nil)
	_ core.ValidatedJSONMarshaler = ExperimentCapability{}
	_ json.Unmarshaler            = (*ExperimentCapability)(nil)
	_ core.ValidatedJSONMarshaler = SchedulingCapabilityDocument{}
	_ json.Unmarshaler            = (*SchedulingCapabilityDocument)(nil)
	_ core.ValidatedJSONMarshaler = MemberCapabilityDocument{}
	_ json.Unmarshaler            = (*MemberCapabilityDocument)(nil)
	_ core.ValidatedJSONMarshaler = ExperimentCapabilityDocument{}
	_ json.Unmarshaler            = (*ExperimentCapabilityDocument)(nil)
	_ core.ValidatedJSONMarshaler = SchedulingClaim{}
	_ json.Unmarshaler            = (*SchedulingClaim)(nil)
	_ core.Validatable            = SchedulingClaimRecord{}
)
