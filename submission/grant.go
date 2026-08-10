package submission

import (
	"crypto"
	"encoding/json"
	"errors"
	"io"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/objectstore"
	"github.com/deliri/primitive/v2026/temporal"
)

const (
	// GrantPayloadJSONMaximumBytes bounds the authority-signed authorization.
	GrantPayloadJSONMaximumBytes = 32 << 10
	// GrantDocumentJSONMaximumBytes bounds one received grant including bearer.
	GrantDocumentJSONMaximumBytes = 128 << 10
)

// GrantPayload is the complete authority-signed permission for one exact
// request and bearer capability.
type GrantPayload struct {
	Request       RequestCommitment                      `json:"request_commitment"`
	Authorization controlwire.AuthorityNonce             `json:"authorization_nonce"`
	Capability    objectstore.UploadCapabilityCommitment `json:"capability_commitment"`
	IssuedAt      temporal.Instant                       `json:"issued_at"`
	ExpiresAt     temporal.Instant                       `json:"expires_at"`
	RetainUntil   temporal.Instant                       `json:"retain_until"`
}

// GrantDocument is the receive-only authority response. Its capability is a
// bearer and therefore cannot be marshalled back out.
type GrantDocument struct {
	Capability  objectstore.UploadCapability   `json:"capability"`
	Payload     GrantPayload                   `json:"payload"`
	Attestation attest.Envelope[SigningDomain] `json:"attestation"`
}

// GrantProjection is the issue-only authority response carrying the same wire
// shape as GrantDocument through Objectstore's encode-only bearer projection.
type GrantProjection struct {
	Capability  objectstore.UploadCapabilityProjection
	Payload     GrantPayload
	Attestation attest.Envelope[SigningDomain]
}

// GrantIssuance carries all authority-side grant inputs.
type GrantIssuance struct {
	Signer     crypto.Signer
	Capability objectstore.UploadCapabilityProjection
	Payload    GrantPayload
}

// Validate closes every issuance input without producing a signature or wire
// document.
func (i GrantIssuance) Validate() error {
	if err := errors.Join(i.Payload.Validate(), i.Capability.Validate()); err != nil {
		return contractError(err)
	}
	if err := validateProjectionBinding(i.Payload, i.Capability); err != nil {
		return err
	}
	if err := (attest.SignRequest[SigningDomain]{
		Body: i.Payload, Signer: i.Signer,
	}).Validate(); err != nil {
		return contractError(err)
	}
	return nil
}

// GrantExpectation binds an untrusted response to one request and observation
// instant under caller-selected authority keys.
type GrantExpectation struct {
	Request     RequestPayload
	Document    GrantDocument
	TrustedKeys attest.TrustedKeys
	ObservedAt  temporal.Instant
}

// VerifiedGrant proves an exact, current grant authenticated and bound.
type VerifiedGrant struct {
	document GrantDocument
	proof    attest.Verified[SigningDomain]
}

type (
	grantPayloadWire  GrantPayload
	grantDocumentWire struct {
		Capability  objectstore.UploadCapability   `json:"capability"`
		Payload     GrantPayload                   `json:"payload"`
		Attestation attest.Envelope[SigningDomain] `json:"attestation"`
	}
	grantProjectionWire struct {
		Capability  objectstore.UploadCapabilityProjection `json:"capability"`
		Payload     GrantPayload                           `json:"payload"`
		Attestation attest.Envelope[SigningDomain]         `json:"attestation"`
	}
)

// Validate closes all signed permission facts and their temporal ordering.
func (p GrantPayload) Validate() error {
	if err := errors.Join(
		p.Request.Validate(), p.Authorization.Validate(), p.Capability.Validate(),
		p.IssuedAt.Validate(), p.ExpiresAt.Validate(), p.RetainUntil.Validate(),
	); err != nil {
		return contractError(err)
	}
	issuedToExpiry, issuedErr := p.IssuedAt.Compare(p.ExpiresAt)
	expiryToRetention, retainErr := p.ExpiresAt.Compare(p.RetainUntil)
	if errors.Join(issuedErr, retainErr) != nil ||
		issuedToExpiry != core.ComparisonLess || expiryToRetention != core.ComparisonLess {
		return contractError(issuedErr, retainErr, errors.New("grant lifetime is not strictly ordered"))
	}
	return nil
}

// AttestationDomain returns the authority grant namespace.
func (GrantPayload) AttestationDomain() SigningDomain { return SigningDomainGrantV1 }

// WriteCanonical writes the exact compact signed payload.
func (p GrantPayload) WriteCanonical(destination io.Writer) error {
	encoded, err := p.MarshalJSON()
	if err != nil {
		return err
	}
	_, err = destination.Write(encoded)
	return err
}

// MarshalJSON emits one bounded canonical grant payload.
func (p GrantPayload) MarshalJSON() ([]byte, error) {
	if err := p.Validate(); err != nil {
		return nil, jsonError(err)
	}
	encoded, err := core.MarshalCanonicalJSONDocument(grantPayloadWire(p))
	if err != nil || len(encoded) > GrantPayloadJSONMaximumBytes {
		return nil, jsonError(err)
	}
	return encoded, nil
}

// UnmarshalJSON strictly decodes without mutating the receiver on rejection.
func (p *GrantPayload) UnmarshalJSON(data []byte) error {
	if p == nil {
		return jsonError(errors.New("nil grant payload receiver"))
	}
	wire, err := decodeStrict[grantPayloadWire](data, GrantPayloadJSONMaximumBytes)
	if err != nil {
		return err
	}
	candidate := GrantPayload(wire)
	if err := candidate.Validate(); err != nil {
		return jsonError(err)
	}
	*p = candidate
	return nil
}

// Validate binds the separately transported bearer to the signed commitment
// and expiry.
func (d GrantDocument) Validate() error {
	if err := errors.Join(d.Payload.Validate(), d.Attestation.Validate(), d.Capability.Validate()); err != nil {
		return contractError(err)
	}
	if d.Attestation.Domain != d.Payload.AttestationDomain() {
		return bindingError()
	}
	return validateCapabilityBinding(d.Payload, d.Capability)
}

// UnmarshalJSON accepts the receive-only grant wire and preserves the receiver
// on every rejection.
func (d *GrantDocument) UnmarshalJSON(data []byte) error {
	if d == nil {
		return jsonError(errors.New("nil grant document receiver"))
	}
	wire, err := decodeStrict[grantDocumentWire](data, GrantDocumentJSONMaximumBytes)
	if err != nil {
		return err
	}
	candidate := GrantDocument(wire)
	if err := candidate.Validate(); err != nil {
		return jsonError(err)
	}
	*d = candidate
	return nil
}

// Validate closes the issue-only response and binds its bearer projection.
func (p GrantProjection) Validate() error {
	if err := errors.Join(p.Payload.Validate(), p.Attestation.Validate(), p.Capability.Validate()); err != nil {
		return contractError(err)
	}
	if p.Attestation.Domain != p.Payload.AttestationDomain() {
		return bindingError()
	}
	return validateProjectionBinding(p.Payload, p.Capability)
}

// MarshalJSON emits the sole boundary at which the authority discloses the
// upload bearer.
func (p GrantProjection) MarshalJSON() ([]byte, error) {
	if err := p.Validate(); err != nil {
		return nil, jsonError(err)
	}
	encoded, err := core.MarshalCanonicalJSONDocument(grantProjectionWire(p))
	if err != nil || len(encoded) > GrantDocumentJSONMaximumBytes {
		return nil, jsonError(err)
	}
	return encoded, nil
}

// IssueGrant signs one exact grant and returns its encode-only bearer projection.
func IssueGrant(issuance GrantIssuance) (GrantProjection, error) {
	if err := issuance.Validate(); err != nil {
		return GrantProjection{}, err
	}
	envelope, err := attest.Sign(attest.SignRequest[SigningDomain]{
		Body: issuance.Payload, Signer: issuance.Signer,
	})
	if err != nil {
		return GrantProjection{}, contractError(err)
	}
	projection := GrantProjection{
		Payload: issuance.Payload, Attestation: envelope, Capability: issuance.Capability,
	}
	if err := projection.Validate(); err != nil {
		return GrantProjection{}, err
	}
	return projection, nil
}

// Validate closes every caller-supplied verification fact.
func (e GrantExpectation) Validate() error {
	if err := errors.Join(
		e.Document.Validate(), e.Request.Validate(), e.ObservedAt.Validate(), e.TrustedKeys.Validate(),
	); err != nil {
		return contractError(err)
	}
	return nil
}

// VerifyGrant authenticates the authority signature, binds the exact request,
// and refuses a grant observed outside its signed capability lifetime.
func VerifyGrant(expectation GrantExpectation) (VerifiedGrant, error) {
	if err := expectation.Validate(); err != nil {
		return VerifiedGrant{}, err
	}
	proof, err := attest.Verify(attest.VerifyRequest[SigningDomain]{
		Body: expectation.Document.Payload, Envelope: expectation.Document.Attestation,
		TrustedKeys: expectation.TrustedKeys,
	})
	if err != nil {
		return VerifiedGrant{}, contractError(err)
	}
	wantRequest, err := CommitRequest(expectation.Request)
	if err != nil || wantRequest != expectation.Document.Payload.Request {
		return VerifiedGrant{}, bindingError(err)
	}
	if err := validateObservedLifetime(expectation.Document.Payload, expectation.ObservedAt); err != nil {
		return VerifiedGrant{}, err
	}
	verified := VerifiedGrant{document: expectation.Document, proof: proof}
	return verified, verified.Validate()
}

func validateProjectionBinding(
	payload GrantPayload,
	capability objectstore.UploadCapabilityProjection,
) error {
	encoded, err := capability.MarshalJSON()
	if err != nil {
		return bindingError(err)
	}
	var received objectstore.UploadCapability
	if err := json.Unmarshal(encoded, &received); err != nil {
		return bindingError(err)
	}
	return validateCapabilityBinding(payload, received)
}

func validateCapabilityBinding(payload GrantPayload, capability objectstore.UploadCapability) error {
	commitment, err := capability.Commitment()
	if err != nil || commitment != payload.Capability {
		return bindingError(err)
	}
	target, err := capability.Target()
	if err != nil {
		return bindingError(err)
	}
	comparison, err := target.ExpiresAt.Compare(payload.ExpiresAt)
	if err != nil || comparison != core.ComparisonEqual {
		return bindingError(err)
	}
	return nil
}

func validateObservedLifetime(payload GrantPayload, observed temporal.Instant) error {
	issuedComparison, issuedErr := observed.Compare(payload.IssuedAt)
	expiryComparison, expiryErr := observed.Compare(payload.ExpiresAt)
	if errors.Join(issuedErr, expiryErr) != nil ||
		issuedComparison == core.ComparisonLess || expiryComparison != core.ComparisonLess {
		return bindingError(issuedErr, expiryErr, errors.New("grant is not current"))
	}
	return nil
}

// Validate revalidates the authenticated grant proof.
func (v VerifiedGrant) Validate() error {
	if err := errors.Join(v.document.Validate(), v.proof.Validate()); err != nil {
		return contractError(err)
	}
	return nil
}

// Capability returns the authenticated, current upload bearer.
func (v VerifiedGrant) Capability() (objectstore.UploadCapability, error) {
	if err := v.Validate(); err != nil {
		return objectstore.UploadCapability{}, err
	}
	return v.document.Capability, nil
}

// Payload returns the authenticated permission and retention promise.
func (v VerifiedGrant) Payload() (GrantPayload, error) {
	if err := v.Validate(); err != nil {
		return GrantPayload{}, err
	}
	return v.document.Payload, nil
}

var (
	_ core.Validatable = GrantPayload{}
	_ core.Validatable = GrantDocument{}
	_ core.Validatable = GrantProjection{}
	_ core.Validatable = GrantIssuance{}
	_ core.Validatable = GrantExpectation{}
	_ core.Validatable = VerifiedGrant{}

	_ core.ValidatedJSONMarshaler         = GrantPayload{}
	_ core.ValidatedJSONMarshaler         = GrantProjection{}
	_ json.Unmarshaler                    = (*GrantPayload)(nil)
	_ json.Unmarshaler                    = (*GrantDocument)(nil)
	_ attest.CanonicalBody[SigningDomain] = GrantPayload{}
)
