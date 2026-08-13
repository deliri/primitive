package submission

import (
	"crypto"
	"encoding/json"
	"errors"
	"io"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
)

const (
	// RequestPayloadJSONMaximumBytes bounds the device-signed declaration.
	RequestPayloadJSONMaximumBytes = 32 << 10
	// RequestDocumentJSONMaximumBytes bounds one signed request document.
	RequestDocumentJSONMaximumBytes      = 64 << 10
	requestCommitmentDomain              = "primitive/submission/request-commitment/v1"
	requestCommitmentSeparator      byte = 0
)

// RequestPayload is the exact declaration one installed build signs.
type RequestPayload struct {
	Manifest    ManifestIntent           `json:"manifest"`
	Declaration Declaration              `json:"declaration"`
	Build       core.BuildIdentity       `json:"build"`
	Nonce       controlwire.RequestNonce `json:"request_nonce"`
	Revision    controlwire.Revision     `json:"revision"`
}

// RequestDocument carries one device-signed declaration. Which device key is
// trusted is supplied separately by the authentication layer.
type RequestDocument struct {
	Payload     RequestPayload                 `json:"payload"`
	Attestation attest.Envelope[SigningDomain] `json:"attestation"`
}

// RequestIssuance carries all inputs for device-side request signing.
type RequestIssuance struct {
	Signer  crypto.Signer
	Payload RequestPayload
}

// Validate closes every issuance input without signing.
func (i RequestIssuance) Validate() error {
	if err := (attest.SignRequest[SigningDomain]{
		Body: i.Payload, Signer: i.Signer,
	}).Validate(); err != nil {
		return contractError(err)
	}
	return nil
}

// RequestVerification carries caller-selected device keys and an untrusted
// request into authentication.
type RequestVerification struct {
	Document    RequestDocument
	TrustedKeys attest.TrustedKeys
}

// VerifiedRequest can only be obtained by authenticating the device signature.
type VerifiedRequest struct {
	document RequestDocument
	proof    attest.Verified[SigningDomain]
}

type (
	requestPayloadWire  RequestPayload
	requestDocumentWire RequestDocument
)

// Validate closes every signed request fact.
func (p RequestPayload) Validate() error {
	if err := errors.Join(
		p.Declaration.Validate(), p.Manifest.Validate(), p.Build.Validate(),
		p.Revision.Validate(), p.Nonce.Validate(),
	); err != nil {
		return contractError(err)
	}
	return nil
}

// AttestationDomain returns the device request namespace.
func (RequestPayload) AttestationDomain() SigningDomain { return SigningDomainRequestV1 }

// WriteCanonical writes the exact compact signed payload.
func (p RequestPayload) WriteCanonical(destination io.Writer) error {
	encoded, err := p.MarshalJSON()
	if err != nil {
		return err
	}
	_, err = destination.Write(encoded)
	return err
}

// MarshalJSON emits one bounded canonical payload.
func (p RequestPayload) MarshalJSON() ([]byte, error) {
	if err := p.Validate(); err != nil {
		return nil, jsonError(err)
	}
	encoded, err := core.MarshalCanonicalJSONDocument(requestPayloadWire(p))
	if err != nil || len(encoded) > RequestPayloadJSONMaximumBytes {
		return nil, jsonError(err)
	}
	return encoded, nil
}

// UnmarshalJSON strictly decodes without mutating the receiver on rejection.
func (p *RequestPayload) UnmarshalJSON(data []byte) error {
	if p == nil {
		return jsonError(errors.New("nil submission request payload receiver"))
	}
	wire, err := decodeStrict[requestPayloadWire](data, RequestPayloadJSONMaximumBytes)
	if err != nil {
		return err
	}
	candidate := RequestPayload(wire)
	if err := candidate.Validate(); err != nil {
		return jsonError(err)
	}
	*p = candidate
	return nil
}

// Validate closes the payload and exact request-signature namespace.
func (d RequestDocument) Validate() error {
	if err := errors.Join(d.Payload.Validate(), d.Attestation.Validate()); err != nil {
		return contractError(err)
	}
	if d.Attestation.Domain != d.Payload.AttestationDomain() {
		return bindingError()
	}
	return nil
}

// MarshalJSON emits one bounded canonical request.
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

// UnmarshalJSON strictly decodes without mutating the receiver on rejection.
func (d *RequestDocument) UnmarshalJSON(data []byte) error {
	if d == nil {
		return jsonError(errors.New("nil submission request document receiver"))
	}
	wire, err := decodeStrict[requestDocumentWire](data, RequestDocumentJSONMaximumBytes)
	if err != nil {
		return err
	}
	candidate := RequestDocument(wire)
	if err := candidate.Validate(); err != nil {
		return jsonError(err)
	}
	*d = candidate
	return nil
}

// IssueRequest signs one exact declaration with the installed device key.
func IssueRequest(issuance RequestIssuance) (RequestDocument, error) {
	if err := issuance.Validate(); err != nil {
		return RequestDocument{}, err
	}
	envelope, err := attest.Sign(attest.SignRequest[SigningDomain]{
		Body: issuance.Payload, Signer: issuance.Signer,
	})
	if err != nil {
		return RequestDocument{}, contractError(err)
	}
	document := RequestDocument{
		Payload: issuance.Payload, Attestation: envelope,
	}
	if err := document.Validate(); err != nil {
		return RequestDocument{}, err
	}
	return document, nil
}

// Validate closes the complete authority verification input.
func (v RequestVerification) Validate() error {
	if err := errors.Join(v.Document.Validate(), v.TrustedKeys.Validate()); err != nil {
		return contractError(err)
	}
	return nil
}

// VerifyRequest authenticates the exact request with only the caller-selected
// device keys. Submissionauth owns how an installation certificate nominates
// that set.
func VerifyRequest(verification RequestVerification) (VerifiedRequest, error) {
	if err := verification.Validate(); err != nil {
		return VerifiedRequest{}, err
	}
	proof, err := attest.Verify(attest.VerifyRequest[SigningDomain]{
		Body: verification.Document.Payload, Envelope: verification.Document.Attestation,
		TrustedKeys: verification.TrustedKeys,
	})
	if err != nil {
		return VerifiedRequest{}, contractError(err)
	}
	verified := VerifiedRequest{document: verification.Document, proof: proof}
	return verified, verified.Validate()
}

// Validate revalidates the authenticated request proof.
func (v VerifiedRequest) Validate() error {
	if err := errors.Join(v.document.Validate(), v.proof.Validate()); err != nil {
		return contractError(err)
	}
	return nil
}

// Document returns the authenticated request.
func (v VerifiedRequest) Document() (RequestDocument, error) {
	if err := v.Validate(); err != nil {
		return RequestDocument{}, err
	}
	return v.document, nil
}

// RequestCommitment is the non-secret domain-separated closure of one exact
// request payload.
type RequestCommitment struct {
	digest core.SHA256Digest
}

// CommitRequest closes one exact validated request payload.
func CommitRequest(payload RequestPayload) (RequestCommitment, error) {
	encoded, err := payload.MarshalJSON()
	if err != nil {
		return RequestCommitment{}, err
	}
	writer := core.NewDigestWriter()
	if _, err := writer.Write([]byte(requestCommitmentDomain)); err != nil {
		return RequestCommitment{}, contractError(err)
	}
	if _, err := writer.Write([]byte{requestCommitmentSeparator}); err != nil {
		return RequestCommitment{}, contractError(err)
	}
	if _, err := writer.Write(encoded); err != nil {
		return RequestCommitment{}, contractError(err)
	}
	digest, _, err := writer.Seal()
	if err != nil {
		return RequestCommitment{}, contractError(err)
	}
	return newRequestCommitment(digest)
}

func newRequestCommitment(digest core.SHA256Digest) (RequestCommitment, error) {
	candidate := RequestCommitment{digest: digest}
	if err := candidate.Validate(); err != nil {
		return RequestCommitment{}, err
	}
	return candidate, nil
}

// Validate rejects an unset commitment.
func (c RequestCommitment) Validate() error {
	return validateNonzeroDigest(c.digest, "request commitment is all zero")
}

func validateNonzeroDigest(digest core.SHA256Digest, diagnostic string) error {
	raw, err := digest.Bytes()
	if err != nil {
		return contractError(err)
	}
	for _, value := range raw {
		if value != 0 {
			return nil
		}
	}
	return contractError(errors.New(diagnostic))
}

// MarshalJSON emits canonical lowercase hexadecimal.
func (c RequestCommitment) MarshalJSON() ([]byte, error) {
	if err := c.Validate(); err != nil {
		return nil, jsonError(err)
	}
	return json.Marshal(c.digest)
}

// UnmarshalJSON accepts one canonical SHA-256 commitment without mutating on refusal.
func (c *RequestCommitment) UnmarshalJSON(data []byte) error {
	if c == nil {
		return jsonError(errors.New("nil request commitment receiver"))
	}
	var digest core.SHA256Digest
	if err := json.Unmarshal(data, &digest); err != nil {
		return jsonError(err)
	}
	candidate, err := newRequestCommitment(digest)
	if err != nil {
		return jsonError(err)
	}
	*c = candidate
	return nil
}

func decodeStrict[T any](data []byte, maximum uint64) (T, error) {
	var zero T
	limit, err := core.NewByteCount(maximum)
	if err != nil {
		return zero, jsonError(err)
	}
	limits := core.DefaultStrictJSONLimits()
	limits.DocumentMaximumBytes = limit
	wire, err := core.DecodeStrictJSONStructure[T](data, limits)
	if err != nil {
		return zero, jsonError(err)
	}
	return wire, nil
}

var (
	_ core.Validatable = RequestPayload{}
	_ core.Validatable = RequestDocument{}
	_ core.Validatable = RequestIssuance{}
	_ core.Validatable = RequestVerification{}
	_ core.Validatable = VerifiedRequest{}
	_ core.Validatable = RequestCommitment{}

	_ core.ValidatedJSONMarshaler         = RequestPayload{}
	_ core.ValidatedJSONMarshaler         = RequestDocument{}
	_ core.ValidatedJSONMarshaler         = RequestCommitment{}
	_ json.Unmarshaler                    = (*RequestPayload)(nil)
	_ json.Unmarshaler                    = (*RequestDocument)(nil)
	_ json.Unmarshaler                    = (*RequestCommitment)(nil)
	_ attest.CanonicalBody[SigningDomain] = RequestPayload{}
)
