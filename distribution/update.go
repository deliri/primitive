package distribution

import (
	"crypto"
	"encoding/json"
	"errors"
	"io"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/release"
	"github.com/deliri/primitive/v2026/temporal"
)

// UpdateRequestPayload identifies the exact installed build asking for the
// authority's current signed Latest document.
type UpdateRequestPayload struct {
	Build    core.BuildIdentity       `json:"build"`
	Nonce    controlwire.RequestNonce `json:"request_nonce"`
	Revision controlwire.Revision     `json:"revision"`
}

type UpdateRequestDocument struct {
	Payload     UpdateRequestPayload           `json:"payload"`
	Attestation attest.Envelope[SigningDomain] `json:"attestation"`
}

type UpdateRequestIssuance struct {
	Signer  crypto.Signer
	Payload UpdateRequestPayload
}

type UpdateRequestVerification struct {
	Document    UpdateRequestDocument
	TrustedKeys attest.TrustedKeys
}

type VerifiedUpdateRequest struct {
	document UpdateRequestDocument
	proof    attest.Verified[SigningDomain]
}

// UpdateResponsePayload binds the authenticated installed manifest and signed
// Latest document to the exact request and a short response lifetime.
type UpdateResponsePayload struct {
	Installed release.ManifestDocument `json:"installed"`
	Latest    release.LatestDocument   `json:"latest"`
	IssuedAt  temporal.Instant         `json:"issued_at"`
	ExpiresAt temporal.Instant         `json:"expires_at"`
	Request   RequestCommitment        `json:"request_commitment"`
}

type UpdateResponseDocument struct {
	Payload     UpdateResponsePayload          `json:"payload"`
	Attestation attest.Envelope[SigningDomain] `json:"attestation"`
}

type UpdateResponseIssuance struct {
	Signer  crypto.Signer
	Payload UpdateResponsePayload
}

// UpdateResponseVerification carries each independent trust set explicitly:
// response authority, Latest authority, and release-manifest authority.
type UpdateResponseVerification struct {
	Document         UpdateResponseDocument
	ResponseKeys     attest.TrustedKeys
	LatestKeys       attest.TrustedKeys
	ManifestKeys     attest.TrustedKeys
	ObservedAt       temporal.Instant
	Request          UpdateRequestPayload
	ExpectedOffering core.Offering
}

// VerifiedUpdateResponse contains both authenticated proofs consumed by
// Release assessment and selection.
type VerifiedUpdateResponse struct {
	installed release.VerifiedManifest
	latest    release.VerifiedLatest
	document  UpdateResponseDocument
	proof     attest.Verified[SigningDomain]
	request   UpdateRequestPayload
}

type (
	updateRequestPayloadWire   UpdateRequestPayload
	updateRequestDocumentWire  UpdateRequestDocument
	updateResponsePayloadWire  UpdateResponsePayload
	updateResponseDocumentWire UpdateResponseDocument
)

func (p UpdateRequestPayload) Validate() error {
	if err := errors.Join(p.Build.Validate(), p.Nonce.Validate(), p.Revision.Validate()); err != nil {
		return contractError(err)
	}
	if p.Revision != controlwire.Revision2026V1 {
		return contractError(errors.New("update request revision is unsupported"))
	}
	return nil
}

func (UpdateRequestPayload) AttestationDomain() SigningDomain {
	return SigningDomainUpdateRequestV1
}

func (p UpdateRequestPayload) WriteCanonical(destination io.Writer) error {
	encoded, err := p.MarshalJSON()
	if err != nil {
		return err
	}
	return writeCanonical(destination, encoded)
}

func (p UpdateRequestPayload) MarshalJSON() ([]byte, error) {
	if err := p.Validate(); err != nil {
		return nil, jsonError(err)
	}
	encoded, err := core.MarshalCanonicalJSONDocument(updateRequestPayloadWire(p))
	if err != nil || len(encoded) > requestPayloadJSONMaximumBytes {
		return nil, jsonError(err)
	}
	return encoded, nil
}

func (p *UpdateRequestPayload) UnmarshalJSON(data []byte) error {
	if p == nil {
		return jsonError(errors.New("update request payload receiver is nil"))
	}
	wire, err := decodeStrict[updateRequestPayloadWire](data, requestPayloadJSONMaximumBytes)
	if err != nil {
		return err
	}
	candidate := UpdateRequestPayload(wire)
	if err := candidate.Validate(); err != nil {
		return jsonError(err)
	}
	*p = candidate
	return nil
}

func (d UpdateRequestDocument) Validate() error {
	if err := errors.Join(d.Payload.Validate(), d.Attestation.Validate()); err != nil {
		return contractError(err)
	}
	if d.Attestation.Domain != d.Payload.AttestationDomain() {
		return bindingError(errors.New("update request attestation domain differs"))
	}
	return nil
}

func (d UpdateRequestDocument) MarshalJSON() ([]byte, error) {
	if err := d.Validate(); err != nil {
		return nil, jsonError(err)
	}
	encoded, err := core.MarshalCanonicalJSONDocument(updateRequestDocumentWire(d))
	if err != nil || len(encoded) > RequestDocumentJSONMaximumBytes {
		return nil, jsonError(err)
	}
	return encoded, nil
}

func (d *UpdateRequestDocument) UnmarshalJSON(data []byte) error {
	if d == nil {
		return jsonError(errors.New("update request document receiver is nil"))
	}
	wire, err := decodeStrict[updateRequestDocumentWire](data, RequestDocumentJSONMaximumBytes)
	if err != nil {
		return err
	}
	candidate := UpdateRequestDocument(wire)
	if err := candidate.Validate(); err != nil {
		return jsonError(err)
	}
	*d = candidate
	return nil
}

func (i UpdateRequestIssuance) Validate() error {
	if err := (attest.SignRequest[SigningDomain]{Body: i.Payload, Signer: i.Signer}).Validate(); err != nil {
		return contractError(err)
	}
	return nil
}

func IssueUpdateRequest(issuance UpdateRequestIssuance) (UpdateRequestDocument, error) {
	if err := issuance.Validate(); err != nil {
		return UpdateRequestDocument{}, err
	}
	envelope, err := attest.Sign(attest.SignRequest[SigningDomain]{
		Body: issuance.Payload, Signer: issuance.Signer,
	})
	if err != nil {
		return UpdateRequestDocument{}, contractError(err)
	}
	document := UpdateRequestDocument{Payload: issuance.Payload, Attestation: envelope}
	return document, document.Validate()
}

func (v UpdateRequestVerification) Validate() error {
	if err := errors.Join(v.Document.Validate(), v.TrustedKeys.Validate()); err != nil {
		return verificationError(err)
	}
	return nil
}

func VerifyUpdateRequest(verification UpdateRequestVerification) (VerifiedUpdateRequest, error) {
	if err := verification.Validate(); err != nil {
		return VerifiedUpdateRequest{}, err
	}
	proof, err := attest.Verify(attest.VerifyRequest[SigningDomain]{
		Body: verification.Document.Payload, Envelope: verification.Document.Attestation,
		TrustedKeys: verification.TrustedKeys,
	})
	if err != nil {
		return VerifiedUpdateRequest{}, verificationError(err)
	}
	verified := VerifiedUpdateRequest{document: verification.Document, proof: proof}
	return verified, verified.Validate()
}

func (v VerifiedUpdateRequest) Validate() error {
	if err := errors.Join(v.document.Validate(), v.proof.Validate()); err != nil {
		return verificationError(err)
	}
	return nil
}

func (v VerifiedUpdateRequest) Payload() (UpdateRequestPayload, error) {
	if err := v.Validate(); err != nil {
		return UpdateRequestPayload{}, err
	}
	return v.document.Payload, nil
}

func (p UpdateResponsePayload) Validate() error {
	if err := errors.Join(
		p.Request.validateDomain(SigningDomainUpdateRequestV1),
		p.Installed.Validate(), p.Latest.Validate(), validateLifetime(p.IssuedAt, p.ExpiresAt),
	); err != nil {
		return contractError(err)
	}
	return nil
}

func (UpdateResponsePayload) AttestationDomain() SigningDomain {
	return SigningDomainUpdateResponseV1
}

func (p UpdateResponsePayload) WriteCanonical(destination io.Writer) error {
	encoded, err := p.MarshalJSON()
	if err != nil {
		return err
	}
	return writeCanonical(destination, encoded)
}

func (p UpdateResponsePayload) MarshalJSON() ([]byte, error) {
	if err := p.Validate(); err != nil {
		return nil, jsonError(err)
	}
	encoded, err := core.MarshalCanonicalJSONDocument(updateResponsePayloadWire(p))
	if err != nil || len(encoded) > responsePayloadJSONMaximumBytes {
		return nil, jsonError(err)
	}
	return encoded, nil
}

func (p *UpdateResponsePayload) UnmarshalJSON(data []byte) error {
	if p == nil {
		return jsonError(errors.New("update response payload receiver is nil"))
	}
	wire, err := decodeStrict[updateResponsePayloadWire](data, responsePayloadJSONMaximumBytes)
	if err != nil {
		return err
	}
	candidate := UpdateResponsePayload(wire)
	if err := candidate.Validate(); err != nil {
		return jsonError(err)
	}
	*p = candidate
	return nil
}

func (d UpdateResponseDocument) Validate() error {
	if err := errors.Join(d.Payload.Validate(), d.Attestation.Validate()); err != nil {
		return contractError(err)
	}
	if d.Attestation.Domain != d.Payload.AttestationDomain() {
		return bindingError(errors.New("update response attestation domain differs"))
	}
	return nil
}

func (d UpdateResponseDocument) MarshalJSON() ([]byte, error) {
	if err := d.Validate(); err != nil {
		return nil, jsonError(err)
	}
	encoded, err := core.MarshalCanonicalJSONDocument(updateResponseDocumentWire(d))
	if err != nil || len(encoded) > responseDocumentJSONMaximumBytes {
		return nil, jsonError(err)
	}
	return encoded, nil
}

func (d *UpdateResponseDocument) UnmarshalJSON(data []byte) error {
	if d == nil {
		return jsonError(errors.New("update response document receiver is nil"))
	}
	wire, err := decodeStrict[updateResponseDocumentWire](data, responseDocumentJSONMaximumBytes)
	if err != nil {
		return err
	}
	candidate := UpdateResponseDocument(wire)
	if err := candidate.Validate(); err != nil {
		return jsonError(err)
	}
	*d = candidate
	return nil
}

func (i UpdateResponseIssuance) Validate() error {
	if err := (attest.SignRequest[SigningDomain]{Body: i.Payload, Signer: i.Signer}).Validate(); err != nil {
		return contractError(err)
	}
	return nil
}

func IssueUpdateResponse(issuance UpdateResponseIssuance) (UpdateResponseDocument, error) {
	if err := issuance.Validate(); err != nil {
		return UpdateResponseDocument{}, err
	}
	envelope, err := attest.Sign(attest.SignRequest[SigningDomain]{
		Body: issuance.Payload, Signer: issuance.Signer,
	})
	if err != nil {
		return UpdateResponseDocument{}, contractError(err)
	}
	document := UpdateResponseDocument{Payload: issuance.Payload, Attestation: envelope}
	return document, document.Validate()
}

func (v UpdateResponseVerification) Validate() error {
	if err := errors.Join(
		v.Request.Validate(), v.Document.Validate(), v.ResponseKeys.Validate(),
		v.LatestKeys.Validate(), v.ManifestKeys.Validate(),
		v.ExpectedOffering.Validate(), v.ObservedAt.Validate(),
	); err != nil {
		return verificationError(err)
	}
	return nil
}

func VerifyUpdateResponse(verification UpdateResponseVerification) (VerifiedUpdateResponse, error) {
	if err := verification.Validate(); err != nil {
		return VerifiedUpdateResponse{}, err
	}
	request, err := CommitRequest(verification.Request)
	if err != nil || request != verification.Document.Payload.Request {
		return VerifiedUpdateResponse{}, bindingError(errors.New("update response answers another request"), err)
	}
	if err := validateObservedLifetime(
		verification.Document.Payload.IssuedAt,
		verification.Document.Payload.ExpiresAt,
		verification.ObservedAt,
	); err != nil {
		return VerifiedUpdateResponse{}, err
	}
	proof, err := attest.Verify(attest.VerifyRequest[SigningDomain]{
		Body: verification.Document.Payload, Envelope: verification.Document.Attestation,
		TrustedKeys: verification.ResponseKeys,
	})
	if err != nil {
		return VerifiedUpdateResponse{}, verificationError(err)
	}
	latest, err := release.VerifyLatest(release.VerifyLatestRequest{
		Document:   verification.Document.Payload.Latest,
		LatestKeys: verification.LatestKeys, ManifestKeys: verification.ManifestKeys,
		ExpectedOffering: verification.ExpectedOffering,
	})
	if err != nil {
		return VerifiedUpdateResponse{}, verificationError(err)
	}
	if latest.Manifest().Offering() != verification.Request.Build.Offering() {
		return VerifiedUpdateResponse{}, bindingError(errors.New("update response offering differs from installed build"))
	}
	installed, err := release.VerifyManifest(release.VerifyManifestRequest{
		Document:    verification.Document.Payload.Installed,
		TrustedKeys: verification.ManifestKeys, ExpectedOffering: verification.ExpectedOffering,
	})
	if err != nil {
		return VerifiedUpdateResponse{}, verificationError(err)
	}
	if err := bindInstalledManifest(installed, verification.Request.Build); err != nil {
		return VerifiedUpdateResponse{}, err
	}
	verified := VerifiedUpdateResponse{
		document: verification.Document, request: verification.Request,
		installed: installed, latest: latest, proof: proof,
	}
	return verified, verified.Validate()
}

func (v VerifiedUpdateResponse) Validate() error {
	if err := errors.Join(
		v.document.Validate(), v.request.Validate(), v.installed.Validate(),
		v.latest.Validate(), v.proof.Validate(),
	); err != nil {
		return verificationError(err)
	}
	request, err := CommitRequest(v.request)
	if err != nil || request != v.document.Payload.Request ||
		v.installed.Document() != v.document.Payload.Installed ||
		v.latest.Document() != v.document.Payload.Latest {
		return bindingError(errors.New("verified update response binding differs"), err)
	}
	if err := bindInstalledManifest(v.installed, v.request.Build); err != nil {
		return err
	}
	return nil
}

func bindInstalledManifest(installed release.VerifiedManifest, build core.BuildIdentity) error {
	if installed.Offering() != build.Offering() || installed.Version() != build.Version() {
		return bindingError(errors.New("installed manifest differs from requested build"))
	}
	artifact, ok := installed.Artifacts().ForPlatform(build.Platform())
	if !ok || artifact.Build() != build {
		return bindingError(errors.New("installed manifest lacks requested build artifact"))
	}
	return nil
}

// Installed returns the authenticated manifest containing the exact build
// named by the request. It supplies Release evaluation's installed proof.
func (v VerifiedUpdateResponse) Installed() (release.VerifiedManifest, error) {
	if err := v.Validate(); err != nil {
		return release.VerifiedManifest{}, err
	}
	return v.installed, nil
}

func (v VerifiedUpdateResponse) Latest() (release.VerifiedLatest, error) {
	if err := v.Validate(); err != nil {
		return release.VerifiedLatest{}, err
	}
	return v.latest, nil
}

var (
	_ core.Validatable = UpdateRequestPayload{}
	_ core.Validatable = UpdateRequestDocument{}
	_ core.Validatable = UpdateRequestIssuance{}
	_ core.Validatable = UpdateRequestVerification{}
	_ core.Validatable = VerifiedUpdateRequest{}
	_ core.Validatable = UpdateResponsePayload{}
	_ core.Validatable = UpdateResponseDocument{}
	_ core.Validatable = UpdateResponseIssuance{}
	_ core.Validatable = UpdateResponseVerification{}
	_ core.Validatable = VerifiedUpdateResponse{}

	_ core.ValidatedJSONMarshaler         = UpdateRequestPayload{}
	_ core.ValidatedJSONMarshaler         = UpdateRequestDocument{}
	_ core.ValidatedJSONMarshaler         = UpdateResponsePayload{}
	_ core.ValidatedJSONMarshaler         = UpdateResponseDocument{}
	_ json.Unmarshaler                    = (*UpdateRequestPayload)(nil)
	_ json.Unmarshaler                    = (*UpdateRequestDocument)(nil)
	_ json.Unmarshaler                    = (*UpdateResponsePayload)(nil)
	_ json.Unmarshaler                    = (*UpdateResponseDocument)(nil)
	_ attest.CanonicalBody[SigningDomain] = UpdateRequestPayload{}
	_ attest.CanonicalBody[SigningDomain] = UpdateResponsePayload{}
)
