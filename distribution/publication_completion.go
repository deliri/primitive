package distribution

import (
	"crypto"
	"encoding/json"
	"errors"
	"io"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/deploy"
	"github.com/deliri/primitive/v2026/objectstore"
	"github.com/deliri/primitive/v2026/release"
)

// PublicationCompletionPayload is the authority-received proof that every
// object in one granted release publication completed with exact integrity.
type PublicationCompletionPayload struct {
	Evidence         [release.PublicationObjectCount]objectstore.TransferEvidence `json:"evidence"`
	Request          RequestCommitment                                            `json:"request_commitment"`
	Authorization    controlwire.AuthorityNonce                                   `json:"authorization_nonce"`
	Manifest         release.ManifestIdentity                                     `json:"manifest"`
	ManifestDocument release.ManifestDocumentDigest                               `json:"manifest_document"`
	Build            core.BuildIdentity                                           `json:"build"`
}

// PublicationCompletionDocument carries the caller's signature over provider
// evidence that contains no bearer, URL, path, or object bytes.
type PublicationCompletionDocument struct {
	Payload     PublicationCompletionPayload   `json:"payload"`
	Attestation attest.Envelope[SigningDomain] `json:"attestation"`
}

// PublicationCompletionProjection is the issue-only form because Objectstore
// transfer evidence itself has separate issue and receive types.
type PublicationCompletionProjection struct {
	payload     publicationCompletionProjectionPayload
	attestation attest.Envelope[SigningDomain]
}

// PublicationCompletionIssuance closes only a complete Deploy result against
// the authenticated request and grant that authorized it.
type PublicationCompletionIssuance struct {
	Signer   crypto.Signer
	Receipts deploy.Receipts
	Grant    VerifiedPublicationGrant
	Request  VerifiedPublicationRequest
}

// PublicationCompletionExpectation supplies the exact authenticated request,
// signed grant facts, and caller trust used by an authority on receipt.
type PublicationCompletionExpectation struct {
	Document         PublicationCompletionDocument
	Request          VerifiedPublicationRequest
	GrantKeys        attest.TrustedKeys
	CompletionKeys   attest.TrustedKeys
	Grant            PublicationGrantPayload
	GrantAttestation attest.Envelope[SigningDomain]
}

// VerifiedPublicationCompletion is the authenticated, fully bound provider
// evidence set safe for authority persistence and Latest advancement policy.
type VerifiedPublicationCompletion struct {
	document   PublicationCompletionDocument
	proof      attest.Verified[SigningDomain]
	grantProof attest.Verified[SigningDomain]
}

type publicationCompletionProjectionPayload struct {
	evidence         [release.PublicationObjectCount]objectstore.TransferEvidenceProjection
	request          RequestCommitment
	authorization    controlwire.AuthorityNonce
	manifest         release.ManifestIdentity
	manifestDocument release.ManifestDocumentDigest
	build            core.BuildIdentity
}

type (
	publicationCompletionPayloadWire           PublicationCompletionPayload
	publicationCompletionDocumentWire          PublicationCompletionDocument
	publicationCompletionProjectionPayloadWire struct {
		Evidence         [release.PublicationObjectCount]objectstore.TransferEvidenceProjection `json:"evidence"`
		Request          RequestCommitment                                                      `json:"request_commitment"`
		Authorization    controlwire.AuthorityNonce                                             `json:"authorization_nonce"`
		Manifest         release.ManifestIdentity                                               `json:"manifest"`
		ManifestDocument release.ManifestDocumentDigest                                         `json:"manifest_document"`
		Build            core.BuildIdentity                                                     `json:"build"`
	}
	publicationCompletionProjectionWire struct {
		Payload     publicationCompletionProjectionPayloadWire `json:"payload"`
		Attestation attest.Envelope[SigningDomain]             `json:"attestation"`
	}
)

func (p PublicationCompletionPayload) Validate() error {
	if err := errors.Join(
		p.Request.validateDomain(SigningDomainPublicationRequestV1),
		p.Authorization.Validate(), p.Manifest.Validate(), p.ManifestDocument.Validate(), p.Build.Validate(),
	); err != nil {
		return contractError(err)
	}
	for _, evidence := range p.Evidence {
		if err := evidence.Validate(); err != nil {
			return contractError(err)
		}
	}
	return nil
}

func (PublicationCompletionPayload) AttestationDomain() SigningDomain {
	return SigningDomainPublicationCompletionV1
}

func (p PublicationCompletionPayload) WriteCanonical(destination io.Writer) error {
	encoded, err := p.MarshalJSON()
	if err != nil {
		return err
	}
	return writeCanonical(destination, encoded)
}

func (p PublicationCompletionPayload) MarshalJSON() ([]byte, error) {
	if err := p.Validate(); err != nil {
		return nil, jsonError(err)
	}
	encoded, err := core.MarshalCanonicalJSONDocument(publicationCompletionPayloadWire(p))
	if err != nil || len(encoded) > publicationCompletionMaximumBytes {
		return nil, jsonError(err)
	}
	return encoded, nil
}

func (p *PublicationCompletionPayload) UnmarshalJSON(data []byte) error {
	if p == nil {
		return jsonError(errors.New("publication completion payload receiver is nil"))
	}
	wire, err := decodeStrict[publicationCompletionPayloadWire](data, publicationCompletionMaximumBytes)
	if err != nil {
		return err
	}
	candidate := PublicationCompletionPayload(wire)
	if err := candidate.Validate(); err != nil {
		return jsonError(err)
	}
	*p = candidate
	return nil
}

func (d PublicationCompletionDocument) Validate() error {
	if err := errors.Join(d.Payload.Validate(), d.Attestation.Validate()); err != nil {
		return contractError(err)
	}
	if d.Attestation.Domain != d.Payload.AttestationDomain() {
		return bindingError(errors.New("publication completion attestation domain differs"))
	}
	return nil
}

func (d PublicationCompletionDocument) MarshalJSON() ([]byte, error) {
	if err := d.Validate(); err != nil {
		return nil, jsonError(err)
	}
	encoded, err := core.MarshalCanonicalJSONDocument(publicationCompletionDocumentWire(d))
	if err != nil || len(encoded) > ResponseDocumentJSONMaximumBytes {
		return nil, jsonError(err)
	}
	return encoded, nil
}

func (d *PublicationCompletionDocument) UnmarshalJSON(data []byte) error {
	if d == nil {
		return jsonError(errors.New("publication completion document receiver is nil"))
	}
	wire, err := decodeStrict[publicationCompletionDocumentWire](data, ResponseDocumentJSONMaximumBytes)
	if err != nil {
		return err
	}
	candidate := PublicationCompletionDocument(wire)
	if err := candidate.Validate(); err != nil {
		return jsonError(err)
	}
	*d = candidate
	return nil
}

func (p publicationCompletionProjectionPayload) Validate() error {
	if err := errors.Join(
		p.request.validateDomain(SigningDomainPublicationRequestV1),
		p.authorization.Validate(), p.manifest.Validate(), p.manifestDocument.Validate(), p.build.Validate(),
	); err != nil {
		return contractError(err)
	}
	for _, evidence := range p.evidence {
		if err := evidence.Validate(); err != nil {
			return contractError(err)
		}
	}
	return nil
}

func (publicationCompletionProjectionPayload) AttestationDomain() SigningDomain {
	return SigningDomainPublicationCompletionV1
}

func (p publicationCompletionProjectionPayload) wire() publicationCompletionProjectionPayloadWire {
	return publicationCompletionProjectionPayloadWire{
		Request: p.request, Authorization: p.authorization,
		Manifest: p.manifest, ManifestDocument: p.manifestDocument, Build: p.build,
		Evidence: p.evidence,
	}
}

func (p publicationCompletionProjectionPayload) WriteCanonical(destination io.Writer) error {
	encoded, err := p.MarshalJSON()
	if err != nil {
		return err
	}
	return writeCanonical(destination, encoded)
}

func (p publicationCompletionProjectionPayload) MarshalJSON() ([]byte, error) {
	if err := p.Validate(); err != nil {
		return nil, jsonError(err)
	}
	encoded, err := core.MarshalCanonicalJSONDocument(p.wire())
	if err != nil || len(encoded) > publicationCompletionMaximumBytes {
		return nil, jsonError(err)
	}
	return encoded, nil
}

func (p PublicationCompletionProjection) Validate() error {
	if err := errors.Join(p.payload.Validate(), p.attestation.Validate()); err != nil {
		return contractError(err)
	}
	if p.attestation.Domain != p.payload.AttestationDomain() {
		return bindingError(errors.New("publication completion projection domain differs"))
	}
	return nil
}

func (p PublicationCompletionProjection) MarshalJSON() ([]byte, error) {
	if err := p.Validate(); err != nil {
		return nil, jsonError(err)
	}
	encoded, err := core.MarshalCanonicalJSONDocument(publicationCompletionProjectionWire{
		Payload: p.payload.wire(), Attestation: p.attestation,
	})
	if err != nil || len(encoded) > ResponseDocumentJSONMaximumBytes {
		return nil, jsonError(err)
	}
	return encoded, nil
}

func (p PublicationCompletionProjection) Build() (core.BuildIdentity, error) {
	if err := p.Validate(); err != nil {
		return core.BuildIdentity{}, err
	}
	return p.payload.build, nil
}

func publicationCompletionPayload(issuance PublicationCompletionIssuance) (publicationCompletionProjectionPayload, error) {
	if err := errors.Join(
		issuance.Request.Validate(), issuance.Grant.Validate(), issuance.Receipts.Validate(),
	); err != nil {
		return publicationCompletionProjectionPayload{}, contractError(err)
	}
	grant, manifest, err := publicationCompletionOwners(issuance)
	if err != nil {
		return publicationCompletionProjectionPayload{}, err
	}
	request, err := issuance.Request.Payload()
	if err != nil {
		return publicationCompletionProjectionPayload{}, err
	}
	payload := publicationCompletionProjectionPayload{
		request: grant.Request, authorization: grant.Authorization,
		manifest: manifest.Identity(), manifestDocument: manifest.DocumentDigest(), build: request.Build,
	}
	payload.evidence, err = publicationCompletionEvidence(issuance.Receipts, grant)
	if err != nil {
		return publicationCompletionProjectionPayload{}, err
	}
	return payload, payload.Validate()
}

func publicationCompletionOwners(
	issuance PublicationCompletionIssuance,
) (PublicationGrantPayload, release.VerifiedManifest, error) {
	request, err := issuance.Request.Payload()
	if err != nil {
		return PublicationGrantPayload{}, release.VerifiedManifest{}, err
	}
	grantedRequest, err := issuance.Grant.Request()
	if err != nil || grantedRequest != request {
		return PublicationGrantPayload{}, release.VerifiedManifest{}, bindingError(errors.New("completion request differs from grant"), err)
	}
	grant, err := issuance.Grant.Payload()
	if err != nil {
		return PublicationGrantPayload{}, release.VerifiedManifest{}, err
	}
	manifest, err := issuance.Request.Manifest()
	if err != nil {
		return PublicationGrantPayload{}, release.VerifiedManifest{}, err
	}
	if issuance.Receipts.Count() != release.PublicationObjectCount {
		return PublicationGrantPayload{}, release.VerifiedManifest{}, contractError(errors.New("publication completion is partial"))
	}
	return grant, manifest, nil
}

func publicationCompletionEvidence(
	receipts deploy.Receipts,
	grant PublicationGrantPayload,
) ([release.PublicationObjectCount]objectstore.TransferEvidenceProjection, error) {
	var evidenceSet [release.PublicationObjectCount]objectstore.TransferEvidenceProjection
	for index := range evidenceSet {
		receipt, ok := receipts.At(index)
		if !ok || receipt.Commitment() != grant.Commitments[index] {
			return evidenceSet, bindingError(errors.New("publication receipt differs from grant"))
		}
		evidence, err := receipt.Transfer().Evidence()
		if err != nil {
			return evidenceSet, contractError(err)
		}
		evidenceSet[index] = evidence
	}
	return evidenceSet, nil
}

func (i PublicationCompletionIssuance) Validate() error {
	payload, err := publicationCompletionPayload(i)
	if err != nil {
		return err
	}
	if err := (attest.SignRequest[SigningDomain]{Body: payload, Signer: i.Signer}).Validate(); err != nil {
		return contractError(err)
	}
	return nil
}

// IssuePublicationCompletion signs only a complete exact receipt set.
func IssuePublicationCompletion(issuance PublicationCompletionIssuance) (PublicationCompletionProjection, error) {
	if err := issuance.Validate(); err != nil {
		return PublicationCompletionProjection{}, err
	}
	payload, err := publicationCompletionPayload(issuance)
	if err != nil {
		return PublicationCompletionProjection{}, err
	}
	envelope, err := attest.Sign(attest.SignRequest[SigningDomain]{
		Body: payload, Signer: issuance.Signer,
	})
	if err != nil {
		return PublicationCompletionProjection{}, contractError(err)
	}
	projection := PublicationCompletionProjection{payload: payload, attestation: envelope}
	return projection, projection.Validate()
}

func (e PublicationCompletionExpectation) Validate() error {
	if err := errors.Join(
		e.Request.Validate(), e.Grant.Validate(), e.GrantAttestation.Validate(),
		e.Document.Validate(), e.GrantKeys.Validate(), e.CompletionKeys.Validate(),
	); err != nil {
		return verificationError(err)
	}
	if e.GrantAttestation.Domain != e.Grant.AttestationDomain() {
		return bindingError(errors.New("completion grant attestation domain differs"))
	}
	return nil
}

func validatePublicationCompletionBinding(expectation PublicationCompletionExpectation) error {
	manifest, err := validatePublicationCompletionIdentity(expectation)
	if err != nil {
		return err
	}
	return validatePublicationCompletionEvidenceSet(manifest, expectation.Document.Payload.Evidence)
}

func validatePublicationCompletionIdentity(
	expectation PublicationCompletionExpectation,
) (release.VerifiedManifest, error) {
	request, err := expectation.Request.Payload()
	if err != nil {
		return release.VerifiedManifest{}, err
	}
	commitment, err := CommitRequest(request)
	if err != nil || commitment != expectation.Grant.Request ||
		commitment != expectation.Document.Payload.Request ||
		expectation.Grant.Authorization != expectation.Document.Payload.Authorization {
		return release.VerifiedManifest{}, bindingError(errors.New("publication completion request or authorization differs"), err)
	}
	manifest, err := expectation.Request.Manifest()
	if err != nil {
		return release.VerifiedManifest{}, err
	}
	if manifest.Identity() != expectation.Document.Payload.Manifest ||
		manifest.DocumentDigest() != expectation.Document.Payload.ManifestDocument ||
		request.Build != expectation.Document.Payload.Build {
		return release.VerifiedManifest{}, bindingError(errors.New("publication completion manifest differs"))
	}
	return manifest, nil
}

func validatePublicationCompletionEvidenceSet(
	manifest release.VerifiedManifest,
	evidenceSet [release.PublicationObjectCount]objectstore.TransferEvidence,
) error {
	for index, evidence := range evidenceSet {
		role, ok := release.PublicationRoleAt(index)
		if !ok {
			return contractError(errors.New("publication completion role slot is invalid"))
		}
		integrity, integrityErr := manifest.PublicationIntegrity(role)
		if integrityErr != nil {
			return contractError(integrityErr)
		}
		if err := validatePublicationEvidence(evidence, integrity); err != nil {
			return err
		}
	}
	return nil
}

func validatePublicationEvidence(
	evidence objectstore.TransferEvidence,
	integrity release.ArtifactIntegrity,
) error {
	extent, err := integrity.Extent().Uint64()
	if err != nil {
		return contractError(err)
	}
	length, err := core.NewByteLength(extent)
	if err != nil {
		return contractError(err)
	}
	if evidence.Provider() != objectstore.ProviderGoogleCloudStorage ||
		evidence.Direction() != objectstore.DirectionUpload ||
		evidence.Bytes() != length || evidence.SHA256() != integrity.SHA256() ||
		evidence.CRC32C() != integrity.CRC32C() {
		return bindingError(errors.New("publication completion evidence differs from manifest"))
	}
	return nil
}

// VerifyPublicationCompletion authenticates both the original grant and the
// completion, then binds every provider fact to the signed manifest.
func VerifyPublicationCompletion(expectation PublicationCompletionExpectation) (VerifiedPublicationCompletion, error) {
	if err := expectation.Validate(); err != nil {
		return VerifiedPublicationCompletion{}, err
	}
	grantProof, err := attest.Verify(attest.VerifyRequest[SigningDomain]{
		Body: expectation.Grant, Envelope: expectation.GrantAttestation,
		TrustedKeys: expectation.GrantKeys,
	})
	if err != nil {
		return VerifiedPublicationCompletion{}, verificationError(err)
	}
	proof, err := attest.Verify(attest.VerifyRequest[SigningDomain]{
		Body: expectation.Document.Payload, Envelope: expectation.Document.Attestation,
		TrustedKeys: expectation.CompletionKeys,
	})
	if err != nil {
		return VerifiedPublicationCompletion{}, verificationError(err)
	}
	if err := validatePublicationCompletionBinding(expectation); err != nil {
		return VerifiedPublicationCompletion{}, err
	}
	verified := VerifiedPublicationCompletion{
		document: expectation.Document, proof: proof, grantProof: grantProof,
	}
	return verified, verified.Validate()
}

func (v VerifiedPublicationCompletion) Validate() error {
	if err := errors.Join(v.document.Validate(), v.proof.Validate(), v.grantProof.Validate()); err != nil {
		return verificationError(err)
	}
	return nil
}

func (v VerifiedPublicationCompletion) Payload() (PublicationCompletionPayload, error) {
	if err := v.Validate(); err != nil {
		return PublicationCompletionPayload{}, err
	}
	return v.document.Payload, nil
}

var (
	_ core.Validatable = PublicationCompletionPayload{}
	_ core.Validatable = PublicationCompletionDocument{}
	_ core.Validatable = PublicationCompletionProjection{}
	_ core.Validatable = PublicationCompletionIssuance{}
	_ core.Validatable = PublicationCompletionExpectation{}
	_ core.Validatable = VerifiedPublicationCompletion{}

	_ core.ValidatedJSONMarshaler         = PublicationCompletionPayload{}
	_ core.ValidatedJSONMarshaler         = PublicationCompletionDocument{}
	_ core.ValidatedJSONMarshaler         = PublicationCompletionProjection{}
	_ core.ValidatedJSONMarshaler         = publicationCompletionProjectionPayload{}
	_ json.Unmarshaler                    = (*PublicationCompletionPayload)(nil)
	_ json.Unmarshaler                    = (*PublicationCompletionDocument)(nil)
	_ attest.CanonicalBody[SigningDomain] = PublicationCompletionPayload{}
	_ attest.CanonicalBody[SigningDomain] = publicationCompletionProjectionPayload{}
)
