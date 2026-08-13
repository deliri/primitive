package distributionauth

import (
	"encoding/json"
	"errors"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/controlplane"
	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/distribution"
	"github.com/deliri/primitive/v2026/release"
)

const (
	publicationCompletionDocumentSyntaxBytes      = len(`{"completion":,"certificate":}`)
	PublicationCompletionDocumentJSONMaximumBytes = distribution.ResponseDocumentJSONMaximumBytes +
		controlplane.InstallationCertificateDocumentJSONMaximumBytes +
		publicationCompletionDocumentSyntaxBytes + core.CredentialedDocumentWhitespaceMaximumBytes
)

type PublicationRequestDocument struct {
	Request     distribution.PublicationRequestDocument      `json:"request"`
	Certificate controlplane.InstallationCertificateDocument `json:"certificate"`
}

type PublicationRequestAssembly struct {
	Request     distribution.PublicationRequestDocument
	Certificate controlplane.InstallationCertificateDocument
}

type PublicationVerification struct {
	Document     PublicationRequestDocument
	TrustedKeys  attest.TrustedKeys
	ManifestKeys attest.TrustedKeys
}

type VerifiedPublication struct {
	document         PublicationRequestDocument
	requestProof     distribution.VerifiedPublicationRequest
	certificateProof controlplane.VerifiedInstallationCertificate
}

type publicationRequestDocumentWire PublicationRequestDocument

func (d PublicationRequestDocument) Validate() error {
	if err := errors.Join(d.Request.Validate(), d.Certificate.Validate()); err != nil {
		return contractError(err)
	}
	if d.Request.Payload.Build != d.Certificate.Body.Build {
		return bindingError()
	}
	return nil
}

// ControlRoute projects the sole route admitted by this publication request.
func (d PublicationRequestDocument) ControlRoute() (controlwire.RouteContract, error) {
	return controlwire.NewRouteContract(
		d.Request.Payload.Build.Offering(), controlwire.RouteFamilyReleasePublications,
	)
}

// ControlRevision projects the exact device-signed publication revision.
func (d PublicationRequestDocument) ControlRevision() controlwire.Revision {
	return d.Request.Payload.Revision
}

// ControlNonce projects the signed publication request identity.
func (d PublicationRequestDocument) ControlNonce() controlwire.RequestNonce {
	return d.Request.Payload.Nonce
}

func (PublicationRequestDocument) ControlRequestBodyLimit() (core.ByteCount, error) {
	return core.NewByteCount(uint64(RequestDocumentJSONMaximumBytes))
}

func (a PublicationRequestAssembly) Validate() error {
	return PublicationRequestDocument(a).Validate()
}

func AssemblePublication(assembly PublicationRequestAssembly) (PublicationRequestDocument, error) {
	if err := assembly.Validate(); err != nil {
		return PublicationRequestDocument{}, err
	}
	return PublicationRequestDocument(assembly), nil
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
		return jsonError(errors.New("nil credentialed publication request receiver"))
	}
	wire, err := decodeRequest[publicationRequestDocumentWire](data)
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

func (v PublicationVerification) Validate() error {
	if err := errors.Join(
		v.Document.Validate(), v.TrustedKeys.Validate(), v.ManifestKeys.Validate(),
	); err != nil {
		return contractError(err)
	}
	return nil
}

func VerifyPublication(verification PublicationVerification) (VerifiedPublication, error) {
	if err := verification.Validate(); err != nil {
		return VerifiedPublication{}, err
	}
	certificate, err := controlplane.VerifyInstallationCertificate(
		verification.Document.Certificate, verification.TrustedKeys,
	)
	if err != nil {
		return VerifiedPublication{}, contractError(err)
	}
	deviceKeys, err := certificate.DeviceKeys()
	if err != nil {
		return VerifiedPublication{}, contractError(err)
	}
	certificateBody, err := certificate.Body()
	if err != nil {
		return VerifiedPublication{}, contractError(err)
	}
	request, err := distribution.VerifyPublicationRequest(distribution.PublicationRequestVerification{
		Document: verification.Document.Request, RequestKeys: deviceKeys,
		ManifestKeys:     verification.ManifestKeys,
		ExpectedOffering: certificateBody.Build.Offering(),
	})
	if err != nil {
		return VerifiedPublication{}, contractError(err)
	}
	verified := VerifiedPublication{
		document: verification.Document, requestProof: request, certificateProof: certificate,
	}
	return verified, verified.Validate()
}

func (v VerifiedPublication) Validate() error {
	if err := errors.Join(
		v.document.Validate(), v.requestProof.Validate(), v.certificateProof.Validate(),
	); err != nil {
		return contractError(err)
	}
	return nil
}

func (v VerifiedPublication) Payload() (distribution.PublicationRequestPayload, error) {
	if err := v.Validate(); err != nil {
		return distribution.PublicationRequestPayload{}, err
	}
	return v.requestProof.Payload()
}

func (v VerifiedPublication) Manifest() (release.VerifiedManifest, error) {
	if err := v.Validate(); err != nil {
		return release.VerifiedManifest{}, err
	}
	return v.requestProof.Manifest()
}

type PublicationCompletionDocument struct {
	Completion  distribution.PublicationCompletionDocument   `json:"completion"`
	Certificate controlplane.InstallationCertificateDocument `json:"certificate"`
}

type PublicationCompletionProjection struct {
	completion  distribution.PublicationCompletionProjection
	certificate controlplane.InstallationCertificateDocument
}

type PublicationCompletionAssembly struct {
	Completion  distribution.PublicationCompletionDocument
	Certificate controlplane.InstallationCertificateDocument
}

type PublicationCompletionProjectionAssembly struct {
	Completion  distribution.PublicationCompletionProjection
	Certificate controlplane.InstallationCertificateDocument
}

type PublicationCompletionVerification struct {
	Grant       distribution.PublicationGrantDocument
	Document    PublicationCompletionDocument
	Request     VerifiedPublication
	TrustedKeys attest.TrustedKeys
}

type VerifiedPublicationCompletion struct {
	document         PublicationCompletionDocument
	completionProof  distribution.VerifiedPublicationCompletion
	certificateProof controlplane.VerifiedInstallationCertificate
}

type publicationCompletionDocumentWire PublicationCompletionDocument

func (d PublicationCompletionDocument) Validate() error {
	if err := errors.Join(d.Completion.Validate(), d.Certificate.Validate()); err != nil {
		return contractError(err)
	}
	if d.Completion.Payload.Build != d.Certificate.Body.Build {
		return bindingError()
	}
	return nil
}

// ControlRoute projects the sole route admitted by this publication completion.
func (d PublicationCompletionDocument) ControlRoute() (controlwire.RouteContract, error) {
	return controlwire.NewRouteContract(
		d.Completion.Payload.Build.Offering(),
		controlwire.RouteFamilyReleasePublicationCompletions,
	)
}

// ControlRevision projects the authority-signed installation revision bound to
// this device-signed completion.
func (d PublicationCompletionDocument) ControlRevision() controlwire.Revision {
	return d.Certificate.Body.Revision
}

// ControlNonce projects the completion's independently signed request identity.
func (d PublicationCompletionDocument) ControlNonce() controlwire.RequestNonce {
	return d.Completion.Payload.Nonce
}

func (PublicationCompletionDocument) ControlRequestBodyLimit() (core.ByteCount, error) {
	return core.NewByteCount(uint64(PublicationCompletionDocumentJSONMaximumBytes))
}

func (a PublicationCompletionAssembly) Validate() error {
	return PublicationCompletionDocument(a).Validate()
}

func (a PublicationCompletionProjectionAssembly) Validate() error {
	if err := errors.Join(a.Completion.Validate(), a.Certificate.Validate()); err != nil {
		return contractError(err)
	}
	build, err := a.Completion.Build()
	if err != nil || build != a.Certificate.Body.Build {
		return bindingError(err)
	}
	return nil
}

func AssemblePublicationCompletion(
	assembly PublicationCompletionAssembly,
) (PublicationCompletionDocument, error) {
	if err := assembly.Validate(); err != nil {
		return PublicationCompletionDocument{}, err
	}
	return PublicationCompletionDocument(assembly), nil
}

func AssemblePublicationCompletionProjection(
	assembly PublicationCompletionProjectionAssembly,
) (PublicationCompletionProjection, error) {
	if err := assembly.Validate(); err != nil {
		return PublicationCompletionProjection{}, err
	}
	return PublicationCompletionProjection{
		completion: assembly.Completion, certificate: assembly.Certificate,
	}, nil
}

func (p PublicationCompletionProjection) Validate() error {
	return PublicationCompletionProjectionAssembly{
		Completion: p.completion, Certificate: p.certificate,
	}.Validate()
}

func (p PublicationCompletionProjection) MarshalJSON() ([]byte, error) {
	if err := p.Validate(); err != nil {
		return nil, jsonError(err)
	}
	encoded, err := core.MarshalCanonicalJSONDocument(struct {
		Completion  distribution.PublicationCompletionProjection `json:"completion"`
		Certificate controlplane.InstallationCertificateDocument `json:"certificate"`
	}{Completion: p.completion, Certificate: p.certificate})
	if err != nil || len(encoded) > PublicationCompletionDocumentJSONMaximumBytes {
		return nil, jsonError(err)
	}
	return encoded, nil
}

func (d PublicationCompletionDocument) MarshalJSON() ([]byte, error) {
	if err := d.Validate(); err != nil {
		return nil, jsonError(err)
	}
	encoded, err := core.MarshalCanonicalJSONDocument(publicationCompletionDocumentWire(d))
	if err != nil || len(encoded) > PublicationCompletionDocumentJSONMaximumBytes {
		return nil, jsonError(err)
	}
	return encoded, nil
}

func (d *PublicationCompletionDocument) UnmarshalJSON(data []byte) error {
	if d == nil {
		return jsonError(errors.New("nil credentialed publication completion receiver"))
	}
	maximum, err := core.NewByteCount(uint64(PublicationCompletionDocumentJSONMaximumBytes))
	if err != nil {
		return jsonError(err)
	}
	limits := core.DefaultStrictJSONLimits()
	limits.DocumentMaximumBytes = maximum
	wire, err := core.DecodeStrictJSONStructure[publicationCompletionDocumentWire](data, limits)
	if err != nil {
		return jsonError(err)
	}
	candidate := PublicationCompletionDocument(wire)
	if err := candidate.Validate(); err != nil {
		return jsonError(err)
	}
	*d = candidate
	return nil
}

func (v PublicationCompletionVerification) Validate() error {
	if err := errors.Join(
		v.Grant.Validate(), v.Document.Validate(), v.Request.Validate(), v.TrustedKeys.Validate(),
	); err != nil {
		return contractError(err)
	}
	if v.Document.Certificate != v.Request.document.Certificate {
		return bindingError()
	}
	return nil
}

func VerifyPublicationCompletion(
	verification PublicationCompletionVerification,
) (VerifiedPublicationCompletion, error) {
	if err := verification.Validate(); err != nil {
		return VerifiedPublicationCompletion{}, err
	}
	certificate, err := controlplane.VerifyInstallationCertificate(
		verification.Document.Certificate, verification.TrustedKeys,
	)
	if err != nil {
		return VerifiedPublicationCompletion{}, contractError(err)
	}
	deviceKeys, err := certificate.DeviceKeys()
	if err != nil {
		return VerifiedPublicationCompletion{}, contractError(err)
	}
	completion, err := distribution.VerifyPublicationCompletion(distribution.PublicationCompletionExpectation{
		Document: verification.Document.Completion, Request: verification.Request.requestProof,
		Grant: verification.Grant.Payload, GrantAttestation: verification.Grant.Attestation,
		GrantKeys: verification.TrustedKeys, CompletionKeys: deviceKeys,
	})
	if err != nil {
		return VerifiedPublicationCompletion{}, contractError(err)
	}
	verified := VerifiedPublicationCompletion{
		document: verification.Document, completionProof: completion, certificateProof: certificate,
	}
	return verified, verified.Validate()
}

func (v VerifiedPublicationCompletion) Validate() error {
	if err := errors.Join(
		v.document.Validate(), v.completionProof.Validate(), v.certificateProof.Validate(),
	); err != nil {
		return contractError(err)
	}
	return nil
}

func (v VerifiedPublicationCompletion) Payload() (distribution.PublicationCompletionPayload, error) {
	if err := v.Validate(); err != nil {
		return distribution.PublicationCompletionPayload{}, err
	}
	return v.completionProof.Payload()
}

var (
	_ controlwire.RoutedJSONRequest = PublicationCompletionDocument{}
	_ controlwire.RoutedJSONRequest = PublicationRequestDocument{}
	_ core.Validatable              = PublicationRequestDocument{}
	_ core.Validatable              = PublicationRequestAssembly{}
	_ core.Validatable              = PublicationVerification{}
	_ core.Validatable              = VerifiedPublication{}
	_ core.Validatable              = PublicationCompletionDocument{}
	_ core.Validatable              = PublicationCompletionProjection{}
	_ core.Validatable              = PublicationCompletionAssembly{}
	_ core.Validatable              = PublicationCompletionProjectionAssembly{}
	_ core.Validatable              = PublicationCompletionVerification{}
	_ core.Validatable              = VerifiedPublicationCompletion{}

	_ core.ValidatedJSONMarshaler = PublicationRequestDocument{}
	_ core.ValidatedJSONMarshaler = PublicationCompletionDocument{}
	_ core.ValidatedJSONMarshaler = PublicationCompletionProjection{}
	_ json.Unmarshaler            = (*PublicationRequestDocument)(nil)
	_ json.Unmarshaler            = (*PublicationCompletionDocument)(nil)
)
