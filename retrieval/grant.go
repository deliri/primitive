package retrieval

import (
	"crypto"
	"encoding/json"
	"errors"
	"io"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/chit"
	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/objectstore"
	"github.com/deliri/primitive/v2026/temporal"
)

const (
	GrantPayloadJSONMaximumBytes  = 128 << 10
	GrantDocumentJSONMaximumBytes = 256 << 10
)

// GrantPayload is the complete signed authorization for one exact manifest
// object and one separately transported download bearer.
type GrantPayload struct {
	Entry         chit.ManifestEntry                       `json:"entry"`
	Request       RequestCommitment                        `json:"request_commitment"`
	Authorization controlwire.AuthorityNonce               `json:"authorization_nonce"`
	Capability    objectstore.DownloadCapabilityCommitment `json:"capability_commitment"`
	Manifest      chit.ManifestDigest                      `json:"manifest_digest"`
	Chit          chit.ChitID                              `json:"chit_id"`
	IssuedAt      temporal.Instant                         `json:"issued_at"`
	ExpiresAt     temporal.Instant                         `json:"expires_at"`
}

func (p GrantPayload) Validate() error {
	if err := errors.Join(
		p.Entry.Validate(), p.Request.Validate(), p.Authorization.Validate(),
		p.Capability.Validate(), p.Manifest.Validate(), p.Chit.Validate(),
		p.IssuedAt.Validate(), p.ExpiresAt.Validate(),
	); err != nil {
		return contractError(err)
	}
	order, err := p.IssuedAt.Compare(p.ExpiresAt)
	if err != nil || order != core.ComparisonLess {
		return contractError(errors.New("retrieval grant lifetime is not increasing"), err)
	}
	return nil
}

func (GrantPayload) AttestationDomain() SigningDomain { return SigningDomainGrantV1 }

func (p GrantPayload) WriteCanonical(destination io.Writer) error {
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

func (p GrantPayload) MarshalJSON() ([]byte, error) {
	if err := p.Validate(); err != nil {
		return nil, jsonError(err)
	}
	type wire GrantPayload
	encoded, err := core.MarshalCanonicalJSONDocument(wire(p))
	if err != nil || len(encoded) > GrantPayloadJSONMaximumBytes {
		return nil, jsonError(err)
	}
	return encoded, nil
}

func (p *GrantPayload) UnmarshalJSON(data []byte) error {
	if p == nil {
		return jsonError(errors.New("nil retrieval grant payload receiver"))
	}
	type wire GrantPayload
	decoded, err := decodeStrict[wire](data, GrantPayloadJSONMaximumBytes)
	if err != nil {
		return err
	}
	candidate := GrantPayload(decoded)
	if err := candidate.Validate(); err != nil {
		return jsonError(err)
	}
	*p = candidate
	return nil
}

// GrantDocument is receive-only because its capability is a bearer.
type GrantDocument struct {
	Capability  objectstore.DownloadCapability `json:"capability"`
	Payload     GrantPayload                   `json:"payload"`
	Attestation attest.Envelope[SigningDomain] `json:"attestation"`
}

type grantDocumentWire GrantDocument

func (d GrantDocument) Validate() error {
	if err := errors.Join(d.Capability.Validate(), d.Payload.Validate(), d.Attestation.Validate()); err != nil {
		return contractError(err)
	}
	if d.Attestation.Domain != SigningDomainGrantV1 {
		return bindingError(errors.New("retrieval grant signing domain differs"))
	}
	return validateCapabilityBinding(d.Payload, d.Capability)
}

func (d *GrantDocument) UnmarshalJSON(data []byte) error {
	if d == nil {
		return jsonError(errors.New("nil retrieval grant document receiver"))
	}
	decoded, err := decodeStrict[grantDocumentWire](data, GrantDocumentJSONMaximumBytes)
	if err != nil {
		return err
	}
	candidate := GrantDocument(decoded)
	if err := candidate.Validate(); err != nil {
		return jsonError(err)
	}
	*d = candidate
	return nil
}

// GrantProjection is the issue-only bearer projection matching GrantDocument.
type GrantProjection struct {
	Capability  objectstore.DownloadCapabilityProjection
	Payload     GrantPayload
	Attestation attest.Envelope[SigningDomain]
}

type grantProjectionWire struct {
	Capability  objectstore.DownloadCapabilityProjection `json:"capability"`
	Payload     GrantPayload                             `json:"payload"`
	Attestation attest.Envelope[SigningDomain]           `json:"attestation"`
}

func (p GrantProjection) Validate() error {
	if err := errors.Join(p.Capability.Validate(), p.Payload.Validate(), p.Attestation.Validate()); err != nil {
		return contractError(err)
	}
	if p.Attestation.Domain != SigningDomainGrantV1 {
		return bindingError(errors.New("retrieval projection signing domain differs"))
	}
	return validateProjectionBinding(p.Payload, p.Capability)
}

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

type GrantIssuance struct {
	Signer     crypto.Signer
	Capability objectstore.DownloadCapabilityProjection
	Payload    GrantPayload
	Entry      chit.ManifestAddition
	Chit       chit.Verified
}

func (i GrantIssuance) Validate() error {
	if err := errors.Join(
		i.Capability.Validate(), i.Payload.Validate(), i.Chit.Validate(), i.Entry.Validate(),
	); err != nil {
		return contractError(err)
	}
	if err := validateIssuanceBinding(i); err != nil {
		return err
	}
	if err := validateProjectionBinding(i.Payload, i.Capability); err != nil {
		return err
	}
	if err := (attest.SignRequest[SigningDomain]{Body: i.Payload, Signer: i.Signer}).Validate(); err != nil {
		return contractError(err)
	}
	return nil
}

func validateIssuanceBinding(issuance GrantIssuance) error {
	document, err := issuance.Chit.Document()
	if err != nil {
		return bindingError(err)
	}
	payload := issuance.Payload
	if payload.Chit != document.Payload.Identity ||
		payload.Manifest != document.Payload.Manifest.Digest ||
		payload.Entry != issuance.Entry.Entry {
		return bindingError(errors.New("retrieval issuance differs from authenticated chit or entry"))
	}
	header := payload.Entry.Evidence.Payload.Header
	if header.Account != document.Payload.Scope.Account ||
		header.Offering != document.Payload.Scope.Offering {
		return bindingError(errors.New("retrieval entry scope differs from authenticated chit"))
	}
	return nil
}

func IssueGrant(issuance GrantIssuance) (GrantProjection, error) {
	if err := issuance.Validate(); err != nil {
		return GrantProjection{}, err
	}
	envelope, err := attest.Sign(attest.SignRequest[SigningDomain]{Body: issuance.Payload, Signer: issuance.Signer})
	if err != nil {
		return GrantProjection{}, contractError(err)
	}
	projection := GrantProjection{
		Capability: issuance.Capability, Payload: issuance.Payload, Attestation: envelope,
	}
	return projection, projection.Validate()
}

// GrantExpectation binds untrusted authority output to the exact request and
// manifest entry the caller obtained from its authenticated chit manifest.
type GrantExpectation struct {
	Document    GrantDocument
	Request     RequestPayload
	Chit        chit.Verified
	ObservedAt  temporal.Instant
	TrustedKeys attest.TrustedKeys
}

func (e GrantExpectation) Validate() error {
	if err := errors.Join(
		e.Document.Validate(), e.Request.Validate(), e.Chit.Validate(),
		e.ObservedAt.Validate(), e.TrustedKeys.Validate(),
	); err != nil {
		return contractError(err)
	}
	document, err := e.Chit.Document()
	if err != nil {
		return bindingError(err)
	}
	if document.Payload.Identity != e.Request.Chit {
		return bindingError(errors.New("retrieval request differs from authenticated chit"))
	}
	return nil
}

func validateExpectedSelection(selection Selection, sequence chit.EntrySequence) error {
	if selection.Kind == core.CatalogSelectionSpecific && selection.Sequence != sequence {
		return bindingError(errors.New("retrieval grant sequence differs from request"))
	}
	return nil
}

type VerifiedGrant struct {
	document GrantDocument
	proof    attest.Verified[SigningDomain]
}

func VerifyGrant(expectation GrantExpectation) (VerifiedGrant, error) {
	if err := expectation.Validate(); err != nil {
		return VerifiedGrant{}, err
	}
	proof, err := attest.Verify(attest.VerifyRequest[SigningDomain]{
		Body: expectation.Document.Payload, Envelope: expectation.Document.Attestation,
		TrustedKeys: expectation.TrustedKeys,
	})
	if err != nil {
		return VerifiedGrant{}, bindingError(err)
	}
	payload := expectation.Document.Payload
	if err := validateExpectedGrantBinding(expectation, payload); err != nil {
		return VerifiedGrant{}, err
	}
	if err := validateExpectedSelection(expectation.Request.Selection, payload.Entry.Sequence); err != nil {
		return VerifiedGrant{}, err
	}
	if err := validateObservedLifetime(payload, expectation.ObservedAt); err != nil {
		return VerifiedGrant{}, err
	}
	verified := VerifiedGrant{document: expectation.Document, proof: proof}
	return verified, verified.Validate()
}

func validateExpectedGrantBinding(expectation GrantExpectation, payload GrantPayload) error {
	wantRequest, requestErr := CommitRequest(expectation.Request)
	chitDocument, chitErr := expectation.Chit.Document()
	if errors.Join(requestErr, chitErr) != nil {
		return bindingError(requestErr, chitErr)
	}
	if payload.Request != wantRequest || payload.Chit != expectation.Request.Chit ||
		payload.Chit != chitDocument.Payload.Identity ||
		payload.Manifest != chitDocument.Payload.Manifest.Digest {
		return bindingError(errors.New("retrieval grant differs from its request or authenticated chit"))
	}
	return nil
}

func validateObservedLifetime(payload GrantPayload, observed temporal.Instant) error {
	issued, issuedErr := observed.Compare(payload.IssuedAt)
	expires, expiresErr := observed.Compare(payload.ExpiresAt)
	if errors.Join(issuedErr, expiresErr) != nil || issued == core.ComparisonLess || expires != core.ComparisonLess {
		return bindingError(errors.New("retrieval grant is not current"), issuedErr, expiresErr)
	}
	return nil
}

func validateProjectionBinding(payload GrantPayload, capability objectstore.DownloadCapabilityProjection) error {
	encoded, err := capability.MarshalJSON()
	if err != nil {
		return bindingError(err)
	}
	var received objectstore.DownloadCapability
	if err := json.Unmarshal(encoded, &received); err != nil {
		return bindingError(err)
	}
	return validateCapabilityBinding(payload, received)
}

func validateCapabilityBinding(payload GrantPayload, capability objectstore.DownloadCapability) error {
	commitment, err := capability.Commitment()
	if err != nil || commitment != payload.Capability {
		return bindingError(err)
	}
	target, err := capability.Target()
	if err != nil {
		return bindingError(err)
	}
	order, err := target.ExpiresAt.Compare(payload.ExpiresAt)
	if err != nil || order != core.ComparisonEqual {
		return bindingError(errors.New("retrieval capability expiry differs"), err)
	}
	return nil
}

func (v VerifiedGrant) Validate() error {
	if err := errors.Join(v.document.Validate(), v.proof.Validate()); err != nil {
		return bindingError(err)
	}
	return nil
}

func (v VerifiedGrant) Capability() (objectstore.DownloadCapability, error) {
	if err := v.Validate(); err != nil {
		return objectstore.DownloadCapability{}, err
	}
	return v.document.Capability, nil
}

func (v VerifiedGrant) Payload() (GrantPayload, error) {
	if err := v.Validate(); err != nil {
		return GrantPayload{}, err
	}
	return v.document.Payload, nil
}

var (
	_ core.Validatable                    = GrantPayload{}
	_ core.Validatable                    = GrantDocument{}
	_ core.Validatable                    = GrantProjection{}
	_ core.Validatable                    = GrantIssuance{}
	_ core.Validatable                    = GrantExpectation{}
	_ core.Validatable                    = VerifiedGrant{}
	_ core.ValidatedJSONMarshaler         = GrantPayload{}
	_ core.ValidatedJSONMarshaler         = GrantProjection{}
	_ attest.CanonicalBody[SigningDomain] = GrantPayload{}
)
