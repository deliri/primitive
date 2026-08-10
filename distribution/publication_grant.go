package distribution

import (
	"crypto"
	"encoding/json"
	"errors"
	"io"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/objectstore"
	"github.com/deliri/primitive/v2026/release"
	"github.com/deliri/primitive/v2026/temporal"
)

// PublicationGrantPayload is the signed authority permission for every exact
// object in one publication request.
type PublicationGrantPayload struct {
	Request       RequestCommitment                                                      `json:"request_commitment"`
	Authorization controlwire.AuthorityNonce                                             `json:"authorization_nonce"`
	Commitments   [release.PublicationObjectCount]objectstore.UploadCapabilityCommitment `json:"capability_commitments"`
	IssuedAt      temporal.Instant                                                       `json:"issued_at"`
	ExpiresAt     temporal.Instant                                                       `json:"expires_at"`
}

// PublicationGrantDocument is the receive-only form. Objectstore prevents
// callers from serializing its bearer capabilities again.
type PublicationGrantDocument struct {
	Capabilities [release.PublicationObjectCount]objectstore.UploadCapability `json:"capabilities"`
	Payload      PublicationGrantPayload                                      `json:"payload"`
	Attestation  attest.Envelope[SigningDomain]                               `json:"attestation"`
}

// PublicationGrantProjection is the issue-only form and the sole boundary at
// which an authority emits the upload bearers.
type PublicationGrantProjection struct {
	Capabilities [release.PublicationObjectCount]objectstore.UploadCapabilityProjection
	Payload      PublicationGrantPayload
	Attestation  attest.Envelope[SigningDomain]
}

// PublicationGrantIssuance supplies already-issued provider capabilities and
// the authority signer. It does not create provider credentials or URLs.
type PublicationGrantIssuance struct {
	Signer       crypto.Signer
	Capabilities [release.PublicationObjectCount]objectstore.UploadCapabilityProjection
	Payload      PublicationGrantPayload
}

// PublicationGrantExpectation binds one untrusted grant to the request that
// produced it and to the caller-selected authority keys and observation.
type PublicationGrantExpectation struct {
	Document    PublicationGrantDocument
	Request     PublicationRequestPayload
	TrustedKeys attest.TrustedKeys
	ObservedAt  temporal.Instant
}

// VerifiedPublicationGrant is the authenticated upload-capability set.
type VerifiedPublicationGrant struct {
	document PublicationGrantDocument
	request  PublicationRequestPayload
	proof    attest.Verified[SigningDomain]
}

type (
	publicationGrantPayloadWire  PublicationGrantPayload
	publicationGrantDocumentWire struct {
		Capabilities [release.PublicationObjectCount]objectstore.UploadCapability `json:"capabilities"`
		Payload      PublicationGrantPayload                                      `json:"payload"`
		Attestation  attest.Envelope[SigningDomain]                               `json:"attestation"`
	}
	publicationGrantProjectionWire struct {
		Capabilities [release.PublicationObjectCount]objectstore.UploadCapabilityProjection `json:"capabilities"`
		Payload      PublicationGrantPayload                                                `json:"payload"`
		Attestation  attest.Envelope[SigningDomain]                                         `json:"attestation"`
	}
)

func (p PublicationGrantPayload) Validate() error {
	if err := errors.Join(
		p.Request.validateDomain(SigningDomainPublicationRequestV1),
		p.Authorization.Validate(), validateLifetime(p.IssuedAt, p.ExpiresAt),
	); err != nil {
		return contractError(err)
	}
	for index, commitment := range p.Commitments {
		if err := commitment.Validate(); err != nil {
			return contractError(err)
		}
		for prior := range index {
			if commitment == p.Commitments[prior] {
				return contractError(errors.New("publication capability commitment is duplicated"))
			}
		}
	}
	return nil
}

func (PublicationGrantPayload) AttestationDomain() SigningDomain {
	return SigningDomainPublicationGrantV1
}

func (p PublicationGrantPayload) WriteCanonical(destination io.Writer) error {
	encoded, err := p.MarshalJSON()
	if err != nil {
		return err
	}
	return writeCanonical(destination, encoded)
}

func (p PublicationGrantPayload) MarshalJSON() ([]byte, error) {
	if err := p.Validate(); err != nil {
		return nil, jsonError(err)
	}
	encoded, err := core.MarshalCanonicalJSONDocument(publicationGrantPayloadWire(p))
	if err != nil || len(encoded) > responsePayloadJSONMaximumBytes {
		return nil, jsonError(err)
	}
	return encoded, nil
}

func (p *PublicationGrantPayload) UnmarshalJSON(data []byte) error {
	if p == nil {
		return jsonError(errors.New("publication grant payload receiver is nil"))
	}
	wire, err := decodeStrict[publicationGrantPayloadWire](data, responsePayloadJSONMaximumBytes)
	if err != nil {
		return err
	}
	candidate := PublicationGrantPayload(wire)
	if err := candidate.Validate(); err != nil {
		return jsonError(err)
	}
	*p = candidate
	return nil
}

func validatePublicationCapabilitySet(
	capabilities [release.PublicationObjectCount]objectstore.UploadCapability,
	payload PublicationGrantPayload,
) error {
	if err := payload.Validate(); err != nil {
		return err
	}
	for index, capability := range capabilities {
		if err := capability.Validate(); err != nil {
			return contractError(err)
		}
		commitment, err := capability.Commitment()
		if err != nil || commitment != payload.Commitments[index] {
			return bindingError(errors.New("publication bearer differs from signed commitment"), err)
		}
	}
	return nil
}

func validatePublicationProjectionSet(
	capabilities [release.PublicationObjectCount]objectstore.UploadCapabilityProjection,
	payload PublicationGrantPayload,
) error {
	if err := payload.Validate(); err != nil {
		return err
	}
	for index, capability := range capabilities {
		if err := capability.Validate(); err != nil {
			return contractError(err)
		}
		commitment, err := capability.Commitment()
		if err != nil || commitment != payload.Commitments[index] {
			return bindingError(errors.New("publication projection differs from signed commitment"), err)
		}
	}
	return nil
}

func (d PublicationGrantDocument) Validate() error {
	if err := errors.Join(
		validatePublicationCapabilitySet(d.Capabilities, d.Payload),
		d.Attestation.Validate(),
	); err != nil {
		return contractError(err)
	}
	if d.Attestation.Domain != d.Payload.AttestationDomain() {
		return bindingError(errors.New("publication grant attestation domain differs"))
	}
	return nil
}

func (d *PublicationGrantDocument) UnmarshalJSON(data []byte) error {
	if d == nil {
		return jsonError(errors.New("publication grant document receiver is nil"))
	}
	wire, err := decodeStrict[publicationGrantDocumentWire](data, publicationGrantJSONMaximumBytes)
	if err != nil {
		return err
	}
	candidate := PublicationGrantDocument(wire)
	if err := candidate.Validate(); err != nil {
		return jsonError(err)
	}
	*d = candidate
	return nil
}

func (p PublicationGrantProjection) Validate() error {
	if err := errors.Join(
		validatePublicationProjectionSet(p.Capabilities, p.Payload),
		p.Attestation.Validate(),
	); err != nil {
		return contractError(err)
	}
	if p.Attestation.Domain != p.Payload.AttestationDomain() {
		return bindingError(errors.New("publication grant projection domain differs"))
	}
	return nil
}

func (p PublicationGrantProjection) MarshalJSON() ([]byte, error) {
	if err := p.Validate(); err != nil {
		return nil, jsonError(err)
	}
	encoded, err := core.MarshalCanonicalJSONDocument(publicationGrantProjectionWire(p))
	if err != nil || len(encoded) > publicationGrantJSONMaximumBytes {
		return nil, jsonError(err)
	}
	return encoded, nil
}

func (i PublicationGrantIssuance) Validate() error {
	if err := validatePublicationProjectionSet(i.Capabilities, i.Payload); err != nil {
		return err
	}
	if err := (attest.SignRequest[SigningDomain]{Body: i.Payload, Signer: i.Signer}).Validate(); err != nil {
		return contractError(err)
	}
	return nil
}

// IssuePublicationGrant signs the capability commitments and returns the one
// issue-only bearer projection.
func IssuePublicationGrant(issuance PublicationGrantIssuance) (PublicationGrantProjection, error) {
	if err := issuance.Validate(); err != nil {
		return PublicationGrantProjection{}, err
	}
	envelope, err := attest.Sign(attest.SignRequest[SigningDomain]{
		Body: issuance.Payload, Signer: issuance.Signer,
	})
	if err != nil {
		return PublicationGrantProjection{}, contractError(err)
	}
	projection := PublicationGrantProjection{
		Capabilities: issuance.Capabilities, Payload: issuance.Payload, Attestation: envelope,
	}
	return projection, projection.Validate()
}

func (e PublicationGrantExpectation) Validate() error {
	if err := errors.Join(
		e.Request.Validate(), e.Document.Validate(),
		e.TrustedKeys.Validate(), e.ObservedAt.Validate(),
	); err != nil {
		return verificationError(err)
	}
	return nil
}

// VerifyPublicationGrant authenticates, request-binds, and lifetime-binds the
// exact bearer set before any source can be read.
func VerifyPublicationGrant(expectation PublicationGrantExpectation) (VerifiedPublicationGrant, error) {
	if err := expectation.Validate(); err != nil {
		return VerifiedPublicationGrant{}, err
	}
	request, err := CommitRequest(expectation.Request)
	if err != nil || request != expectation.Document.Payload.Request {
		return VerifiedPublicationGrant{}, bindingError(errors.New("publication grant answers another request"), err)
	}
	if err := validateObservedLifetime(
		expectation.Document.Payload.IssuedAt,
		expectation.Document.Payload.ExpiresAt,
		expectation.ObservedAt,
	); err != nil {
		return VerifiedPublicationGrant{}, err
	}
	proof, err := attest.Verify(attest.VerifyRequest[SigningDomain]{
		Body: expectation.Document.Payload, Envelope: expectation.Document.Attestation,
		TrustedKeys: expectation.TrustedKeys,
	})
	if err != nil {
		return VerifiedPublicationGrant{}, verificationError(err)
	}
	verified := VerifiedPublicationGrant{
		document: expectation.Document, request: expectation.Request, proof: proof,
	}
	return verified, verified.Validate()
}

func (v VerifiedPublicationGrant) Validate() error {
	if err := errors.Join(v.document.Validate(), v.request.Validate(), v.proof.Validate()); err != nil {
		return verificationError(err)
	}
	request, err := CommitRequest(v.request)
	if err != nil || request != v.document.Payload.Request {
		return bindingError(errors.New("verified publication grant request differs"), err)
	}
	return nil
}

func (v VerifiedPublicationGrant) Payload() (PublicationGrantPayload, error) {
	if err := v.Validate(); err != nil {
		return PublicationGrantPayload{}, err
	}
	return v.document.Payload, nil
}

func (v VerifiedPublicationGrant) Request() (PublicationRequestPayload, error) {
	if err := v.Validate(); err != nil {
		return PublicationRequestPayload{}, err
	}
	return v.request, nil
}

func (v VerifiedPublicationGrant) Capability(
	role release.PublicationRole,
) (objectstore.UploadCapability, objectstore.UploadCapabilityCommitment, error) {
	if err := v.Validate(); err != nil {
		return objectstore.UploadCapability{}, objectstore.UploadCapabilityCommitment{}, err
	}
	index, err := role.Index()
	if err != nil {
		return objectstore.UploadCapability{}, objectstore.UploadCapabilityCommitment{}, contractError(err)
	}
	return v.document.Capabilities[index], v.document.Payload.Commitments[index], nil
}

var (
	_ core.Validatable = PublicationGrantPayload{}
	_ core.Validatable = PublicationGrantDocument{}
	_ core.Validatable = PublicationGrantProjection{}
	_ core.Validatable = PublicationGrantIssuance{}
	_ core.Validatable = PublicationGrantExpectation{}
	_ core.Validatable = VerifiedPublicationGrant{}

	_ core.ValidatedJSONMarshaler         = PublicationGrantPayload{}
	_ core.ValidatedJSONMarshaler         = PublicationGrantProjection{}
	_ json.Unmarshaler                    = (*PublicationGrantPayload)(nil)
	_ json.Unmarshaler                    = (*PublicationGrantDocument)(nil)
	_ attest.CanonicalBody[SigningDomain] = PublicationGrantPayload{}
)
