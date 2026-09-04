package chitauth

import (
	json "encoding/json/v2"
	"errors"

	"github.com/deliri/primitive/v2026/chit"
	"github.com/deliri/primitive/v2026/controlplane"
	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
)

const (
	RequestDocumentJSONMaximumBytes = chit.QueryDocumentJSONMaximumBytes +
		controlplane.InstallationCertificateDocumentJSONMaximumBytes +
		core.CredentialedRequestDocumentSyntaxBytes + core.CredentialedDocumentWhitespaceMaximumBytes
)

type RequestDocument struct {
	Certificate controlplane.InstallationCertificateDocument `json:"certificate"`
	Request     chit.QueryDocument                           `json:"request"`
}

type RequestAssembly struct {
	Certificate controlplane.InstallationCertificateDocument
	Request     chit.QueryDocument
}

type Verification struct {
	Document RequestDocument
	Server   controlplane.Authority
}

type Verified struct {
	document         RequestDocument
	requestProof     chit.VerifiedQuery
	certificateProof controlplane.VerifiedInstallationCertificate
}

type requestDocumentWire RequestDocument

func (d RequestDocument) Validate() error {
	if err := errors.Join(d.Request.Validate(), d.Certificate.Validate()); err != nil {
		return contractError(err)
	}
	query := d.Request.Payload.Query
	certificateScope, err := d.Certificate.Body.Scope()
	if err != nil {
		return bindingError(err)
	}
	if d.Request.Payload.Build != d.Certificate.Body.Build || query.Scope != certificateScope {
		return bindingError()
	}
	return nil
}

// ControlRoute projects the sole route admitted by this credentialed query.
func (d RequestDocument) ControlRoute() (controlwire.RouteContract, error) {
	return controlwire.NewRouteContract(
		d.Request.Payload.Build.Offering(), controlwire.RouteFamilyChits,
	)
}

// ControlRevision projects the exact device-signed query revision.
func (d RequestDocument) ControlRevision() controlwire.Revision {
	return d.Request.Payload.Revision
}

// ControlNonce projects the signed query identity.
func (d RequestDocument) ControlNonce() controlwire.RequestNonce {
	return d.Request.Payload.Nonce
}

func (RequestDocument) ControlRequestBodyLimit() (core.ByteCount, error) {
	return core.NewByteCount(uint64(RequestDocumentJSONMaximumBytes))
}

func (a RequestAssembly) Validate() error { return RequestDocument(a).Validate() }

func Assemble(assembly RequestAssembly) (RequestDocument, error) {
	if err := assembly.Validate(); err != nil {
		return RequestDocument{}, err
	}
	return RequestDocument(assembly), nil
}

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

func (d *RequestDocument) UnmarshalJSON(data []byte) error {
	if d == nil {
		return jsonError(errors.New("nil credentialed chit query receiver"))
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

func (v Verification) Validate() error {
	if err := errors.Join(v.Server.Validate(), v.Document.Validate()); err != nil {
		return contractError(err)
	}
	return nil
}

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
	request, err := chit.VerifyQuery(chit.QueryVerification{
		Document: verification.Document.Request, TrustedKeys: deviceKeys,
	})
	if err != nil {
		return Verified{}, contractError(err)
	}
	verified := Verified{
		document: verification.Document, requestProof: request, certificateProof: certificate,
	}
	return verified, verified.Validate()
}

func (v Verified) Validate() error {
	if err := errors.Join(v.document.Validate(), v.requestProof.Validate(), v.certificateProof.Validate()); err != nil {
		return contractError(err)
	}
	return nil
}

func (v Verified) Payload() (chit.QueryPayload, error) {
	if err := v.Validate(); err != nil {
		return chit.QueryPayload{}, err
	}
	return v.requestProof.Payload()
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
