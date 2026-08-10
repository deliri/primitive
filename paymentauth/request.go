package paymentauth

import (
	"encoding/json"
	"errors"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/controlplane"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/payment"
)

const (
	RequestDocumentJSONMaximumBytes = payment.QueryDocumentJSONMaximumBytes +
		controlplane.InstallationCertificateDocumentJSONMaximumBytes +
		core.CredentialedRequestDocumentSyntaxBytes + core.CredentialedDocumentWhitespaceMaximumBytes
)

type RequestDocument struct {
	Request     payment.QueryDocument                        `json:"request"`
	Certificate controlplane.InstallationCertificateDocument `json:"certificate"`
}

type RequestAssembly struct {
	Request     payment.QueryDocument
	Certificate controlplane.InstallationCertificateDocument
}

type Verification struct {
	Document    RequestDocument
	TrustedKeys attest.TrustedKeys
}

type Verified struct {
	document         RequestDocument
	requestProof     payment.VerifiedQuery
	certificateProof controlplane.VerifiedInstallationCertificate
}

type requestDocumentWire RequestDocument

func (d RequestDocument) Validate() error {
	if err := errors.Join(d.Request.Validate(), d.Certificate.Validate()); err != nil {
		return contractError(err)
	}
	query := d.Request.Payload.Query
	if d.Request.Payload.Build != d.Certificate.Body.Build ||
		query.Scope.Account != d.Certificate.Body.Account {
		return bindingError()
	}
	return nil
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
		return jsonError(errors.New("nil credentialed payment query receiver"))
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
	if err := errors.Join(v.Document.Validate(), v.TrustedKeys.Validate()); err != nil {
		return contractError(err)
	}
	return nil
}

func Verify(verification Verification) (Verified, error) {
	if err := verification.Validate(); err != nil {
		return Verified{}, err
	}
	certificate, err := controlplane.VerifyInstallationCertificate(
		verification.Document.Certificate, verification.TrustedKeys,
	)
	if err != nil {
		return Verified{}, contractError(err)
	}
	deviceKeys, err := certificate.DeviceKeys()
	if err != nil {
		return Verified{}, contractError(err)
	}
	request, err := payment.VerifyQuery(payment.QueryVerification{
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

func (v Verified) Payload() (payment.QueryPayload, error) {
	if err := v.Validate(); err != nil {
		return payment.QueryPayload{}, err
	}
	return v.requestProof.Payload()
}

var (
	_ core.Validatable = RequestDocument{}
	_ core.Validatable = RequestAssembly{}
	_ core.Validatable = Verification{}
	_ core.Validatable = Verified{}

	_ core.ValidatedJSONMarshaler = RequestDocument{}
	_ json.Unmarshaler            = (*RequestDocument)(nil)
)
