package receipt

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"hash/crc32"
	"io"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

const (
	evidenceHeaderCanonicalJSONMaximumBytes = len(
		`{"receipt_identity":"","account_identity":"","offering_identity":"","revision":"","occurred_at_nanoseconds":""}`,
	) + ReceiptIDHexBytes + 2*LifecycleIdentityHexBytes + len("v1") + 20
	evidenceBodyCanonicalJSONMaximumBytes = len(
		`{"submission_identity":"","object_identity":"","extent_bytes":,"sha256":"","crc32c":""}`,
	) + 2*LifecycleIdentityHexBytes + 20 + 64 + core.CRC32CBase64Bytes
	// EvidencePayloadCanonicalJSONMaximumBytes is the exact maximum canonical payload extent.
	EvidencePayloadCanonicalJSONMaximumBytes = len(`{"header":,"body":}`) +
		evidenceHeaderCanonicalJSONMaximumBytes + evidenceBodyCanonicalJSONMaximumBytes
	// attestBodyLengthDigitsMaximum is the widest decimal body length Attest
	// budgets for, at its own one-mebibyte canonical body ceiling.
	attestBodyLengthDigitsMaximum = 7
	// evidenceBodyLengthDigitsMaximum is the widest decimal body length a
	// Receipt payload can reach, since the payload is bounded far below Attest's
	// ceiling. Attest's envelope maximum is narrowed by the digits Receipt
	// cannot spend and by the domain text it does not use.
	evidenceBodyLengthDigitsMaximum          = 3
	receiptEnvelopeCanonicalJSONMaximumBytes = attest.EnvelopeCanonicalJSONMaximumBytes -
		(attest.SigningDomainMaximumBytes - len(evidenceDomainToken)) -
		(attestBodyLengthDigitsMaximum - evidenceBodyLengthDigitsMaximum)
	// EvidenceDocumentCanonicalJSONMaximumBytes is the exact maximum compact document extent.
	EvidenceDocumentCanonicalJSONMaximumBytes = len(`{"payload":,"attestation":}`) +
		EvidencePayloadCanonicalJSONMaximumBytes + receiptEnvelopeCanonicalJSONMaximumBytes
	evidenceDocumentWhitespaceAllowance = 8 << 10
	// EvidenceDocumentJSONMaximumBytes bounds accepted strict JSON including whitespace.
	EvidenceDocumentJSONMaximumBytes = EvidenceDocumentCanonicalJSONMaximumBytes +
		evidenceDocumentWhitespaceAllowance
)

// EvidenceBody is the immutable integrity statement for one accepted object.
type EvidenceBody struct {
	Extent     core.ByteLength    `json:"extent_bytes"`
	CRC32C     core.CRC32C        `json:"crc32c"`
	SHA256     core.SHA256Digest  `json:"sha256"`
	Submission SubmissionIdentity `json:"submission_identity"`
	Object     ObjectIdentity     `json:"object_identity"`
}

// Header identifies one signed evidence fact and its scope.
type Header struct {
	Identity   ReceiptID        `json:"receipt_identity"`
	Account    AccountIdentity  `json:"account_identity"`
	Offering   OfferingIdentity `json:"offering_identity"`
	Revision   Revision         `json:"revision"`
	OccurredAt temporal.Instant `json:"occurred_at_nanoseconds"`
}

// EvidencePayload is the canonical typed body signed through Attest.
type EvidencePayload struct {
	Header Header       `json:"header"`
	Body   EvidenceBody `json:"body"`
}

// EvidenceDocument carries one untrusted payload and detached attestation.
type EvidenceDocument struct {
	Payload     EvidencePayload         `json:"payload"`
	Attestation attest.Envelope[Domain] `json:"attestation"`
}

// IssueEvidenceRequest carries exact issuance inputs.
type IssueEvidenceRequest struct {
	Key        ed25519.PrivateKey
	Body       EvidenceBody
	OccurredAt temporal.Instant
	Identity   ReceiptID
	Account    AccountIdentity
	Offering   OfferingIdentity
}

// EvidenceExpectation is the exact scope and integrity a caller will accept.
type EvidenceExpectation struct {
	Account  AccountIdentity
	Offering OfferingIdentity
	Body     EvidenceBody
}

// VerifyEvidenceRequest carries untrusted evidence, caller trust, and intent.
type VerifyEvidenceRequest struct {
	Document    EvidenceDocument
	TrustedKeys attest.TrustedKeys
	Expected    EvidenceExpectation
}

// VerifiedEvidence is an in-package sealed authentication proof.
type VerifiedEvidence struct {
	document EvidenceDocument
	verified bool
}

func (b EvidenceBody) Validate() error {
	if err := b.Submission.Validate(); err != nil {
		return contractError(errors.New("evidence submission is invalid"), err)
	}
	if err := b.Object.Validate(); err != nil {
		return contractError(errors.New("evidence object is invalid"), err)
	}
	if err := b.SHA256.Validate(); err != nil {
		return contractError(errors.New("evidence SHA-256 is invalid"), err)
	}
	if err := b.CRC32C.Validate(); err != nil {
		return contractError(errors.New("evidence CRC32C is invalid"), err)
	}
	if (b.Extent.Uint64() == 0) != isCanonicalEmptyEvidence(b) {
		return contractError(errors.New(
			"evidence extent and integrity disagree about the empty byte stream",
		))
	}
	return nil
}

// isCanonicalEmptyEvidence reports the exact integrity of the empty byte
// stream. Extent and integrity must agree in both directions: an empty extent
// carrying any other digest, and a nonempty extent claiming the empty digest,
// are equally contradictory statements about the same object.
func isCanonicalEmptyEvidence(body EvidenceBody) bool {
	emptySHA := core.NewSHA256Digest(sha256.Sum256(nil))
	emptyCRC := core.NewCRC32C(crc32.Checksum(nil, crc32.MakeTable(crc32.Castagnoli)))
	return body.SHA256 == emptySHA && body.CRC32C == emptyCRC
}

func (h Header) Validate() error {
	if err := h.Identity.Validate(); err != nil {
		return contractError(errors.New("evidence identity is invalid"), err)
	}
	if err := h.Account.Validate(); err != nil {
		return contractError(errors.New("evidence account is invalid"), err)
	}
	if err := h.Offering.Validate(); err != nil {
		return contractError(errors.New("evidence offering is invalid"), err)
	}
	if err := h.Revision.Validate(); err != nil {
		return contractError(errors.New("evidence revision is invalid"), err)
	}
	if err := h.OccurredAt.Validate(); err != nil {
		return contractError(errors.New("evidence occurrence is invalid"), err)
	}
	return nil
}

func (p EvidencePayload) Validate() error {
	if err := p.Header.Validate(); err != nil {
		return contractError(err)
	}
	if err := p.Body.Validate(); err != nil {
		return contractError(err)
	}
	return nil
}

// AttestationDomain returns the Receipt-owned signing domain.
func (EvidencePayload) AttestationDomain() Domain { return DomainEvidenceV1 }

// WriteCanonical writes the compact canonical payload exactly once.
func (p EvidencePayload) WriteCanonical(destination io.Writer) error {
	encoded, err := p.MarshalJSON()
	if err != nil {
		return err
	}
	return writeCanonical(destination, encoded)
}

func (d EvidenceDocument) Validate() error {
	if err := d.Payload.Validate(); err != nil {
		return contractError(err)
	}
	if err := d.Attestation.Validate(); err != nil {
		return contractError(err)
	}
	if d.Attestation.Domain != d.Payload.AttestationDomain() {
		return verificationError(errors.New("receipt attestation domain differs from its payload"))
	}
	return nil
}

// payload projects the request into the exact structure that will be signed.
// Validation and issuance both read it, so what Validate admitted is always
// what Sign covers.
func (r IssueEvidenceRequest) payload() EvidencePayload {
	return EvidencePayload{
		Header: Header{
			Identity: r.Identity, Account: r.Account, Offering: r.Offering,
			Revision: RevisionV1, OccurredAt: r.OccurredAt,
		},
		Body: r.Body,
	}
}

func (r IssueEvidenceRequest) Validate() error {
	payload := r.payload()
	if err := payload.Validate(); err != nil {
		return err
	}
	if err := (attest.SignRequest[Domain]{Body: payload, Key: r.Key}).Validate(); err != nil {
		return contractError(err)
	}
	return nil
}

// IssueEvidence signs one fully typed accepted-evidence fact.
func IssueEvidence(request IssueEvidenceRequest) (EvidenceDocument, error) {
	if err := request.Validate(); err != nil {
		return EvidenceDocument{}, err
	}
	payload := request.payload()
	envelope, err := attest.Sign(attest.SignRequest[Domain]{
		Body: payload,
		Key:  request.Key,
	})
	if err != nil {
		return EvidenceDocument{}, contractError(err)
	}
	document := EvidenceDocument{Payload: payload, Attestation: envelope}
	if err := document.Validate(); err != nil {
		return EvidenceDocument{}, err
	}
	return document, nil
}

func (e EvidenceExpectation) Validate() error {
	if err := e.Account.Validate(); err != nil {
		return contractError(errors.New("expected account is invalid"), err)
	}
	if err := e.Offering.Validate(); err != nil {
		return contractError(errors.New("expected offering is invalid"), err)
	}
	if err := e.Body.Validate(); err != nil {
		return contractError(errors.New("expected evidence body is invalid"), err)
	}
	return nil
}

func (r VerifyEvidenceRequest) Validate() error {
	if err := r.Document.Validate(); err != nil {
		return verificationError(err)
	}
	if err := r.TrustedKeys.Validate(); err != nil {
		return verificationError(err)
	}
	if err := r.Expected.Validate(); err != nil {
		return contractError(err)
	}
	return nil
}

// VerifyEvidence authenticates a document before comparing caller intent.
func VerifyEvidence(request VerifyEvidenceRequest) (VerifiedEvidence, error) {
	if err := request.Validate(); err != nil {
		return VerifiedEvidence{}, err
	}
	if _, err := attest.Verify(attest.VerifyRequest[Domain]{
		Body:        request.Document.Payload,
		Envelope:    request.Document.Attestation,
		TrustedKeys: request.TrustedKeys,
	}); err != nil {
		return VerifiedEvidence{}, verificationError(err)
	}
	if err := compareExpectation(request.Document.Payload, request.Expected); err != nil {
		return VerifiedEvidence{}, err
	}
	verified := VerifiedEvidence{document: request.Document, verified: true}
	if err := verified.Validate(); err != nil {
		return VerifiedEvidence{}, err
	}
	return verified, nil
}

func compareExpectation(payload EvidencePayload, expected EvidenceExpectation) error {
	switch {
	case payload.Header.Account != expected.Account:
		return newScopeMismatch(ScopeFieldAccount)
	case payload.Header.Offering != expected.Offering:
		return newScopeMismatch(ScopeFieldOffering)
	case payload.Body.Submission != expected.Body.Submission:
		return newScopeMismatch(ScopeFieldSubmission)
	case payload.Body.Object != expected.Body.Object:
		return newScopeMismatch(ScopeFieldObject)
	case payload.Body.Extent != expected.Body.Extent:
		return newScopeMismatch(ScopeFieldExtent)
	case payload.Body.SHA256 != expected.Body.SHA256:
		return newScopeMismatch(ScopeFieldSHA256)
	case payload.Body.CRC32C != expected.Body.CRC32C:
		return newScopeMismatch(ScopeFieldCRC32C)
	default:
		return nil
	}
}

// Validate rejects the zero proof and validates its retained document.
func (v VerifiedEvidence) Validate() error {
	if !v.verified {
		return verificationError(errors.New("verified evidence is unset"))
	}
	if err := v.document.Validate(); err != nil {
		return verificationError(err)
	}
	return nil
}

// Document returns the exact authenticated document.
func (v VerifiedEvidence) Document() (EvidenceDocument, error) {
	if err := v.Validate(); err != nil {
		return EvidenceDocument{}, err
	}
	return v.document, nil
}

// Header returns the authenticated header.
func (v VerifiedEvidence) Header() (Header, error) {
	if err := v.Validate(); err != nil {
		return Header{}, err
	}
	return v.document.Payload.Header, nil
}

// Body returns the authenticated evidence body.
func (v VerifiedEvidence) Body() (EvidenceBody, error) {
	if err := v.Validate(); err != nil {
		return EvidenceBody{}, err
	}
	return v.document.Payload.Body, nil
}

// evidenceBodyWire fixes the canonical member order for both directions. Every
// member is a pointer, so every field has the same size and alignment and no
// layout optimizer can reorder the signed contract. Reordering members here
// changes the signed bytes; reordering EvidenceBody's own fields does not.
type evidenceBodyWire struct {
	Submission *SubmissionIdentity `json:"submission_identity"`
	Object     *ObjectIdentity     `json:"object_identity"`
	Extent     *core.ByteLength    `json:"extent_bytes"`
	SHA256     *core.SHA256Digest  `json:"sha256"`
	CRC32C     *core.CRC32C        `json:"crc32c"`
}

func (b EvidenceBody) MarshalJSON() ([]byte, error) {
	if err := b.Validate(); err != nil {
		return nil, jsonError(err)
	}
	encoded, err := json.Marshal(evidenceBodyWire{
		Submission: &b.Submission,
		Object:     &b.Object,
		Extent:     &b.Extent,
		SHA256:     &b.SHA256,
		CRC32C:     &b.CRC32C,
	})
	if err != nil || len(encoded) > evidenceBodyCanonicalJSONMaximumBytes {
		return nil, jsonError(errors.New("evidence body encoding exceeded its bound"), err)
	}
	return encoded, nil
}

func (b *EvidenceBody) UnmarshalJSON(data []byte) error {
	if b == nil {
		return jsonError(errors.New("nil evidence body receiver"))
	}
	limits, err := (jsonStructureContract{
		maximumBytes: evidenceBodyCanonicalJSONMaximumBytes + 2048,
		depth:        1, fields: 5,
	}).limits()
	if err != nil {
		return err
	}
	wire, err := core.DecodeStrictJSONStructure[evidenceBodyWire](data, limits)
	if err != nil {
		return jsonError(err)
	}
	if wire.Submission == nil || wire.Object == nil || wire.Extent == nil ||
		wire.SHA256 == nil || wire.CRC32C == nil {
		return jsonError(errors.New("evidence body omits a required field"))
	}
	candidate := EvidenceBody{
		Submission: *wire.Submission, Object: *wire.Object, Extent: *wire.Extent,
		SHA256: *wire.SHA256, CRC32C: *wire.CRC32C,
	}
	if err := candidate.Validate(); err != nil {
		return jsonError(err)
	}
	*b = candidate
	return nil
}

// headerWire fixes the canonical member order of the signed header for both
// directions. Every member is a pointer, so no layout optimizer can reorder the
// signed contract.
type headerWire struct {
	Identity   *ReceiptID        `json:"receipt_identity"`
	Account    *AccountIdentity  `json:"account_identity"`
	Offering   *OfferingIdentity `json:"offering_identity"`
	Revision   *Revision         `json:"revision"`
	OccurredAt *temporal.Instant `json:"occurred_at_nanoseconds"`
}

func (h Header) MarshalJSON() ([]byte, error) {
	if err := h.Validate(); err != nil {
		return nil, jsonError(err)
	}
	encoded, err := json.Marshal(headerWire{
		Identity: &h.Identity, Account: &h.Account, Offering: &h.Offering,
		Revision: &h.Revision, OccurredAt: &h.OccurredAt,
	})
	if err != nil || len(encoded) > evidenceHeaderCanonicalJSONMaximumBytes {
		return nil, jsonError(errors.New("evidence header encoding exceeded its bound"), err)
	}
	return encoded, nil
}

func (h *Header) UnmarshalJSON(data []byte) error {
	if h == nil {
		return jsonError(errors.New("nil evidence header receiver"))
	}
	limits, err := (jsonStructureContract{
		maximumBytes: evidenceHeaderCanonicalJSONMaximumBytes + 2048,
		depth:        1, fields: 5,
	}).limits()
	if err != nil {
		return err
	}
	wire, err := core.DecodeStrictJSONStructure[headerWire](data, limits)
	if err != nil {
		return jsonError(err)
	}
	if wire.Identity == nil || wire.Account == nil || wire.Offering == nil ||
		wire.Revision == nil || wire.OccurredAt == nil {
		return jsonError(errors.New("evidence header omits a required field"))
	}
	candidate := Header{
		Identity: *wire.Identity, Account: *wire.Account, Offering: *wire.Offering,
		Revision: *wire.Revision, OccurredAt: *wire.OccurredAt,
	}
	if err := candidate.Validate(); err != nil {
		return jsonError(err)
	}
	*h = candidate
	return nil
}

// payloadWire fixes the canonical member order of the signed payload for both
// directions. Every member is a pointer, so no layout optimizer can reorder the
// signed contract.
type payloadWire struct {
	Header *Header       `json:"header"`
	Body   *EvidenceBody `json:"body"`
}

func (p EvidencePayload) MarshalJSON() ([]byte, error) {
	if err := p.Validate(); err != nil {
		return nil, jsonError(err)
	}
	encoded, err := json.Marshal(payloadWire{Header: &p.Header, Body: &p.Body})
	if err != nil || len(encoded) > EvidencePayloadCanonicalJSONMaximumBytes {
		return nil, jsonError(errors.New("evidence payload encoding exceeded its bound"), err)
	}
	return encoded, nil
}

func (p *EvidencePayload) UnmarshalJSON(data []byte) error {
	if p == nil {
		return jsonError(errors.New("nil evidence payload receiver"))
	}
	limits, err := (jsonStructureContract{
		maximumBytes: EvidencePayloadCanonicalJSONMaximumBytes + 4096,
		depth:        2, fields: 7,
	}).limits()
	if err != nil {
		return err
	}
	wire, err := core.DecodeStrictJSONStructure[payloadWire](data, limits)
	if err != nil {
		return jsonError(err)
	}
	if wire.Header == nil || wire.Body == nil {
		return jsonError(errors.New("evidence payload omits a required field"))
	}
	candidate := EvidencePayload{Header: *wire.Header, Body: *wire.Body}
	if err := candidate.Validate(); err != nil {
		return jsonError(err)
	}
	*p = candidate
	return nil
}

// documentWire fixes the canonical member order of the transported document for
// both directions. Every member is a pointer, so no layout optimizer can
// reorder it.
type documentWire struct {
	Payload     *EvidencePayload         `json:"payload"`
	Attestation *attest.Envelope[Domain] `json:"attestation"`
}

func (d EvidenceDocument) MarshalJSON() ([]byte, error) {
	if err := d.Validate(); err != nil {
		return nil, jsonError(err)
	}
	encoded, err := json.Marshal(documentWire{
		Payload: &d.Payload, Attestation: &d.Attestation,
	})
	if err != nil || len(encoded) > EvidenceDocumentCanonicalJSONMaximumBytes {
		return nil, jsonError(errors.New("evidence document encoding exceeded its bound"), err)
	}
	return encoded, nil
}

func (d *EvidenceDocument) UnmarshalJSON(data []byte) error {
	if d == nil {
		return jsonError(errors.New("nil evidence document receiver"))
	}
	limits, err := (jsonStructureContract{
		maximumBytes: EvidenceDocumentJSONMaximumBytes,
		depth:        4, fields: 12,
	}).limits()
	if err != nil {
		return err
	}
	wire, err := core.DecodeStrictJSONStructure[documentWire](data, limits)
	if err != nil {
		return jsonError(err)
	}
	if wire.Payload == nil || wire.Attestation == nil {
		return jsonError(errors.New("evidence document omits a required field"))
	}
	candidate := EvidenceDocument{Payload: *wire.Payload, Attestation: *wire.Attestation}
	if err := candidate.Validate(); err != nil {
		return jsonError(err)
	}
	*d = candidate
	return nil
}

var (
	_ attest.CanonicalBody[Domain] = EvidencePayload{}
	_ core.Validatable             = EvidenceBody{}
	_ core.Validatable             = Header{}
	_ core.Validatable             = EvidencePayload{}
	_ core.Validatable             = EvidenceDocument{}
	_ core.Validatable             = VerifiedEvidence{}
)
