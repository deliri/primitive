package chit

import (
	"crypto"
	"errors"
	"io"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/receipt"
)

// ManifestMembershipPayload is an authority receipt of exact membership in a
// previously verified complete stream. It is one bounded record, not a cached
// collection. The owning immutable Chit closes scope, identity and retention.
type ManifestMembershipPayload struct {
	Custody Payload       `json:"custody"`
	Entry   ManifestEntry `json:"entry"`
}

func (p ManifestMembershipPayload) Validate() error {
	if err := errors.Join(p.Custody.Validate(), p.Entry.Validate()); err != nil {
		return contractError(err)
	}
	header := p.Entry.Evidence.Payload.Header
	if p.Entry.Sequence.Uint64() > p.Custody.Manifest.Objects.Uint64() || header.Principal != p.Custody.Scope.Principal || header.Offering != p.Custody.Scope.Offering {
		return conflictError(errors.New("manifest membership exceeds its custody boundary"))
	}
	return nil
}

func (ManifestMembershipPayload) AttestationDomain() SigningDomain {
	return SigningDomainManifestMembershipV1
}

func (p ManifestMembershipPayload) MarshalJSON() ([]byte, error) {
	if err := p.Validate(); err != nil {
		return nil, jsonError(err)
	}
	type wire ManifestMembershipPayload
	data, err := core.MarshalCanonicalJSONDocument(wire(p))
	if err != nil || len(data) > ChitPayloadJSONMaximumBytes {
		return nil, jsonError(err)
	}
	return data, nil
}

func (p *ManifestMembershipPayload) UnmarshalJSON(data []byte) error {
	if p == nil {
		return jsonError(errors.New("nil manifest membership payload receiver"))
	}
	type wire ManifestMembershipPayload
	decoded, err := decodeStrict[wire](data, ChitPayloadJSONMaximumBytes)
	if err != nil {
		return err
	}
	candidate := ManifestMembershipPayload(decoded)
	if err := candidate.Validate(); err != nil {
		return jsonError(err)
	}
	*p = candidate
	return nil
}

func (p ManifestMembershipPayload) WriteCanonical(destination io.Writer) error {
	if core.WriterIsNil(destination) {
		return contractError(io.ErrClosedPipe)
	}
	data, err := p.MarshalJSON()
	if err != nil {
		return err
	}
	written, err := destination.Write(data)
	if err != nil {
		return contractError(err)
	}
	if written != len(data) {
		return contractError(io.ErrShortWrite)
	}
	return nil
}

type ManifestMembershipDocument struct {
	Payload     ManifestMembershipPayload      `json:"payload"`
	Attestation attest.Envelope[SigningDomain] `json:"attestation"`
}

func (d ManifestMembershipDocument) Validate() error {
	if err := errors.Join(d.Payload.Validate(), d.Attestation.Validate()); err != nil {
		return contractError(err)
	}
	if d.Attestation.Domain != SigningDomainManifestMembershipV1 {
		return verificationError(errors.New("manifest membership signing domain differs"))
	}
	return nil
}

func (d ManifestMembershipDocument) MarshalJSON() ([]byte, error) {
	if err := d.Validate(); err != nil {
		return nil, jsonError(err)
	}
	type wire ManifestMembershipDocument
	data, err := core.MarshalCanonicalJSONDocument(wire(d))
	if err != nil || len(data) > ChitDocumentJSONMaximumBytes {
		return nil, jsonError(err)
	}
	return data, nil
}

func (d *ManifestMembershipDocument) UnmarshalJSON(data []byte) error {
	if d == nil {
		return jsonError(errors.New("nil manifest membership document receiver"))
	}
	type wire ManifestMembershipDocument
	decoded, err := decodeStrict[wire](data, ChitDocumentJSONMaximumBytes)
	if err != nil {
		return err
	}
	candidate := ManifestMembershipDocument(decoded)
	if err := candidate.Validate(); err != nil {
		return jsonError(err)
	}
	*d = candidate
	return nil
}

type ManifestMembershipIssuance struct {
	Signer  crypto.Signer
	Custody Verified
	Entry   VerifiedManifestEntry
}

func (r ManifestMembershipIssuance) Validate() error {
	_, err := r.payload()
	return err
}

func (r ManifestMembershipIssuance) payload() (ManifestMembershipPayload, error) {
	custody, custodyErr := r.Custody.Document()
	addition, entryErr := r.Entry.Addition()
	summary, summaryErr := r.Entry.Summary()
	if err := errors.Join(custodyErr, entryErr, summaryErr); err != nil {
		return ManifestMembershipPayload{}, err
	}
	if summary != custody.Payload.Manifest {
		return ManifestMembershipPayload{}, conflictError(errors.New("manifest membership closure differs"))
	}
	payload := ManifestMembershipPayload{Custody: custody.Payload, Entry: addition.Entry}
	if err := (attest.SignRequest[SigningDomain]{Body: payload, Signer: r.Signer}).Validate(); err != nil {
		return ManifestMembershipPayload{}, contractError(err)
	}
	return payload, nil
}

// IssueManifestMembership accepts only the compiler-visible result of a
// complete verified fold. A signed object receipt alone cannot enter this path.
func IssueManifestMembership(request ManifestMembershipIssuance) (ManifestMembershipDocument, error) {
	payload, err := request.payload()
	if err != nil {
		return ManifestMembershipDocument{}, err
	}
	envelope, err := attest.Sign(attest.SignRequest[SigningDomain]{Body: payload, Signer: request.Signer})
	if err != nil {
		return ManifestMembershipDocument{}, contractError(err)
	}
	document := ManifestMembershipDocument{Payload: payload, Attestation: envelope}
	if err := document.Validate(); err != nil {
		return ManifestMembershipDocument{}, err
	}
	return document, nil
}

type ManifestMembershipVerification struct {
	Document    ManifestMembershipDocument
	Custody     Verified
	Evidence    receipt.VerifiedEvidence
	Sequence    EntrySequence
	TrustedKeys attest.TrustedKeys
}

func (r ManifestMembershipVerification) Validate() error {
	return errors.Join(r.Document.Validate(), r.Custody.Validate(), r.Evidence.Validate(), r.Sequence.Validate(), r.TrustedKeys.Validate())
}

// VerifyManifestMembership restores one exact member without rescanning its
// siblings. It preserves the complete-stream proof made by the producer and
// authenticates the object receipt independently under the caller's scope.
func VerifyManifestMembership(request ManifestMembershipVerification) (VerifiedManifestEntry, error) {
	if err := request.Validate(); err != nil {
		return VerifiedManifestEntry{}, contractError(err)
	}
	custody, custodyErr := request.Custody.Document()
	evidence, evidenceErr := request.Evidence.Document()
	if err := errors.Join(custodyErr, evidenceErr); err != nil {
		return VerifiedManifestEntry{}, err
	}
	if _, err := attest.Verify(attest.VerifyRequest[SigningDomain]{Body: request.Document.Payload, Envelope: request.Document.Attestation, TrustedKeys: request.TrustedKeys}); err != nil {
		return VerifiedManifestEntry{}, verificationError(err)
	}
	payload := request.Document.Payload
	if payload.Custody != custody.Payload || payload.Entry.Sequence != request.Sequence || payload.Entry.Evidence != evidence {
		return VerifiedManifestEntry{}, conflictError(errors.New("manifest membership expectation differs"))
	}
	proof := VerifiedManifestEntry{addition: ManifestAddition{Entry: payload.Entry, Evidence: request.Evidence}, summary: payload.Custody.Manifest}
	if err := proof.Validate(); err != nil {
		return VerifiedManifestEntry{}, err
	}
	return proof, nil
}

var (
	_ core.ValidatedJSONMarshaler         = ManifestMembershipPayload{}
	_ core.ValidatedJSONMarshaler         = ManifestMembershipDocument{}
	_ attest.CanonicalBody[SigningDomain] = ManifestMembershipPayload{}
	_ core.Validatable                    = ManifestMembershipIssuance{}
	_ core.Validatable                    = ManifestMembershipVerification{}
)
