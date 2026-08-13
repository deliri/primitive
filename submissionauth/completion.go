package submissionauth

import (
	"encoding/json"
	"errors"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/controlplane"
	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/submission"
)

const (
	completionDocumentSyntaxBytes = len(`{"completion":,"certificate":}`)
	// CompletionDocumentJSONMaximumBytes bounds one credentialed upload
	// completion, including bounded insignificant outer whitespace.
	CompletionDocumentJSONMaximumBytes = submission.CompletionDocumentJSONMaximumBytes +
		controlplane.InstallationCertificateDocumentJSONMaximumBytes +
		completionDocumentSyntaxBytes + core.CredentialedDocumentWhitespaceMaximumBytes
)

// CompletionDocument carries one device-signed provider completion beside the
// installation certificate that nominates the device key.
type CompletionDocument struct {
	Completion  submission.CompletionDocument                `json:"completion"`
	Certificate controlplane.InstallationCertificateDocument `json:"certificate"`
}

// CompletionProjection is the issue-only credentialed form. It keeps the
// inner provider evidence issue-only while binding the installation
// certificate needed by the receiving authority.
type CompletionProjection struct {
	completion  submission.CompletionProjection
	certificate controlplane.InstallationCertificateDocument
}

// CompletionAssembly binds two independently signed documents.
type CompletionAssembly struct {
	Completion  submission.CompletionDocument
	Certificate controlplane.InstallationCertificateDocument
}

// CompletionProjectionAssembly binds an issue-only completion projection to
// the installation certificate that nominates its signing device.
type CompletionProjectionAssembly struct {
	Completion  submission.CompletionProjection
	Certificate controlplane.InstallationCertificateDocument
}

// CompletionVerification supplies the authenticated original request, exact
// grant, and authority keys used for both certificate and grant verification.
type CompletionVerification struct {
	Grant       submission.GrantDocument
	Document    CompletionDocument
	Request     Verified
	TrustedKeys attest.TrustedKeys
}

// VerifiedCompletion proves certificate authentication happened before the
// nominated device key authenticated the completion.
type VerifiedCompletion struct {
	document         CompletionDocument
	completionProof  submission.VerifiedCompletion
	certificateProof controlplane.VerifiedInstallationCertificate
}

type completionDocumentWire CompletionDocument

func (d CompletionDocument) Validate() error {
	if err := errors.Join(d.Completion.Validate(), d.Certificate.Validate()); err != nil {
		return contractError(err)
	}
	if d.Completion.Payload.Build != d.Certificate.Body.Build {
		return bindingError()
	}
	return nil
}

// ControlRoute projects the sole route admitted by this completion.
func (d CompletionDocument) ControlRoute() (controlwire.RouteContract, error) {
	return controlwire.NewRouteContract(
		d.Completion.Payload.Build.Offering(), controlwire.RouteFamilySubmissionCompletions,
	)
}

// ControlRevision projects the authority-signed installation revision bound to
// this device-signed completion.
func (d CompletionDocument) ControlRevision() controlwire.Revision {
	return d.Certificate.Body.Revision
}

// ControlNonce projects the completion's independently signed request identity.
func (d CompletionDocument) ControlNonce() controlwire.RequestNonce {
	return d.Completion.Payload.Nonce
}

func (CompletionDocument) ControlRequestBodyLimit() (core.ByteCount, error) {
	return core.NewByteCount(uint64(CompletionDocumentJSONMaximumBytes))
}

func (a CompletionAssembly) Validate() error {
	return (CompletionDocument(a)).Validate()
}

func (a CompletionProjectionAssembly) Validate() error {
	if err := errors.Join(a.Completion.Validate(), a.Certificate.Validate()); err != nil {
		return contractError(err)
	}
	build, err := a.Completion.Build()
	if err != nil || build != a.Certificate.Body.Build {
		return bindingError()
	}
	return nil
}

// AssembleCompletionProjection binds one issue-only completion to one
// installation certificate without decoding the sender's own wire document.
func AssembleCompletionProjection(assembly CompletionProjectionAssembly) (CompletionProjection, error) {
	if err := assembly.Validate(); err != nil {
		return CompletionProjection{}, err
	}
	return CompletionProjection{completion: assembly.Completion, certificate: assembly.Certificate}, nil
}

func (p CompletionProjection) Validate() error {
	return CompletionProjectionAssembly{Completion: p.completion, Certificate: p.certificate}.Validate()
}

func (p CompletionProjection) MarshalJSON() ([]byte, error) {
	if err := p.Validate(); err != nil {
		return nil, jsonError(err)
	}
	encoded, err := core.MarshalCanonicalJSONDocument(struct {
		Completion  submission.CompletionProjection              `json:"completion"`
		Certificate controlplane.InstallationCertificateDocument `json:"certificate"`
	}{
		Completion: p.completion, Certificate: p.certificate,
	})
	if err != nil || len(encoded) > CompletionDocumentJSONMaximumBytes {
		return nil, jsonError(err)
	}
	return encoded, nil
}

// AssembleCompletion binds one completion to one installation certificate.
func AssembleCompletion(assembly CompletionAssembly) (CompletionDocument, error) {
	if err := assembly.Validate(); err != nil {
		return CompletionDocument{}, err
	}
	return CompletionDocument(assembly), nil
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
		return jsonError(errors.New("nil credentialed submission completion receiver"))
	}
	maximum, err := core.NewByteCount(uint64(CompletionDocumentJSONMaximumBytes))
	if err != nil {
		return jsonError(err)
	}
	limits := core.DefaultStrictJSONLimits()
	limits.DocumentMaximumBytes = maximum
	wire, err := core.DecodeStrictJSONStructure[completionDocumentWire](data, limits)
	if err != nil {
		return jsonError(err)
	}
	candidate := CompletionDocument(wire)
	if err := candidate.Validate(); err != nil {
		return jsonError(err)
	}
	*d = candidate
	return nil
}

func (v CompletionVerification) Validate() error {
	if err := errors.Join(
		v.Document.Validate(), v.Request.Validate(), v.Grant.Validate(), v.TrustedKeys.Validate(),
	); err != nil {
		return contractError(err)
	}
	if v.Document.Certificate != v.Request.document.Certificate {
		return bindingError()
	}
	return nil
}

// VerifyCompletion authenticates the installation certificate first, then
// uses only its nominated device key to authenticate the completion and bind
// it to the previously authenticated request and authority grant.
func VerifyCompletion(verification CompletionVerification) (VerifiedCompletion, error) {
	if err := verification.Validate(); err != nil {
		return VerifiedCompletion{}, err
	}
	certificate, err := controlplane.VerifyInstallationCertificate(
		verification.Document.Certificate, verification.TrustedKeys,
	)
	if err != nil {
		return VerifiedCompletion{}, contractError(err)
	}
	deviceKeys, err := certificate.DeviceKeys()
	if err != nil {
		return VerifiedCompletion{}, contractError(err)
	}
	completion, err := submission.VerifyCompletion(submission.CompletionExpectation{
		Document: verification.Document.Completion,
		Request:  verification.Request.document.Request.Payload,
		Grant:    verification.Grant, GrantKeys: verification.TrustedKeys,
		CompletionKeys: deviceKeys,
	})
	if err != nil {
		return VerifiedCompletion{}, contractError(err)
	}
	verified := VerifiedCompletion{
		document: verification.Document, certificateProof: certificate,
		completionProof: completion,
	}
	return verified, verified.Validate()
}

func (v VerifiedCompletion) Validate() error {
	if err := errors.Join(
		v.document.Validate(), v.certificateProof.Validate(), v.completionProof.Validate(),
	); err != nil {
		return contractError(err)
	}
	return nil
}

func (v VerifiedCompletion) Payload() (submission.CompletionPayload, error) {
	if err := v.Validate(); err != nil {
		return submission.CompletionPayload{}, err
	}
	return v.completionProof.Payload()
}

var (
	_ controlwire.RoutedJSONRequest = CompletionDocument{}
	_ core.Validatable              = CompletionDocument{}
	_ core.Validatable              = CompletionProjection{}
	_ core.Validatable              = CompletionAssembly{}
	_ core.Validatable              = CompletionProjectionAssembly{}
	_ core.Validatable              = CompletionVerification{}
	_ core.Validatable              = VerifiedCompletion{}

	_ core.ValidatedJSONMarshaler = CompletionDocument{}
	_ core.ValidatedJSONMarshaler = CompletionProjection{}
	_ json.Unmarshaler            = (*CompletionDocument)(nil)
)
