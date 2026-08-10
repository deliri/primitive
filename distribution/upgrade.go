package distribution

import (
	"crypto"
	"encoding/json"
	"errors"
	"io"
	"os"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/objectstore"
	"github.com/deliri/primitive/v2026/release"
	"github.com/deliri/primitive/v2026/temporal"
	"github.com/deliri/primitive/v2026/upgrade"
)

// UpgradeRequestPayload asks for one exact candidate artifact already selected
// from an authenticated Latest document.
type UpgradeRequestPayload struct {
	Available release.AvailableSummary `json:"available"`
	Nonce     controlwire.RequestNonce `json:"request_nonce"`
	Revision  controlwire.Revision     `json:"revision"`
}

type UpgradeRequestDocument struct {
	Payload     UpgradeRequestPayload          `json:"payload"`
	Attestation attest.Envelope[SigningDomain] `json:"attestation"`
}

type UpgradeRequestIssuance struct {
	Signer  crypto.Signer
	Payload UpgradeRequestPayload
}

type UpgradeRequestVerification struct {
	Document    UpgradeRequestDocument
	TrustedKeys attest.TrustedKeys
}

type VerifiedUpgradeRequest struct {
	document UpgradeRequestDocument
	proof    attest.Verified[SigningDomain]
}

// UpgradeGrantPayload signs only the non-secret closure of one exact download
// bearer beside its request and lifetime.
type UpgradeGrantPayload struct {
	Request       RequestCommitment                        `json:"request_commitment"`
	Authorization controlwire.AuthorityNonce               `json:"authorization_nonce"`
	Capability    objectstore.DownloadCapabilityCommitment `json:"capability_commitment"`
	IssuedAt      temporal.Instant                         `json:"issued_at"`
	ExpiresAt     temporal.Instant                         `json:"expires_at"`
}

type UpgradeGrantDocument struct {
	Capability  objectstore.DownloadCapability `json:"capability"`
	Payload     UpgradeGrantPayload            `json:"payload"`
	Attestation attest.Envelope[SigningDomain] `json:"attestation"`
}

type UpgradeGrantProjection struct {
	Capability  objectstore.DownloadCapabilityProjection
	Payload     UpgradeGrantPayload
	Attestation attest.Envelope[SigningDomain]
}

type UpgradeGrantIssuance struct {
	Signer     crypto.Signer
	Capability objectstore.DownloadCapabilityProjection
	Payload    UpgradeGrantPayload
}

type UpgradeGrantExpectation struct {
	Document    UpgradeGrantDocument
	TrustedKeys attest.TrustedKeys
	Request     UpgradeRequestPayload
	ObservedAt  temporal.Instant
}

type VerifiedUpgradeGrant struct {
	document UpgradeGrantDocument
	request  UpgradeRequestPayload
	proof    attest.Verified[SigningDomain]
}

// UpgradeStageRequest supplies only the local effect capabilities that cannot
// cross the wire. The candidate and download bearer remain grant-bound.
type UpgradeStageRequest struct {
	Client    objectstore.Client
	Observer  objectstore.ProgressObserver
	Root      *os.Root
	Directory core.AbsolutePath
	Grant     VerifiedUpgradeGrant
	Prepared  release.PreparedRelease
	Policy    objectstore.Policy
}

type (
	upgradeRequestPayloadWire  UpgradeRequestPayload
	upgradeRequestDocumentWire UpgradeRequestDocument
	upgradeGrantPayloadWire    UpgradeGrantPayload
	upgradeGrantDocumentWire   struct {
		Capability  objectstore.DownloadCapability `json:"capability"`
		Payload     UpgradeGrantPayload            `json:"payload"`
		Attestation attest.Envelope[SigningDomain] `json:"attestation"`
	}
	upgradeGrantProjectionWire struct {
		Capability  objectstore.DownloadCapabilityProjection `json:"capability"`
		Payload     UpgradeGrantPayload                      `json:"payload"`
		Attestation attest.Envelope[SigningDomain]           `json:"attestation"`
	}
)

func (p UpgradeRequestPayload) Validate() error {
	if err := errors.Join(p.Available.Validate(), p.Nonce.Validate(), p.Revision.Validate()); err != nil {
		return contractError(err)
	}
	if p.Revision != controlwire.Revision2026V1 {
		return contractError(errors.New("upgrade request revision is unsupported"))
	}
	return nil
}

func (UpgradeRequestPayload) AttestationDomain() SigningDomain {
	return SigningDomainUpgradeRequestV1
}

func (p UpgradeRequestPayload) WriteCanonical(destination io.Writer) error {
	encoded, err := p.MarshalJSON()
	if err != nil {
		return err
	}
	return writeCanonical(destination, encoded)
}

func (p UpgradeRequestPayload) MarshalJSON() ([]byte, error) {
	if err := p.Validate(); err != nil {
		return nil, jsonError(err)
	}
	encoded, err := core.MarshalCanonicalJSONDocument(upgradeRequestPayloadWire(p))
	if err != nil || len(encoded) > requestPayloadJSONMaximumBytes {
		return nil, jsonError(err)
	}
	return encoded, nil
}

func (p *UpgradeRequestPayload) UnmarshalJSON(data []byte) error {
	if p == nil {
		return jsonError(errors.New("upgrade request payload receiver is nil"))
	}
	wire, err := decodeStrict[upgradeRequestPayloadWire](data, requestPayloadJSONMaximumBytes)
	if err != nil {
		return err
	}
	candidate := UpgradeRequestPayload(wire)
	if err := candidate.Validate(); err != nil {
		return jsonError(err)
	}
	*p = candidate
	return nil
}

func (d UpgradeRequestDocument) Validate() error {
	if err := errors.Join(d.Payload.Validate(), d.Attestation.Validate()); err != nil {
		return contractError(err)
	}
	if d.Attestation.Domain != d.Payload.AttestationDomain() {
		return bindingError(errors.New("upgrade request attestation domain differs"))
	}
	return nil
}

func (d UpgradeRequestDocument) MarshalJSON() ([]byte, error) {
	if err := d.Validate(); err != nil {
		return nil, jsonError(err)
	}
	encoded, err := core.MarshalCanonicalJSONDocument(upgradeRequestDocumentWire(d))
	if err != nil || len(encoded) > RequestDocumentJSONMaximumBytes {
		return nil, jsonError(err)
	}
	return encoded, nil
}

func (d *UpgradeRequestDocument) UnmarshalJSON(data []byte) error {
	if d == nil {
		return jsonError(errors.New("upgrade request document receiver is nil"))
	}
	wire, err := decodeStrict[upgradeRequestDocumentWire](data, RequestDocumentJSONMaximumBytes)
	if err != nil {
		return err
	}
	candidate := UpgradeRequestDocument(wire)
	if err := candidate.Validate(); err != nil {
		return jsonError(err)
	}
	*d = candidate
	return nil
}

func (i UpgradeRequestIssuance) Validate() error {
	if err := (attest.SignRequest[SigningDomain]{Body: i.Payload, Signer: i.Signer}).Validate(); err != nil {
		return contractError(err)
	}
	return nil
}

func IssueUpgradeRequest(issuance UpgradeRequestIssuance) (UpgradeRequestDocument, error) {
	if err := issuance.Validate(); err != nil {
		return UpgradeRequestDocument{}, err
	}
	envelope, err := attest.Sign(attest.SignRequest[SigningDomain]{
		Body: issuance.Payload, Signer: issuance.Signer,
	})
	if err != nil {
		return UpgradeRequestDocument{}, contractError(err)
	}
	document := UpgradeRequestDocument{Payload: issuance.Payload, Attestation: envelope}
	return document, document.Validate()
}

func (v UpgradeRequestVerification) Validate() error {
	if err := errors.Join(v.Document.Validate(), v.TrustedKeys.Validate()); err != nil {
		return verificationError(err)
	}
	return nil
}

func VerifyUpgradeRequest(verification UpgradeRequestVerification) (VerifiedUpgradeRequest, error) {
	if err := verification.Validate(); err != nil {
		return VerifiedUpgradeRequest{}, err
	}
	proof, err := attest.Verify(attest.VerifyRequest[SigningDomain]{
		Body: verification.Document.Payload, Envelope: verification.Document.Attestation,
		TrustedKeys: verification.TrustedKeys,
	})
	if err != nil {
		return VerifiedUpgradeRequest{}, verificationError(err)
	}
	verified := VerifiedUpgradeRequest{document: verification.Document, proof: proof}
	return verified, verified.Validate()
}

func (v VerifiedUpgradeRequest) Validate() error {
	if err := errors.Join(v.document.Validate(), v.proof.Validate()); err != nil {
		return verificationError(err)
	}
	return nil
}

func (v VerifiedUpgradeRequest) Payload() (UpgradeRequestPayload, error) {
	if err := v.Validate(); err != nil {
		return UpgradeRequestPayload{}, err
	}
	return v.document.Payload, nil
}

func (p UpgradeGrantPayload) Validate() error {
	if err := errors.Join(
		p.Request.validateDomain(SigningDomainUpgradeRequestV1),
		p.Authorization.Validate(), p.Capability.Validate(),
		validateLifetime(p.IssuedAt, p.ExpiresAt),
	); err != nil {
		return contractError(err)
	}
	return nil
}

func (UpgradeGrantPayload) AttestationDomain() SigningDomain {
	return SigningDomainUpgradeGrantV1
}

func (p UpgradeGrantPayload) WriteCanonical(destination io.Writer) error {
	encoded, err := p.MarshalJSON()
	if err != nil {
		return err
	}
	return writeCanonical(destination, encoded)
}

func (p UpgradeGrantPayload) MarshalJSON() ([]byte, error) {
	if err := p.Validate(); err != nil {
		return nil, jsonError(err)
	}
	encoded, err := core.MarshalCanonicalJSONDocument(upgradeGrantPayloadWire(p))
	if err != nil || len(encoded) > responsePayloadJSONMaximumBytes {
		return nil, jsonError(err)
	}
	return encoded, nil
}

func (p *UpgradeGrantPayload) UnmarshalJSON(data []byte) error {
	if p == nil {
		return jsonError(errors.New("upgrade grant payload receiver is nil"))
	}
	wire, err := decodeStrict[upgradeGrantPayloadWire](data, responsePayloadJSONMaximumBytes)
	if err != nil {
		return err
	}
	candidate := UpgradeGrantPayload(wire)
	if err := candidate.Validate(); err != nil {
		return jsonError(err)
	}
	*p = candidate
	return nil
}

func validateUpgradeCapability(capability objectstore.DownloadCapability, payload UpgradeGrantPayload) error {
	if err := errors.Join(capability.Validate(), payload.Validate()); err != nil {
		return contractError(err)
	}
	commitment, err := capability.Commitment()
	if err != nil || commitment != payload.Capability {
		return bindingError(errors.New("upgrade bearer differs from signed commitment"), err)
	}
	return nil
}

func validateUpgradeProjection(
	capability objectstore.DownloadCapabilityProjection,
	payload UpgradeGrantPayload,
) error {
	if err := errors.Join(capability.Validate(), payload.Validate()); err != nil {
		return contractError(err)
	}
	commitment, err := capability.Commitment()
	if err != nil || commitment != payload.Capability {
		return bindingError(errors.New("upgrade projection differs from signed commitment"), err)
	}
	return nil
}

func (d UpgradeGrantDocument) Validate() error {
	if err := errors.Join(
		validateUpgradeCapability(d.Capability, d.Payload), d.Attestation.Validate(),
	); err != nil {
		return contractError(err)
	}
	if d.Attestation.Domain != d.Payload.AttestationDomain() {
		return bindingError(errors.New("upgrade grant attestation domain differs"))
	}
	return nil
}

func (d *UpgradeGrantDocument) UnmarshalJSON(data []byte) error {
	if d == nil {
		return jsonError(errors.New("upgrade grant document receiver is nil"))
	}
	wire, err := decodeStrict[upgradeGrantDocumentWire](data, responseDocumentJSONMaximumBytes)
	if err != nil {
		return err
	}
	candidate := UpgradeGrantDocument(wire)
	if err := candidate.Validate(); err != nil {
		return jsonError(err)
	}
	*d = candidate
	return nil
}

func (p UpgradeGrantProjection) Validate() error {
	if err := errors.Join(
		validateUpgradeProjection(p.Capability, p.Payload), p.Attestation.Validate(),
	); err != nil {
		return contractError(err)
	}
	if p.Attestation.Domain != p.Payload.AttestationDomain() {
		return bindingError(errors.New("upgrade grant projection domain differs"))
	}
	return nil
}

func (p UpgradeGrantProjection) MarshalJSON() ([]byte, error) {
	if err := p.Validate(); err != nil {
		return nil, jsonError(err)
	}
	encoded, err := core.MarshalCanonicalJSONDocument(upgradeGrantProjectionWire(p))
	if err != nil || len(encoded) > responseDocumentJSONMaximumBytes {
		return nil, jsonError(err)
	}
	return encoded, nil
}

func (i UpgradeGrantIssuance) Validate() error {
	if err := validateUpgradeProjection(i.Capability, i.Payload); err != nil {
		return err
	}
	if err := (attest.SignRequest[SigningDomain]{Body: i.Payload, Signer: i.Signer}).Validate(); err != nil {
		return contractError(err)
	}
	return nil
}

func IssueUpgradeGrant(issuance UpgradeGrantIssuance) (UpgradeGrantProjection, error) {
	if err := issuance.Validate(); err != nil {
		return UpgradeGrantProjection{}, err
	}
	envelope, err := attest.Sign(attest.SignRequest[SigningDomain]{
		Body: issuance.Payload, Signer: issuance.Signer,
	})
	if err != nil {
		return UpgradeGrantProjection{}, contractError(err)
	}
	projection := UpgradeGrantProjection{
		Capability: issuance.Capability, Payload: issuance.Payload, Attestation: envelope,
	}
	return projection, projection.Validate()
}

func (e UpgradeGrantExpectation) Validate() error {
	if err := errors.Join(
		e.Request.Validate(), e.Document.Validate(),
		e.TrustedKeys.Validate(), e.ObservedAt.Validate(),
	); err != nil {
		return verificationError(err)
	}
	return nil
}

func VerifyUpgradeGrant(expectation UpgradeGrantExpectation) (VerifiedUpgradeGrant, error) {
	if err := expectation.Validate(); err != nil {
		return VerifiedUpgradeGrant{}, err
	}
	request, err := CommitRequest(expectation.Request)
	if err != nil || request != expectation.Document.Payload.Request {
		return VerifiedUpgradeGrant{}, bindingError(errors.New("upgrade grant answers another request"), err)
	}
	if err := validateObservedLifetime(
		expectation.Document.Payload.IssuedAt,
		expectation.Document.Payload.ExpiresAt,
		expectation.ObservedAt,
	); err != nil {
		return VerifiedUpgradeGrant{}, err
	}
	proof, err := attest.Verify(attest.VerifyRequest[SigningDomain]{
		Body: expectation.Document.Payload, Envelope: expectation.Document.Attestation,
		TrustedKeys: expectation.TrustedKeys,
	})
	if err != nil {
		return VerifiedUpgradeGrant{}, verificationError(err)
	}
	verified := VerifiedUpgradeGrant{
		document: expectation.Document, request: expectation.Request, proof: proof,
	}
	return verified, verified.Validate()
}

func (v VerifiedUpgradeGrant) Validate() error {
	if err := errors.Join(v.document.Validate(), v.request.Validate(), v.proof.Validate()); err != nil {
		return verificationError(err)
	}
	request, err := CommitRequest(v.request)
	if err != nil || request != v.document.Payload.Request {
		return bindingError(errors.New("verified upgrade request differs"), err)
	}
	return nil
}

func (v VerifiedUpgradeGrant) Request() (UpgradeRequestPayload, error) {
	if err := v.Validate(); err != nil {
		return UpgradeRequestPayload{}, err
	}
	return v.request, nil
}

func (r UpgradeStageRequest) Validate() error {
	if err := errors.Join(
		r.Grant.Validate(), r.Prepared.Validate(), r.Client.Validate(), r.Policy.Validate(),
		r.Directory.Validate(),
	); err != nil {
		return contractError(err)
	}
	if r.Root == nil {
		return contractError(errors.New("upgrade stage root is nil"))
	}
	request, err := r.Grant.Request()
	if err != nil {
		return err
	}
	summary, err := r.Prepared.Summary()
	if err != nil || summary != request.Available {
		return bindingError(errors.New("upgrade stage candidate differs from grant"), err)
	}
	return nil
}

// PrepareUpgradeStage projects the authenticated download bearer directly
// into Upgrade's crash-recoverable local staging request.
func PrepareUpgradeStage(request UpgradeStageRequest) (upgrade.StageRequest, error) {
	if err := request.Validate(); err != nil {
		return upgrade.StageRequest{}, err
	}
	source := upgrade.DownloadSource{
		Client: request.Client, Observer: request.Observer,
		Capability: request.Grant.document.Capability,
		Commitment: request.Grant.document.Payload.Capability,
		Policy:     request.Policy,
	}
	stage := upgrade.StageRequest{
		Root: request.Root, Directory: request.Directory,
		Source: source, Prepared: request.Prepared,
	}
	return stage, stage.Validate()
}

var (
	_ core.Validatable = UpgradeRequestPayload{}
	_ core.Validatable = UpgradeRequestDocument{}
	_ core.Validatable = UpgradeRequestIssuance{}
	_ core.Validatable = UpgradeRequestVerification{}
	_ core.Validatable = VerifiedUpgradeRequest{}
	_ core.Validatable = UpgradeGrantPayload{}
	_ core.Validatable = UpgradeGrantDocument{}
	_ core.Validatable = UpgradeGrantProjection{}
	_ core.Validatable = UpgradeGrantIssuance{}
	_ core.Validatable = UpgradeGrantExpectation{}
	_ core.Validatable = VerifiedUpgradeGrant{}
	_ core.Validatable = UpgradeStageRequest{}

	_ core.ValidatedJSONMarshaler         = UpgradeRequestPayload{}
	_ core.ValidatedJSONMarshaler         = UpgradeRequestDocument{}
	_ core.ValidatedJSONMarshaler         = UpgradeGrantPayload{}
	_ core.ValidatedJSONMarshaler         = UpgradeGrantProjection{}
	_ json.Unmarshaler                    = (*UpgradeRequestPayload)(nil)
	_ json.Unmarshaler                    = (*UpgradeRequestDocument)(nil)
	_ json.Unmarshaler                    = (*UpgradeGrantPayload)(nil)
	_ json.Unmarshaler                    = (*UpgradeGrantDocument)(nil)
	_ attest.CanonicalBody[SigningDomain] = UpgradeRequestPayload{}
	_ attest.CanonicalBody[SigningDomain] = UpgradeGrantPayload{}
)
