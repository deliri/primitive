package submissionauth

import (
	json "encoding/json/v2"
	"errors"

	"github.com/deliri/primitive/v2026/controlplane"
	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/submission"
)

const (
	// RequestDocumentJSONMaximumBytes bounds one credentialed submission
	// request, including bounded insignificant outer whitespace.
	RequestDocumentJSONMaximumBytes = submission.RequestDocumentJSONMaximumBytes +
		controlplane.InstallationCertificateDocumentJSONMaximumBytes +
		core.CredentialedRequestDocumentSyntaxBytes + core.CredentialedDocumentWhitespaceMaximumBytes
)

// RequestDocument carries the signed evidence declaration and the
// installation certificate that nominates its device key.
type RequestDocument struct {
	Request     submission.RequestDocument                   `json:"request"`
	Certificate controlplane.InstallationCertificateDocument `json:"certificate"`
}

// RequestAssembly carries the two independently issued documents into their
// binding constructor.
type RequestAssembly struct {
	Request     submission.RequestDocument
	Certificate controlplane.InstallationCertificateDocument
}

// Verification carries one authority capability and one untrusted credentialed
// request into authentication.
type Verification struct {
	Document RequestDocument
	Server   controlplane.Authority
}

// Verified proves the authority certificate authenticated before its device
// key became the sole authority for the Submission request.
type Verified struct {
	document         RequestDocument
	requestProof     submission.VerifiedRequest
	certificateProof controlplane.VerifiedInstallationCertificate
}

type requestDocumentWire RequestDocument

// Validate closes both documents and binds their exact build identities.
func (d RequestDocument) Validate() error {
	if err := errors.Join(d.Request.Validate(), d.Certificate.Validate()); err != nil {
		return contractError(err)
	}
	if d.Request.Payload.Build != d.Certificate.Body.Build {
		return bindingError()
	}
	return nil
}

// ControlRoute projects the sole route admitted by this credentialed request.
func (d RequestDocument) ControlRoute() (controlwire.RouteContract, error) {
	return controlwire.NewRouteContract(
		d.Request.Payload.Build.Offering(), controlwire.RouteFamilySubmissions,
	)
}

// ControlRevision projects the exact device-signed request revision.
func (d RequestDocument) ControlRevision() controlwire.Revision {
	return d.Request.Payload.Revision
}

// ControlNonce projects the signed request identity.
func (d RequestDocument) ControlNonce() controlwire.RequestNonce {
	return d.Request.Payload.Nonce
}

func (RequestDocument) ControlRequestBodyLimit() (core.ByteCount, error) {
	return core.NewByteCount(uint64(RequestDocumentJSONMaximumBytes))
}

// Validate closes every assembly input without constructing a document.
func (a RequestAssembly) Validate() error {
	return RequestDocument(a).Validate()
}

// Assemble binds one independently signed request to one independently signed
// installation certificate without adding a compatibility or policy layer.
func Assemble(assembly RequestAssembly) (RequestDocument, error) {
	if err := assembly.Validate(); err != nil {
		return RequestDocument{}, err
	}
	document := RequestDocument(assembly)
	return document, nil
}

// MarshalJSON emits one bounded canonical credentialed request.
func (d RequestDocument) MarshalJSON() ([]byte, error) {
	if err := d.Validate(); err != nil {
		return nil, jsonError(err)
	}
	encoded, err := core.MarshalCanonicalJSONDocument(requestDocumentWire(d))
	if err != nil || len(encoded) > RequestDocumentJSONMaximumBytes {
		return nil, jsonError(err)
	}
	return encoded, nil
}

// UnmarshalJSON strictly decodes without mutating the receiver on refusal.
func (d *RequestDocument) UnmarshalJSON(data []byte) error {
	if d == nil {
		return jsonError(errors.New("nil credentialed submission request receiver"))
	}
	maximum, err := core.NewByteCount(uint64(RequestDocumentJSONMaximumBytes))
	if err != nil {
		return jsonError(err)
	}
	limits := core.DefaultStrictJSONLimits()
	limits.DocumentMaximumBytes = maximum
	wire, err := core.DecodeStrictJSONStructure[requestDocumentWire](data, limits)
	if err != nil {
		return jsonError(err)
	}
	candidate := RequestDocument(wire)
	if err := candidate.Validate(); err != nil {
		return jsonError(err)
	}
	*d = candidate
	return nil
}

// Validate closes the full authority-verification input.
func (v Verification) Validate() error {
	if err := errors.Join(v.Server.Validate(), v.Document.Validate()); err != nil {
		return contractError(err)
	}
	return nil
}

// Verify authenticates the certificate before reading its device keys, then
// authenticates the exact Submission request under only those keys.
func Verify(verification Verification) (Verified, error) {
	if err := verification.Validate(); err != nil {
		return Verified{}, err
	}
	certificate, err := verification.Server.VerifyInstallationCertificate(
		verification.Document.Certificate,
	)
	if err != nil {
		return Verified{}, contractError(err)
	}
	deviceKeys, err := certificate.DeviceKeys()
	if err != nil {
		return Verified{}, contractError(err)
	}
	request, err := submission.VerifyRequest(submission.RequestVerification{
		Document: verification.Document.Request, TrustedKeys: deviceKeys,
	})
	if err != nil {
		return Verified{}, contractError(err)
	}
	verified := Verified{
		document: verification.Document, certificateProof: certificate, requestProof: request,
	}
	return verified, verified.Validate()
}

// Validate revalidates both authenticated proofs and their document binding.
func (v Verified) Validate() error {
	if err := errors.Join(
		v.document.Validate(), v.certificateProof.Validate(), v.requestProof.Validate(),
	); err != nil {
		return contractError(err)
	}
	return nil
}

// Document returns the authenticated credentialed request.
func (v Verified) Document() (RequestDocument, error) {
	if err := v.Validate(); err != nil {
		return RequestDocument{}, err
	}
	return v.document, nil
}

var (
	_ controlwire.RoutedJSONRequest = RequestDocument{}
	_ core.Validatable              = RequestDocument{}
	_ core.Validatable              = RequestAssembly{}
	_ core.Validatable              = Verification{}
	_ core.Validatable              = Verified{}

	_ core.ValidatedJSONMarshaler = RequestDocument{}
	_ json.Unmarshaler            = (*RequestDocument)(nil)
)
