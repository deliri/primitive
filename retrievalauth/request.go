package retrievalauth

import (
	json "encoding/json/v2"
	"errors"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/controlplane"
	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/retrieval"
)

type RequestDocument struct {
	Certificate controlplane.InstallationCertificateDocument `json:"certificate"`
	Request     retrieval.RequestDocument                    `json:"request"`
}

func (d RequestDocument) Validate() error {
	if err := errors.Join(d.Request.Validate(), d.Certificate.Validate()); err != nil {
		return contractError(err)
	}
	if d.Request.Payload.Build != d.Certificate.Body.Build {
		return bindingError(errors.New("retrieval request build differs from certificate"))
	}
	scope, err := d.Certificate.Body.Scope()
	if err != nil || d.Request.Payload.Scope != scope {
		return bindingError(errors.New("retrieval request scope differs from certificate"), err)
	}
	if d.Request.Attestation.Signer != d.Certificate.Body.DeviceKey {
		return bindingError(errors.New("retrieval request signer differs from certificate"))
	}
	return nil
}

// ControlRoute projects the sole route admitted by this credentialed request.
func (d RequestDocument) ControlRoute() (controlwire.RouteContract, error) {
	if err := d.Validate(); err != nil {
		return controlwire.RouteContract{}, err
	}
	return controlwire.NewRouteContract(
		d.Request.Payload.Build.Offering(), controlwire.RouteFamilyRetrievals,
	)
}

// ControlRevision projects the exact device-signed retrieval revision.
func (d RequestDocument) ControlRevision() controlwire.Revision {
	return d.Request.Payload.Revision
}

// ControlNonce projects the signed request identity.
func (d RequestDocument) ControlNonce() controlwire.RequestNonce {
	return d.Request.Payload.Nonce
}

type RequestAssembly struct {
	Certificate controlplane.InstallationCertificateDocument
	Request     retrieval.RequestDocument
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
	if err != nil {
		return nil, jsonError(err)
	}
	return encoded, nil
}

func (d *RequestDocument) UnmarshalJSON(data []byte) error {
	if d == nil {
		return jsonError(errors.New("nil credentialed retrieval request receiver"))
	}
	type wire RequestDocument
	decoded, err := core.DecodeStrictJSONStructure[wire](data, core.ExtensibleJSONLimits())
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
	Document RequestDocument
	Server   controlplane.Authority
}

func (v Verification) Validate() error {
	if err := errors.Join(v.Server.Validate(), v.Document.Validate()); err != nil {
		return contractError(err)
	}
	return nil
}

type Verified struct {
	document         RequestDocument
	certificateProof controlplane.VerifiedInstallationCertificate
	requestProof     attest.Verified[retrieval.SigningDomain]
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
	proof, err := attest.Verify(attest.VerifyRequest[retrieval.SigningDomain]{
		Body:        verification.Document.Request.Payload,
		Envelope:    verification.Document.Request.Attestation,
		TrustedKeys: deviceKeys,
	})
	if err != nil {
		return Verified{}, contractError(err)
	}
	verified := Verified{document: verification.Document, requestProof: proof, certificateProof: certificate}
	if err := verified.Validate(); err != nil {
		return Verified{}, err
	}
	return verified, nil
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
	_ controlwire.RoutedJSONRequest = RequestDocument{}
	_ core.Validatable              = RequestDocument{}
	_ core.Validatable              = RequestAssembly{}
	_ core.Validatable              = Verification{}
	_ core.Validatable              = Verified{}
	_ core.ValidatedJSONMarshaler   = RequestDocument{}
	_ json.Unmarshaler              = (*RequestDocument)(nil)
)
