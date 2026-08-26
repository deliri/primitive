package chit

import (
	"crypto"
	json "encoding/json/v2"
	"errors"
	"io"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/receipt"
	"github.com/deliri/primitive/v2026/temporal"
)

const (
	ChitPayloadJSONMaximumBytes  = 32 << 10
	ChitDocumentJSONMaximumBytes = 64 << 10
)

// Version is the positive monotonic version within one collection.
type Version struct{ value uint64 }

func NewVersion(value uint64) (Version, error) {
	candidate := Version{value: value}
	if err := candidate.Validate(); err != nil {
		return Version{}, err
	}
	return candidate, nil
}

func (v Version) Validate() error {
	if v.value == 0 {
		return contractError(errors.New("chit version is zero"))
	}
	return nil
}

func (v Version) Uint64() uint64 { return v.value }

func (v Version) MarshalJSON() ([]byte, error) {
	if err := v.Validate(); err != nil {
		return nil, jsonError(err)
	}
	return json.Marshal(v.value)
}

func (v *Version) UnmarshalJSON(data []byte) error {
	if v == nil {
		return jsonError(errors.New("nil chit version receiver"))
	}
	var value uint64
	if err := json.Unmarshal(data, &value); err != nil {
		return jsonError(err)
	}
	candidate, err := NewVersion(value)
	if err != nil {
		return jsonError(err)
	}
	canonical, _ := candidate.MarshalJSON()
	if string(canonical) != string(data) {
		return jsonError(errors.New("chit version is not canonical"))
	}
	*v = candidate
	return nil
}

// Payload is the immutable authority statement for one logical uploaded
// version. The manifest summary closes every object without embedding a slice.
type Payload struct {
	Scope       receipt.Scope    `json:"scope"`
	Manifest    ManifestSummary  `json:"manifest"`
	AcceptedAt  temporal.Instant `json:"accepted_at"`
	RetainUntil temporal.Instant `json:"retain_until"`
	Version     Version          `json:"version"`
	Identity    ChitID           `json:"chit_id"`
	Collection  CollectionID     `json:"collection_id"`
	Partition   Partition        `json:"partition"`
}

func (p Payload) Validate() error {
	if err := errors.Join(
		p.Identity.Validate(), p.Collection.Validate(), p.Partition.Validate(), p.Scope.Validate(),
		p.Manifest.Validate(), p.AcceptedAt.Validate(), p.RetainUntil.Validate(),
		p.Version.Validate(),
	); err != nil {
		return contractError(err)
	}
	order, err := p.AcceptedAt.Compare(p.RetainUntil)
	if err != nil || order != core.ComparisonLess {
		return contractError(errors.New("chit retention does not follow acceptance"), err)
	}
	return nil
}

func (Payload) AttestationDomain() SigningDomain { return SigningDomainChitV1 }

func (p Payload) WriteCanonical(destination io.Writer) error {
	if destination == nil {
		return contractError(errors.New("chit canonical destination is nil"))
	}
	encoded, err := p.MarshalJSON()
	if err != nil {
		return err
	}
	written, err := destination.Write(encoded)
	if err != nil {
		return contractError(err)
	}
	if written != len(encoded) {
		return contractError(io.ErrShortWrite)
	}
	return nil
}

func (p Payload) MarshalJSON() ([]byte, error) {
	if err := p.Validate(); err != nil {
		return nil, jsonError(err)
	}
	type wire Payload
	encoded, err := core.MarshalCanonicalJSONDocument(wire(p))
	if err != nil || len(encoded) > ChitPayloadJSONMaximumBytes {
		return nil, jsonError(err)
	}
	return encoded, nil
}

func (p *Payload) UnmarshalJSON(data []byte) error {
	if p == nil {
		return jsonError(errors.New("nil chit payload receiver"))
	}
	type wire Payload
	decoded, err := decodeStrict[wire](data, ChitPayloadJSONMaximumBytes)
	if err != nil {
		return err
	}
	candidate := Payload(decoded)
	if err := candidate.Validate(); err != nil {
		return jsonError(err)
	}
	*p = candidate
	return nil
}

// Document carries one authority-signed immutable chit.
type Document struct {
	Payload     Payload                        `json:"payload"`
	Attestation attest.Envelope[SigningDomain] `json:"attestation"`
}

func (d Document) Validate() error {
	if err := errors.Join(d.Payload.Validate(), d.Attestation.Validate()); err != nil {
		return contractError(err)
	}
	if d.Attestation.Domain != d.Payload.AttestationDomain() {
		return verificationError(errors.New("chit signing domain differs"))
	}
	return nil
}

func (d Document) MarshalJSON() ([]byte, error) {
	if err := d.Validate(); err != nil {
		return nil, jsonError(err)
	}
	type wire Document
	encoded, err := core.MarshalCanonicalJSONDocument(wire(d))
	if err != nil || len(encoded) > ChitDocumentJSONMaximumBytes {
		return nil, jsonError(err)
	}
	return encoded, nil
}

func (d *Document) UnmarshalJSON(data []byte) error {
	if d == nil {
		return jsonError(errors.New("nil chit document receiver"))
	}
	type wire Document
	decoded, err := decodeStrict[wire](data, ChitDocumentJSONMaximumBytes)
	if err != nil {
		return err
	}
	candidate := Document(decoded)
	if err := candidate.Validate(); err != nil {
		return jsonError(err)
	}
	*d = candidate
	return nil
}

type Issuance struct {
	Signer      crypto.Signer
	Existing    *Document
	Payload     Payload
	TrustedKeys attest.TrustedKeys
}

func (i Issuance) Validate() error {
	if err := errors.Join(
		(attest.SignRequest[SigningDomain]{Body: i.Payload, Signer: i.Signer}).Validate(),
		i.TrustedKeys.Validate(),
	); err != nil {
		return contractError(err)
	}
	if i.Existing != nil {
		if err := i.Existing.Validate(); err != nil {
			return contractError(err)
		}
	}
	return nil
}

func Issue(issuance Issuance) (Document, error) {
	if err := issuance.Validate(); err != nil {
		return Document{}, err
	}
	if issuance.Existing != nil {
		return convergeExisting(issuance)
	}
	return issueFresh(issuance)
}

func convergeExisting(issuance Issuance) (Document, error) {
	existing := *issuance.Existing
	verified, err := Verify(Verification{
		Document: existing,
		Expected: Expectation{
			Identity: existing.Payload.Identity,
			Scope:    existing.Payload.Scope,
		},
		TrustedKeys: issuance.TrustedKeys,
	})
	if err != nil {
		return Document{}, err
	}
	if existing.Payload != issuance.Payload {
		return Document{}, conflictError(errors.New("occupied chit version differs"))
	}
	return verified.Document()
}

func issueFresh(issuance Issuance) (Document, error) {
	envelope, err := attest.Sign(attest.SignRequest[SigningDomain]{Body: issuance.Payload, Signer: issuance.Signer})
	if err != nil {
		return Document{}, contractError(err)
	}
	document := Document{Payload: issuance.Payload, Attestation: envelope}
	verified, err := Verify(Verification{
		Document: document,
		Expected: Expectation{
			Identity: issuance.Payload.Identity,
			Scope:    issuance.Payload.Scope,
		},
		TrustedKeys: issuance.TrustedKeys,
	})
	if err != nil {
		return Document{}, err
	}
	return verified.Document()
}

// Expectation prevents a valid chit for another account, offering, or ID from
// satisfying a caller's request.
type Expectation struct {
	Scope    receipt.Scope
	Identity ChitID
}

func (e Expectation) Validate() error {
	if err := errors.Join(e.Identity.Validate(), e.Scope.Validate()); err != nil {
		return contractError(err)
	}
	return nil
}

type Verification struct {
	Expected    Expectation
	Document    Document
	TrustedKeys attest.TrustedKeys
}

func (v Verification) Validate() error {
	if err := errors.Join(v.Document.Validate(), v.Expected.Validate(), v.TrustedKeys.Validate()); err != nil {
		return contractError(err)
	}
	return nil
}

// Verified is the sealed authentication result.
type Verified struct {
	document Document
	proof    attest.Verified[SigningDomain]
}

func Verify(verification Verification) (Verified, error) {
	if err := verification.Validate(); err != nil {
		return Verified{}, err
	}
	proof, err := attest.Verify(attest.VerifyRequest[SigningDomain]{
		Body: verification.Document.Payload, Envelope: verification.Document.Attestation,
		TrustedKeys: verification.TrustedKeys,
	})
	if err != nil {
		return Verified{}, verificationError(err)
	}
	if verification.Document.Payload.Identity != verification.Expected.Identity ||
		verification.Document.Payload.Scope != verification.Expected.Scope {
		return Verified{}, conflictError(errors.New("chit scope differs from expectation"))
	}
	verified := Verified{document: verification.Document, proof: proof}
	return verified, verified.Validate()
}

func (v Verified) Validate() error {
	if err := errors.Join(v.document.Validate(), v.proof.Validate()); err != nil {
		return verificationError(err)
	}
	return nil
}

func (v Verified) Document() (Document, error) {
	if err := v.Validate(); err != nil {
		return Document{}, err
	}
	return v.document, nil
}

func decodeStrict[T any](data []byte, maximum uint64) (T, error) {
	var zero T
	limit, err := core.NewByteCount(maximum)
	if err != nil {
		return zero, jsonError(err)
	}
	limits := core.DefaultStrictJSONLimits()
	limits.DocumentMaximumBytes = limit
	decoded, err := core.DecodeStrictJSONStructure[T](data, limits)
	if err != nil {
		return zero, jsonError(err)
	}
	return decoded, nil
}

var (
	_ core.Validatable                    = Version{}
	_ core.Validatable                    = Payload{}
	_ core.Validatable                    = Document{}
	_ core.Validatable                    = Issuance{}
	_ core.Validatable                    = Expectation{}
	_ core.Validatable                    = Verification{}
	_ core.Validatable                    = Verified{}
	_ core.ValidatedJSONMarshaler         = Version{}
	_ core.ValidatedJSONMarshaler         = Payload{}
	_ core.ValidatedJSONMarshaler         = Document{}
	_ attest.CanonicalBody[SigningDomain] = Payload{}
)
