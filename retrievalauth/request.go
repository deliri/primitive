package retrievalauth

import (
	"encoding/json"
	"errors"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/controlplane"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/retrieval"
)

const (
	requestDocumentSyntaxBytes      = len(`{"request":,"certificate":}`)
	requestDocumentWhitespaceBytes  = 8 << 10
	RequestDocumentJSONMaximumBytes = retrieval.RequestDocumentJSONMaximumBytes +
		controlplane.InstallationCertificateDocumentJSONMaximumBytes +
		requestDocumentSyntaxBytes + requestDocumentWhitespaceBytes
)

type RequestDocument struct {
	Request     retrieval.RequestDocument                    `json:"request"`
	Certificate controlplane.InstallationCertificateDocument `json:"certificate"`
}

func (d RequestDocument) Validate() error {
	if err := errors.Join(d.Request.Validate(), d.Certificate.Validate()); err != nil {
		return contractError(err)
	}
	if d.Request.Payload.Build != d.Certificate.Body.Build {
		return bindingError(errors.New("retrieval request build differs from certificate"))
	}
	return nil
}

type RequestAssembly struct {
	Request     retrieval.RequestDocument
	Certificate controlplane.InstallationCertificateDocument
}

func (a RequestAssembly) Validate() error {
	return RequestDocument(a).Validate()
}

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
	type wire RequestDocument
	encoded, err := core.MarshalCanonicalJSONDocument(wire(d))
	if err != nil || len(encoded) > RequestDocumentJSONMaximumBytes {
		return nil, jsonError(err)
	}
	return encoded, nil
}

func (d *RequestDocument) UnmarshalJSON(data []byte) error {
	if d == nil {
		return jsonError(errors.New("nil credentialed retrieval request receiver"))
	}
	maximum, err := core.NewByteCount(uint64(RequestDocumentJSONMaximumBytes))
	if err != nil {
		return jsonError(err)
	}
	limits := core.DefaultStrictJSONLimits()
	limits.DocumentMaximumBytes = maximum
	type wire RequestDocument
	decoded, err := core.DecodeStrictJSONStructure[wire](data, limits)
	if err != nil {
		return jsonError(err)
	}
	candidate := RequestDocument(decoded)
	if err := candidate.Validate(); err != nil {
		return jsonError(err)
	}
	*d = candidate
	return nil
}

type Verification struct {
	Document    RequestDocument
	TrustedKeys attest.TrustedKeys
}

func (v Verification) Validate() error {
	if err := errors.Join(v.Document.Validate(), v.TrustedKeys.Validate()); err != nil {
		return contractError(err)
	}
	return nil
}

type Verified struct {
	document         RequestDocument
	requestProof     attest.Verified[retrieval.SigningDomain]
	certificateProof controlplane.VerifiedInstallationCertificate
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
	proof, err := attest.Verify(attest.VerifyRequest[retrieval.SigningDomain]{
		Body:        verification.Document.Request.Payload,
		Envelope:    verification.Document.Request.Attestation,
		TrustedKeys: deviceKeys,
	})
	if err != nil {
		return Verified{}, contractError(err)
	}
	verified := Verified{document: verification.Document, requestProof: proof, certificateProof: certificate}
	return verified, verified.Validate()
}

func (v Verified) Validate() error {
	if err := errors.Join(v.document.Validate(), v.requestProof.Validate(), v.certificateProof.Validate()); err != nil {
		return contractError(err)
	}
	return nil
}

func (v Verified) Document() (RequestDocument, error) {
	if err := v.Validate(); err != nil {
		return RequestDocument{}, err
	}
	return v.document, nil
}

var (
	_ core.Validatable            = RequestDocument{}
	_ core.Validatable            = RequestAssembly{}
	_ core.Validatable            = Verification{}
	_ core.Validatable            = Verified{}
	_ core.ValidatedJSONMarshaler = RequestDocument{}
	_ json.Unmarshaler            = (*RequestDocument)(nil)
)
