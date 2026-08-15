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
)

const (
	// CompletionPayloadJSONMaximumBytes bounds one provider-evidence completion.
	CompletionPayloadJSONMaximumBytes = 64 << 10
	// CompletionDocumentJSONMaximumBytes bounds the signed completion document.
	CompletionDocumentJSONMaximumBytes = 96 << 10
)

// CompletionPayload is the receive-side statement that one exact granted
// upload completed. Evidence contains no bearer, URL, path, or object bytes.
type CompletionPayload struct {
	Evidence      objectstore.TransferEvidence           `json:"evidence"`
	Build         core.BuildIdentity                     `json:"build"`
	Nonce         controlwire.RequestNonce               `json:"request_nonce"`
	Request       RequestCommitment                      `json:"request_commitment"`
	Capability    objectstore.UploadCapabilityCommitment `json:"capability_commitment"`
	Authorization controlwire.AuthorityNonce             `json:"authorization_nonce"`
}

// CompletionDocument carries the installed device's signature over one exact
// provider result and the grant facts that authorized it.
type CompletionDocument struct {
	Payload     CompletionPayload              `json:"payload"`
	Attestation attest.Envelope[SigningDomain] `json:"attestation"`
}

// CompletionProjection is the issue-only form. Objectstore transfer evidence
// remains issue-only until this explicit external projection is encoded.
type CompletionProjection struct {
	payload     completionProjectionPayload
	attestation attest.Envelope[SigningDomain]
}

// CompletionIssuance binds an actual confirmed transfer to the authenticated
// request and grant that produced its bearer.
type CompletionIssuance struct {
	Signer   crypto.Signer
	Transfer objectstore.Transfer
	Request  RequestPayload
	Grant    VerifiedGrant
}

// CompletionExpectation supplies the original request, exact signed grant,
// and the two independently selected trust sets used by an authority.
type CompletionExpectation struct {
	Request        RequestPayload
	Grant          GrantDocument
	Document       CompletionDocument
	GrantKeys      attest.TrustedKeys
	CompletionKeys attest.TrustedKeys
}

// VerifiedCompletion is the authenticated exact provider evidence safe for an
// authority to reconcile against its own object and custody records.
type VerifiedCompletion struct {
	document   CompletionDocument
	grantProof attest.Verified[SigningDomain]
	proof      attest.Verified[SigningDomain]
}

type completionProjectionPayload struct {
	evidence      objectstore.TransferEvidenceProjection
	build         core.BuildIdentity
	nonce         controlwire.RequestNonce
	request       RequestCommitment
	capability    objectstore.UploadCapabilityCommitment
	authorization controlwire.AuthorityNonce
}

type (
	completionPayloadWire           CompletionPayload
	completionDocumentWire          CompletionDocument
	completionProjectionPayloadWire struct {
		Evidence      objectstore.TransferEvidenceProjection `json:"evidence"`
		Build         core.BuildIdentity                     `json:"build"`
		Nonce         controlwire.RequestNonce               `json:"request_nonce"`
		Request       RequestCommitment                      `json:"request_commitment"`
		Capability    objectstore.UploadCapabilityCommitment `json:"capability_commitment"`
		Authorization controlwire.AuthorityNonce             `json:"authorization_nonce"`
	}
	completionProjectionWire struct {
		Payload     completionProjectionPayloadWire `json:"payload"`
		Attestation attest.Envelope[SigningDomain]  `json:"attestation"`
	}
)

func (p CompletionPayload) Validate() error {
	if err := errors.Join(
		p.Build.Validate(), p.Nonce.Validate(), p.Request.Validate(), p.Capability.Validate(),
		p.Authorization.Validate(), p.Evidence.Validate(),
	); err != nil {
		return contractError(err)
	}
	if p.Evidence.Direction() != objectstore.DirectionUpload {
		return bindingError(errors.New("submission completion evidence is not an upload"))
	}
	return nil
}

func (CompletionPayload) AttestationDomain() SigningDomain {
	return SigningDomainCompletionV1
}

func (p CompletionPayload) WriteCanonical(destination io.Writer) error {
	encoded, err := p.MarshalJSON()
	if err != nil {
		return err
	}
	return writeCanonicalPayload(destination, encoded)
}

func (p CompletionPayload) MarshalJSON() ([]byte, error) {
	if err := p.Validate(); err != nil {
		return nil, jsonError(err)
	}
	encoded, err := core.MarshalCanonicalJSONDocument(completionPayloadWire(p))
	if err != nil || len(encoded) > CompletionPayloadJSONMaximumBytes {
		return nil, jsonError(err)
	}
	return encoded, nil
}

func (p *CompletionPayload) UnmarshalJSON(data []byte) error {
	if p == nil {
		return jsonError(errors.New("nil submission completion payload receiver"))
	}
	wire, err := decodeStrict[completionPayloadWire](data, CompletionPayloadJSONMaximumBytes)
	if err != nil {
		return err
	}
	candidate := CompletionPayload(wire)
	if err := candidate.Validate(); err != nil {
		return jsonError(err)
	}
	*p = candidate
	return nil
}

func (d CompletionDocument) Validate() error {
	if err := errors.Join(d.Payload.Validate(), d.Attestation.Validate()); err != nil {
		return contractError(err)
	}
	if d.Attestation.Domain != d.Payload.AttestationDomain() {
		return bindingError(errors.New("submission completion attestation domain differs"))
	}
	return nil
}

func (d CompletionDocument) MarshalJSON() ([]byte, error) {
	if err := d.Validate(); err != nil {
		return nil, jsonError(err)
	}
	encoded, err := core.MarshalCanonicalJSONDocument(completionDocumentWire(d))
	if err != nil || len(encoded) > CompletionDocumentJSONMaximumBytes {
		return nil, jsonError(err)
	}
	return encoded, nil
}

func (d *CompletionDocument) UnmarshalJSON(data []byte) error {
	if d == nil {
		return jsonError(errors.New("nil submission completion document receiver"))
	}
	wire, err := decodeStrict[completionDocumentWire](data, CompletionDocumentJSONMaximumBytes)
	if err != nil {
		return err
	}
	candidate := CompletionDocument(wire)
	if err := candidate.Validate(); err != nil {
		return jsonError(err)
	}
	*d = candidate
	return nil
}

func (p completionProjectionPayload) Validate() error {
	if err := errors.Join(
		p.build.Validate(), p.nonce.Validate(), p.request.Validate(), p.capability.Validate(),
		p.authorization.Validate(), p.evidence.Validate(),
	); err != nil {
		return contractError(err)
	}
	return nil
}

func (completionProjectionPayload) AttestationDomain() SigningDomain {
	return SigningDomainCompletionV1
}

func (p completionProjectionPayload) wire() completionProjectionPayloadWire {
	return completionProjectionPayloadWire{
		Build: p.build, Nonce: p.nonce, Request: p.request, Capability: p.capability,
		Authorization: p.authorization, Evidence: p.evidence,
	}
}

func (p completionProjectionPayload) WriteCanonical(destination io.Writer) error {
	encoded, err := p.MarshalJSON()
	if err != nil {
		return err
	}
	return writeCanonicalPayload(destination, encoded)
}

func (p completionProjectionPayload) MarshalJSON() ([]byte, error) {
	if err := p.Validate(); err != nil {
		return nil, jsonError(err)
	}
	encoded, err := core.MarshalCanonicalJSONDocument(p.wire())
	if err != nil || len(encoded) > CompletionPayloadJSONMaximumBytes {
		return nil, jsonError(err)
	}
	return encoded, nil
}

func (p CompletionProjection) Validate() error {
	if err := errors.Join(p.payload.Validate(), p.attestation.Validate()); err != nil {
		return contractError(err)
	}
	if p.attestation.Domain != p.payload.AttestationDomain() {
		return bindingError(errors.New("submission completion projection domain differs"))
	}
	return nil
}

// Build returns the signed installed-build fact so an outer credential
// envelope can bind the projection without decoding its own wire output.
func (p CompletionProjection) Build() (core.BuildIdentity, error) {
	if err := p.Validate(); err != nil {
		return core.BuildIdentity{}, err
	}
	return p.payload.build, nil
}

func (p CompletionProjection) MarshalJSON() ([]byte, error) {
	if err := p.Validate(); err != nil {
		return nil, jsonError(err)
	}
	encoded, err := core.MarshalCanonicalJSONDocument(completionProjectionWire{
		Payload: p.payload.wire(), Attestation: p.attestation,
	})
	if err != nil || len(encoded) > CompletionDocumentJSONMaximumBytes {
		return nil, jsonError(err)
	}
	return encoded, nil
}

func (p CompletionProjection) ValidateJSONProjection(encoded []byte, limits core.StrictJSONLimits) error {
	return core.ValidateReceiveOnlyJSONProjection[CompletionProjection, CompletionDocument, *CompletionDocument](
		p, encoded, limits,
	)
}

func completionProjection(issuance CompletionIssuance) (completionProjectionPayload, error) {
	if err := errors.Join(issuance.Request.Validate(), issuance.Grant.Validate(), issuance.Transfer.Validate()); err != nil {
		return completionProjectionPayload{}, contractError(err)
	}
	grant, err := issuance.Grant.Payload()
	if err != nil {
		return completionProjectionPayload{}, contractError(err)
	}
	request, err := CommitRequest(issuance.Request)
	if err != nil || request != grant.Request {
		return completionProjectionPayload{}, bindingError(errors.New("completion request differs from grant"), err)
	}
	capability, err := issuance.Grant.Capability()
	if err != nil {
		return completionProjectionPayload{}, contractError(err)
	}
	provider, err := capability.Provider()
	if err != nil || provider != issuance.Transfer.Provider() {
		return completionProjectionPayload{}, bindingError(errors.New("completion provider differs from grant"), err)
	}
	if err := validateCompletionIntegrity(issuance.Transfer, issuance.Request.Declaration); err != nil {
		return completionProjectionPayload{}, err
	}
	evidence, err := issuance.Transfer.Evidence()
	if err != nil {
		return completionProjectionPayload{}, contractError(err)
	}
	payload := completionProjectionPayload{
		build: issuance.Request.Build, nonce: issuance.Request.Nonce,
		request: request, capability: grant.Capability,
		authorization: grant.Authorization, evidence: evidence,
	}
	return payload, payload.Validate()
}

func (i CompletionIssuance) Validate() error {
	payload, err := completionProjection(i)
	if err != nil {
		return err
	}
	if err := (attest.SignRequest[SigningDomain]{Body: payload, Signer: i.Signer}).Validate(); err != nil {
		return contractError(err)
	}
	return nil
}

// IssueCompletion signs one exact confirmed provider result against its grant.
func IssueCompletion(issuance CompletionIssuance) (CompletionProjection, error) {
	if err := issuance.Validate(); err != nil {
		return CompletionProjection{}, err
	}
	payload, err := completionProjection(issuance)
	if err != nil {
		return CompletionProjection{}, err
	}
	envelope, err := attest.Sign(attest.SignRequest[SigningDomain]{Body: payload, Signer: issuance.Signer})
	if err != nil {
		return CompletionProjection{}, contractError(err)
	}
	projection := CompletionProjection{payload: payload, attestation: envelope}
	return projection, projection.Validate()
}

func (e CompletionExpectation) Validate() error {
	if err := errors.Join(
		e.Document.Validate(), e.Request.Validate(), e.Grant.Validate(),
		e.GrantKeys.Validate(), e.CompletionKeys.Validate(),
	); err != nil {
		return contractError(err)
	}
	return nil
}

func validateCompletionBinding(expectation CompletionExpectation) error {
	request, err := CommitRequest(expectation.Request)
	if err != nil || request != expectation.Grant.Payload.Request ||
		request != expectation.Document.Payload.Request {
		return bindingError(errors.New("completion request commitment differs"), err)
	}
	payload := expectation.Document.Payload
	grant := expectation.Grant.Payload
	if payload.Build != expectation.Request.Build || payload.Nonce != expectation.Request.Nonce ||
		payload.Authorization != grant.Authorization ||
		payload.Capability != grant.Capability {
		return bindingError(errors.New("completion grant facts differ"))
	}
	provider, err := expectation.Grant.Capability.Provider()
	if err != nil || provider != payload.Evidence.Provider() {
		return bindingError(errors.New("completion evidence provider differs"), err)
	}
	return validateCompletionEvidence(payload.Evidence, expectation.Request.Declaration)
}

func validateCompletionIntegrity(transfer objectstore.Transfer, declaration Declaration) error {
	if transfer.Direction() != objectstore.DirectionUpload || transfer.Bytes() != declaration.Extent ||
		transfer.SHA256() != declaration.SHA256 || transfer.CRC32C() != declaration.CRC32C {
		return bindingError(errors.New("completion transfer differs from declaration"))
	}
	return nil
}

func validateCompletionEvidence(evidence objectstore.TransferEvidence, declaration Declaration) error {
	if evidence.Direction() != objectstore.DirectionUpload || evidence.Bytes() != declaration.Extent ||
		evidence.SHA256() != declaration.SHA256 || evidence.CRC32C() != declaration.CRC32C {
		return bindingError(errors.New("completion evidence differs from declaration"))
	}
	return nil
}

// VerifyCompletion authenticates the grant and device completion, then binds
// the provider evidence to the exact request and capability commitment.
func VerifyCompletion(expectation CompletionExpectation) (VerifiedCompletion, error) {
	if err := expectation.Validate(); err != nil {
		return VerifiedCompletion{}, err
	}
	if err := validateCompletionBinding(expectation); err != nil {
		return VerifiedCompletion{}, err
	}
	grantProof, err := attest.Verify(attest.VerifyRequest[SigningDomain]{
		Body: expectation.Grant.Payload, Envelope: expectation.Grant.Attestation,
		TrustedKeys: expectation.GrantKeys,
	})
	if err != nil {
		return VerifiedCompletion{}, contractError(err)
	}
	proof, err := attest.Verify(attest.VerifyRequest[SigningDomain]{
		Body: expectation.Document.Payload, Envelope: expectation.Document.Attestation,
		TrustedKeys: expectation.CompletionKeys,
	})
	if err != nil {
		return VerifiedCompletion{}, contractError(err)
	}
	verified := VerifiedCompletion{document: expectation.Document, grantProof: grantProof, proof: proof}
	return verified, verified.Validate()
}

func (v VerifiedCompletion) Validate() error {
	if err := errors.Join(v.document.Validate(), v.grantProof.Validate(), v.proof.Validate()); err != nil {
		return contractError(err)
	}
	return nil
}

func (v VerifiedCompletion) Payload() (CompletionPayload, error) {
	if err := v.Validate(); err != nil {
		return CompletionPayload{}, err
	}
	return v.document.Payload, nil
}

var (
	_ core.Validatable = CompletionPayload{}
	_ core.Validatable = CompletionDocument{}
	_ core.Validatable = CompletionProjection{}
	_ core.Validatable = CompletionIssuance{}
	_ core.Validatable = CompletionExpectation{}
	_ core.Validatable = VerifiedCompletion{}

	_ core.ValidatedJSONMarshaler         = CompletionPayload{}
	_ core.ValidatedJSONMarshaler         = CompletionDocument{}
	_ core.ValidatedJSONMarshaler         = CompletionProjection{}
	_ core.ValidatedJSONMarshaler         = completionProjectionPayload{}
	_ core.ValidatedJSONProjection        = CompletionProjection{}
	_ json.Unmarshaler                    = (*CompletionPayload)(nil)
	_ json.Unmarshaler                    = (*CompletionDocument)(nil)
	_ attest.CanonicalBody[SigningDomain] = CompletionPayload{}
	_ attest.CanonicalBody[SigningDomain] = completionProjectionPayload{}
)
