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
)

// PublicationRequestPayload asks an authority to admit one exact signed
// release manifest for immutable publication.
type PublicationRequestPayload struct {
	Manifest release.ManifestDocument `json:"manifest"`
	Build    core.BuildIdentity       `json:"build"`
	Nonce    controlwire.RequestNonce `json:"request_nonce"`
	Revision controlwire.Revision     `json:"revision"`
}

// PublicationRequestDocument carries the caller signature independently of
// the nested release-manifest signature.
type PublicationRequestDocument struct {
	Payload     PublicationRequestPayload      `json:"payload"`
	Attestation attest.Envelope[SigningDomain] `json:"attestation"`
}

// PublicationRequestIssuance supplies the release-side signing capability.
type PublicationRequestIssuance struct {
	Signer  crypto.Signer
	Payload PublicationRequestPayload
}

// PublicationRequestVerification supplies both caller and manifest trust
// authorities. Distribution decides neither set.
type PublicationRequestVerification struct {
	Document         PublicationRequestDocument
	RequestKeys      attest.TrustedKeys
	ManifestKeys     attest.TrustedKeys
	ExpectedOffering core.Offering
}

// VerifiedPublicationRequest proves both signatures and their exact offering.
type VerifiedPublicationRequest struct {
	document PublicationRequestDocument
	manifest release.VerifiedManifest
	proof    attest.Verified[SigningDomain]
}

type (
	publicationRequestPayloadWire  PublicationRequestPayload
	publicationRequestDocumentWire PublicationRequestDocument
)

func (p PublicationRequestPayload) Validate() error {
	if err := errors.Join(
		p.Manifest.Validate(), p.Build.Validate(), p.Nonce.Validate(), p.Revision.Validate(),
	); err != nil {
		return contractError(err)
	}
	if p.Revision != controlwire.Revision2026V1 {
		return contractError(errors.New("publication request revision is unsupported"))
	}
	return nil
}

func (PublicationRequestPayload) AttestationDomain() SigningDomain {
	return SigningDomainPublicationRequestV1
}

func (p PublicationRequestPayload) WriteCanonical(destination io.Writer) error {
	encoded, err := p.MarshalJSON()
	if err != nil {
		return err
	}
	return writeCanonical(destination, encoded)
}

func (p PublicationRequestPayload) MarshalJSON() ([]byte, error) {
	if err := p.Validate(); err != nil {
		return nil, jsonError(err)
	}
	encoded, err := core.MarshalCanonicalJSONDocument(publicationRequestPayloadWire(p))
	if err != nil || len(encoded) > requestPayloadJSONMaximumBytes {
		return nil, jsonError(err)
	}
	return encoded, nil
}

func (p *PublicationRequestPayload) UnmarshalJSON(data []byte) error {
	if p == nil {
		return jsonError(errors.New("publication request payload receiver is nil"))
	}
	wire, err := decodeStrict[publicationRequestPayloadWire](data, requestPayloadJSONMaximumBytes)
	if err != nil {
		return err
	}
	candidate := PublicationRequestPayload(wire)
	if err := candidate.Validate(); err != nil {
		return jsonError(err)
	}
	*p = candidate
	return nil
}

func (d PublicationRequestDocument) Validate() error {
	if err := errors.Join(d.Payload.Validate(), d.Attestation.Validate()); err != nil {
		return contractError(err)
	}
	if d.Attestation.Domain != d.Payload.AttestationDomain() {
		return bindingError(errors.New("publication request attestation domain differs"))
	}
	return nil
}

func (d PublicationRequestDocument) MarshalJSON() ([]byte, error) {
	if err := d.Validate(); err != nil {
		return nil, jsonError(err)
	}
	encoded, err := core.MarshalCanonicalJSONDocument(publicationRequestDocumentWire(d))
	if err != nil || len(encoded) > RequestDocumentJSONMaximumBytes {
		return nil, jsonError(err)
	}
	return encoded, nil
}

func (d *PublicationRequestDocument) UnmarshalJSON(data []byte) error {
	if d == nil {
		return jsonError(errors.New("publication request document receiver is nil"))
	}
	wire, err := decodeStrict[publicationRequestDocumentWire](data, RequestDocumentJSONMaximumBytes)
	if err != nil {
		return err
	}
	candidate := PublicationRequestDocument(wire)
	if err := candidate.Validate(); err != nil {
		return jsonError(err)
	}
	*d = candidate
	return nil
}

func (i PublicationRequestIssuance) Validate() error {
	if err := (attest.SignRequest[SigningDomain]{Body: i.Payload, Signer: i.Signer}).Validate(); err != nil {
		return contractError(err)
	}
	return nil
}

// IssuePublicationRequest signs one exact manifest request.
func IssuePublicationRequest(issuance PublicationRequestIssuance) (PublicationRequestDocument, error) {
	if err := issuance.Validate(); err != nil {
		return PublicationRequestDocument{}, err
	}
	envelope, err := attest.Sign(attest.SignRequest[SigningDomain]{
		Body: issuance.Payload, Signer: issuance.Signer,
	})
	if err != nil {
		return PublicationRequestDocument{}, contractError(err)
	}
	document := PublicationRequestDocument{Payload: issuance.Payload, Attestation: envelope}
	return document, document.Validate()
}

func (v PublicationRequestVerification) Validate() error {
	if err := errors.Join(
		v.Document.Validate(), v.RequestKeys.Validate(),
		v.ManifestKeys.Validate(), v.ExpectedOffering.Validate(),
	); err != nil {
		return verificationError(err)
	}
	return nil
}

// VerifyPublicationRequest authenticates the request and its nested release
// manifest before returning either fact.
func VerifyPublicationRequest(verification PublicationRequestVerification) (VerifiedPublicationRequest, error) {
	if err := verification.Validate(); err != nil {
		return VerifiedPublicationRequest{}, err
	}
	proof, err := attest.Verify(attest.VerifyRequest[SigningDomain]{
		Body: verification.Document.Payload, Envelope: verification.Document.Attestation,
		TrustedKeys: verification.RequestKeys,
	})
	if err != nil {
		return VerifiedPublicationRequest{}, verificationError(err)
	}
	manifest, err := release.VerifyManifest(release.VerifyManifestRequest{
		Document:         verification.Document.Payload.Manifest,
		TrustedKeys:      verification.ManifestKeys,
		ExpectedOffering: verification.ExpectedOffering,
	})
	if err != nil {
		return VerifiedPublicationRequest{}, verificationError(err)
	}
	verified := VerifiedPublicationRequest{
		document: verification.Document, manifest: manifest, proof: proof,
	}
	return verified, verified.Validate()
}

func (v VerifiedPublicationRequest) Validate() error {
	if err := errors.Join(v.document.Validate(), v.manifest.Validate(), v.proof.Validate()); err != nil {
		return verificationError(err)
	}
	if v.document.Payload.Manifest != v.manifest.Document() {
		return bindingError(errors.New("verified publication manifest differs from request"))
	}
	return nil
}

func (v VerifiedPublicationRequest) Payload() (PublicationRequestPayload, error) {
	if err := v.Validate(); err != nil {
		return PublicationRequestPayload{}, err
	}
	return v.document.Payload, nil
}

func (v VerifiedPublicationRequest) Manifest() (release.VerifiedManifest, error) {
	if err := v.Validate(); err != nil {
		return release.VerifiedManifest{}, err
	}
	return v.manifest, nil
}

var (
	_ core.Validatable = PublicationRequestPayload{}
	_ core.Validatable = PublicationRequestDocument{}
	_ core.Validatable = PublicationRequestIssuance{}
	_ core.Validatable = PublicationRequestVerification{}
	_ core.Validatable = VerifiedPublicationRequest{}

	_ core.ValidatedJSONMarshaler         = PublicationRequestPayload{}
	_ core.ValidatedJSONMarshaler         = PublicationRequestDocument{}
	_ json.Unmarshaler                    = (*PublicationRequestPayload)(nil)
	_ json.Unmarshaler                    = (*PublicationRequestDocument)(nil)
	_ attest.CanonicalBody[SigningDomain] = PublicationRequestPayload{}
)
