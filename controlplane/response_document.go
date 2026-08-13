package controlplane

import (
	"bytes"
	"crypto"
	"encoding/json"
	"errors"
	"io"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
)

const (
	// ResponseCommitmentJSONMaximumBytes bounds the canonical signed facts.
	ResponseCommitmentJSONMaximumBytes = 4 << 10
)

// ResponseCommitment is the one authority-signed agreement joining a common
// control header to the exact canonical bytes of a product-owned response.
// The body remains typed on the wire while its digest keeps bearer documents
// receive-only after admission.
type ResponseCommitment struct {
	Header     ResponseHeader    `json:"header"`
	BodySHA256 core.SHA256Digest `json:"body_sha256"`
	BodyLength core.ByteLength   `json:"body_length_bytes"`
}

// ResponseIssuance is the authority input for one authenticated response.
type ResponseIssuance[Body core.ValidatedJSONMarshaler] struct {
	Signer     crypto.Signer
	Header     ResponseHeader
	Body       Body
	Assessment controlwire.ProtocolAssessment
}

// UpgradeRequiredIssuance is the authority input for one common signed
// refusal. It deliberately carries no product body: an incompatible client
// must never be asked to decode facts from a contract it cannot speak.
type UpgradeRequiredIssuance struct {
	Signer     crypto.Signer
	Header     ResponseHeader
	Assessment controlwire.ProtocolAssessment
}

// ResponseProjection is the issue-only authenticated wire response. The
// product body can therefore use an encode-only bearer projection.
type ResponseProjection[Body core.ValidatedJSONMarshaler] struct {
	header      ResponseHeader
	body        Body
	attestation attest.Envelope[SigningDomain]
}

// ResponseDocument is the receive-only authenticated wire response. Its
// fields remain inaccessible until VerifyResponse has authenticated the
// authority and bound the header to the originating request.
type ResponseDocument[
	Body any,
	BodyPtr interface {
		*Body
		core.Validatable
		json.Unmarshaler
	},
] struct {
	body        Body
	commitment  ResponseCommitment
	attestation attest.Envelope[SigningDomain]
	set         bool
}

// ResponseVerification supplies caller-owned trust and request facts.
type ResponseVerification[
	Body any,
	BodyPtr interface {
		*Body
		core.Validatable
		json.Unmarshaler
	},
] struct {
	Document    ResponseDocument[Body, BodyPtr]
	Expected    ResponseExpectation
	TrustedKeys attest.TrustedKeys
}

// VerifiedResponse is the only path that exposes an authenticated product
// body. Its fields are deliberately unexported.
type VerifiedResponse[
	Body any,
	BodyPtr interface {
		*Body
		core.Validatable
		json.Unmarshaler
	},
] struct {
	body   Body
	header ResponseHeader
	proof  attest.Verified[SigningDomain]
}

type responseCommitmentWire ResponseCommitment

type responseDocumentWire struct {
	Header ResponseHeader `json:"header"`
	// doctrine:local-allowed=external-wire
	Body        json.RawMessage                `json:"body,omitempty"`
	Attestation attest.Envelope[SigningDomain] `json:"attestation"`
}

var _ core.ValidatedJSONProjection = ResponseProjection[RegistrationDocument]{}

func (c ResponseCommitment) Validate() error {
	if err := errors.Join(c.Header.Validate(), c.BodySHA256.Validate(), c.BodyLength.Validate()); err != nil {
		return responseDocumentError(err)
	}
	if err := validateResponseBodyPresence(c.Header, c.BodyLength); err != nil {
		return responseDocumentError(err)
	}
	return nil
}

func (ResponseCommitment) AttestationDomain() SigningDomain {
	return SigningDomainResponseV1
}

func (c ResponseCommitment) WriteCanonical(destination io.Writer) error {
	encoded, err := c.MarshalJSON()
	if err != nil {
		return err
	}
	_, err = destination.Write(encoded)
	return err
}

func (c ResponseCommitment) MarshalJSON() ([]byte, error) {
	if err := c.Validate(); err != nil {
		return nil, jsonError(err)
	}
	encoded, err := core.MarshalCanonicalJSONDocument(responseCommitmentWire(c))
	if err != nil || len(encoded) > ResponseCommitmentJSONMaximumBytes {
		return nil, jsonError(responseDocumentError(err))
	}
	return encoded, nil
}

func (c *ResponseCommitment) UnmarshalJSON(data []byte) error {
	if c == nil {
		return jsonError(responseDocumentError())
	}
	limits, err := responseJSONLimits(ResponseCommitmentJSONMaximumBytes)
	if err != nil {
		return jsonError(responseDocumentError(err))
	}
	wire, err := core.DecodeStrictJSONStructure[responseCommitmentWire](data, limits)
	if err != nil {
		return jsonError(responseDocumentError(err))
	}
	candidate := ResponseCommitment(wire)
	if err := candidate.Validate(); err != nil {
		return jsonError(err)
	}
	*c = candidate
	return nil
}

func (i ResponseIssuance[Body]) Validate() error {
	if err := validateResponseAssessment(i.Header, i.Assessment, controlwire.ProtocolSupportOutcomeAccepted); err != nil {
		return err
	}
	commitment, _, err := responseCommitmentFor(i.Header, i.Body)
	if err != nil {
		return err
	}
	request := attest.SignRequest[SigningDomain]{Body: commitment, Signer: i.Signer}
	if err := request.Validate(); err != nil {
		return responseDocumentError(err)
	}
	return nil
}

// ValidateForFamily closes the product-auth boundary: a product response
// wrapper names its one route family here, while the generic envelope remains
// blind to the product body it signs.
func (i ResponseIssuance[Body]) ValidateForFamily(family controlwire.RouteFamily) error {
	if err := family.Validate(); err != nil {
		return responseDocumentError(err)
	}
	if err := i.Validate(); err != nil {
		return err
	}
	if i.Header.Family != family {
		return responseDocumentError(core.ErrControlPlaneResponseBinding)
	}
	return nil
}

func (i UpgradeRequiredIssuance) Validate() error {
	if err := validateResponseAssessment(i.Header, i.Assessment, controlwire.ProtocolSupportOutcomeUpgradeRequired); err != nil {
		return err
	}
	commitment, err := responseCommitmentFromRaw(i.Header, nil)
	if err != nil {
		return err
	}
	request := attest.SignRequest[SigningDomain]{Body: commitment, Signer: i.Signer}
	if err := request.Validate(); err != nil {
		return responseDocumentError(err)
	}
	return nil
}

// IssueResponse signs one response header and the exact canonical product
// body as one indivisible authority agreement.
func IssueResponse[Body core.ValidatedJSONMarshaler](
	issuance ResponseIssuance[Body],
) (ResponseProjection[Body], error) {
	if err := issuance.Validate(); err != nil {
		return ResponseProjection[Body]{}, err
	}
	return issueResponse(issuance)
}

// IssueResponseForFamily is the product-auth issuance door. It prevents a
// valid body and assessment from being signed for a sibling route family.
func IssueResponseForFamily[Body core.ValidatedJSONMarshaler](
	issuance ResponseIssuance[Body],
	family controlwire.RouteFamily,
) (ResponseProjection[Body], error) {
	if err := issuance.ValidateForFamily(family); err != nil {
		return ResponseProjection[Body]{}, err
	}
	return issueResponse(issuance)
}

func issueResponse[Body core.ValidatedJSONMarshaler](
	issuance ResponseIssuance[Body],
) (ResponseProjection[Body], error) {
	commitment, _, err := responseCommitmentFor(issuance.Header, issuance.Body)
	if err != nil {
		return ResponseProjection[Body]{}, err
	}
	envelope, err := attest.Sign(attest.SignRequest[SigningDomain]{
		Body: commitment, Signer: issuance.Signer,
	})
	if err != nil {
		return ResponseProjection[Body]{}, responseDocumentError(err)
	}
	projection := ResponseProjection[Body]{
		header: issuance.Header, body: issuance.Body, attestation: envelope,
	}
	return projection, projection.Validate()
}

// IssueUpgradeRequiredResponse signs one request-bound refusal without a
// product body. Body selects the response document the client was expecting;
// no value of that type is constructed, encoded, or exposed.
func IssueUpgradeRequiredResponse[Body core.ValidatedJSONMarshaler](
	issuance UpgradeRequiredIssuance,
) (ResponseProjection[Body], error) {
	if err := issuance.Validate(); err != nil {
		return ResponseProjection[Body]{}, err
	}
	commitment, err := responseCommitmentFromRaw(issuance.Header, nil)
	if err != nil {
		return ResponseProjection[Body]{}, err
	}
	envelope, err := attest.Sign(attest.SignRequest[SigningDomain]{
		Body: commitment, Signer: issuance.Signer,
	})
	if err != nil {
		return ResponseProjection[Body]{}, responseDocumentError(err)
	}
	projection := ResponseProjection[Body]{header: issuance.Header, attestation: envelope}
	return projection, projection.Validate()
}

func (p ResponseProjection[Body]) Validate() error {
	commitment, _, err := responseProjectionCommitment(p.header, p.body)
	if err != nil {
		return err
	}
	return validateResponseAttestation(commitment, p.attestation)
}

func (p ResponseProjection[Body]) MarshalJSON() ([]byte, error) {
	commitment, body, err := responseProjectionCommitment(p.header, p.body)
	if err != nil {
		return nil, jsonError(err)
	}
	if err := validateResponseAttestation(commitment, p.attestation); err != nil {
		return nil, jsonError(err)
	}
	encoded, err := core.MarshalCanonicalJSONDocument(responseDocumentWire{
		Header: p.header,
		// doctrine:local-allowed=external-wire
		Body:        json.RawMessage(body),
		Attestation: p.attestation,
	})
	if err != nil || len(encoded) > core.JSONDocumentMaximumBytes {
		return nil, jsonError(responseDocumentError(err))
	}
	return encoded, nil
}

// ValidateJSONProjection proves the exact issue-only bytes through the real
// strict wire shape without making the producer type an external decoder.
// The corresponding ResponseDocument remains the only ingress type.
func (p ResponseProjection[Body]) ValidateJSONProjection(
	encoded []byte,
	limits core.StrictJSONLimits,
) error {
	if err := p.Validate(); err != nil {
		return err
	}
	if _, err := core.DecodeStrictJSONStructure[responseDocumentWire](encoded, limits); err != nil {
		return responseDocumentError(err)
	}
	want, err := p.MarshalJSON()
	if err != nil {
		return err
	}
	if !bytes.Equal(encoded, want) {
		return responseDocumentError(core.ErrJSONContract)
	}
	return nil
}

func (ResponseProjection[Body]) ControlResponseProjection() {}

func (d ResponseDocument[Body, BodyPtr]) Validate() error {
	if !d.set {
		return responseDocumentError()
	}
	if d.commitment.Header.Status != ProductStatusUpgradeRequired {
		if err := BodyPtr(&d.body).Validate(); err != nil {
			return responseDocumentError(err)
		}
	}
	if err := validateResponseBodyPresence(d.commitment.Header, d.commitment.BodyLength); err != nil {
		return responseDocumentError(err)
	}
	return validateResponseAttestation(d.commitment, d.attestation)
}

func (d *ResponseDocument[Body, BodyPtr]) UnmarshalJSON(data []byte) error {
	if d == nil {
		return jsonError(responseDocumentError())
	}
	limits, err := responseJSONLimits(core.JSONDocumentMaximumBytes)
	if err != nil {
		return jsonError(responseDocumentError(err))
	}
	wire, err := core.DecodeStrictJSONStructure[responseDocumentWire](data, limits)
	if err != nil {
		return jsonError(responseDocumentError(err))
	}
	body := BodyPtr(new(Body))
	if wire.Header.Status == ProductStatusUpgradeRequired {
		if len(wire.Body) != 0 {
			return jsonError(responseDocumentError(core.ErrJSONContract))
		}
	} else {
		if len(wire.Body) == 0 {
			return jsonError(responseDocumentError(core.ErrJSONContract))
		}
		if err := body.UnmarshalJSON(wire.Body); err != nil {
			return jsonError(responseDocumentError(err))
		}
	}
	commitment, err := responseCommitmentFromRaw(wire.Header, wire.Body)
	if err != nil {
		return jsonError(err)
	}
	candidate := ResponseDocument[Body, BodyPtr]{
		body: *body, commitment: commitment, attestation: wire.Attestation, set: true,
	}
	if err := candidate.Validate(); err != nil {
		return jsonError(err)
	}
	*d = candidate
	return nil
}

func (ResponseDocument[Body, BodyPtr]) ControlResponseDocument() {}

func (v ResponseVerification[Body, BodyPtr]) Validate() error {
	if err := errors.Join(v.Document.Validate(), v.Expected.Validate(), v.TrustedKeys.Validate()); err != nil {
		return responseDocumentError(err)
	}
	return nil
}

// VerifyResponse authenticates the authority and binds the common header to
// the exact request before it exposes the product body.
func VerifyResponse[
	Body any,
	BodyPtr interface {
		*Body
		core.Validatable
		json.Unmarshaler
	},
](verification ResponseVerification[Body, BodyPtr]) (VerifiedResponse[Body, BodyPtr], error) {
	if err := verification.Validate(); err != nil {
		return VerifiedResponse[Body, BodyPtr]{}, err
	}
	header := verification.Document.commitment.Header
	if err := header.ValidateAgainst(verification.Expected); err != nil {
		return VerifiedResponse[Body, BodyPtr]{}, err
	}
	proof, err := attest.Verify(attest.VerifyRequest[SigningDomain]{
		Body: verification.Document.commitment, Envelope: verification.Document.attestation,
		TrustedKeys: verification.TrustedKeys,
	})
	if err != nil {
		return VerifiedResponse[Body, BodyPtr]{}, responseDocumentError(err)
	}
	verified := VerifiedResponse[Body, BodyPtr]{
		body: verification.Document.body, header: header, proof: proof,
	}
	return verified, verified.Validate()
}

func (v VerifiedResponse[Body, BodyPtr]) Validate() error {
	if err := errors.Join(v.header.Validate(), v.proof.Validate()); err != nil {
		return responseDocumentError(err)
	}
	if v.header.Status != ProductStatusUpgradeRequired {
		if err := BodyPtr(&v.body).Validate(); err != nil {
			return responseDocumentError(err)
		}
	}
	return nil
}

func (v VerifiedResponse[Body, BodyPtr]) Body() (Body, error) {
	if err := v.Validate(); err != nil {
		return *new(Body), err
	}
	if v.header.Status == ProductStatusUpgradeRequired {
		return *new(Body), core.ErrControlPlaneUpgradeRequired
	}
	return v.body, nil
}

func (v VerifiedResponse[Body, BodyPtr]) Header() (ResponseHeader, error) {
	if err := v.Validate(); err != nil {
		return ResponseHeader{}, err
	}
	return v.header, nil
}

func responseCommitmentFor[Body core.ValidatedJSONMarshaler](
	header ResponseHeader,
	body Body,
) (ResponseCommitment, []byte, error) {
	if err := header.Validate(); err != nil {
		return ResponseCommitment{}, nil, responseDocumentError(err)
	}
	encoded, err := core.EncodeValidatedJSON(body, core.DefaultStrictJSONLimits())
	if err != nil {
		return ResponseCommitment{}, nil, responseDocumentError(err)
	}
	commitment, err := responseCommitmentFromRaw(header, encoded)
	return commitment, encoded, err
}

func responseProjectionCommitment[Body core.ValidatedJSONMarshaler](
	header ResponseHeader,
	body Body,
) (ResponseCommitment, []byte, error) {
	if header.Status == ProductStatusUpgradeRequired {
		commitment, err := responseCommitmentFromRaw(header, nil)
		return commitment, nil, err
	}
	return responseCommitmentFor(header, body)
}

func responseCommitmentFromRaw(
	header ResponseHeader,
	body []byte,
) (ResponseCommitment, error) {
	length, err := core.NewByteLength(uint64(len(body)))
	if err != nil {
		return ResponseCommitment{}, responseDocumentError(err)
	}
	commitment := ResponseCommitment{
		Header: header, BodySHA256: core.SHA256Of(body), BodyLength: length,
	}
	return commitment, commitment.Validate()
}

func validateResponseAssessment(
	header ResponseHeader,
	assessment controlwire.ProtocolAssessment,
	want controlwire.ProtocolSupportOutcome,
) error {
	if err := errors.Join(header.Validate(), assessment.Validate(), want.Validate()); err != nil {
		return responseDocumentError(err)
	}
	capability := controlwire.ProtocolCapability{Revision: header.Revision, Family: header.Family}
	if assessment.Capability != capability || assessment.Outcome != want {
		return responseDocumentError(core.ErrControlPlaneResponseBinding)
	}
	if want == controlwire.ProtocolSupportOutcomeUpgradeRequired && header.Status != ProductStatusUpgradeRequired {
		return responseDocumentError(core.ErrControlPlaneDecisionConsistency)
	}
	if want == controlwire.ProtocolSupportOutcomeAccepted && header.Status == ProductStatusUpgradeRequired {
		return responseDocumentError(core.ErrControlPlaneUpgradeRequired)
	}
	return nil
}

func validateResponseBodyPresence(header ResponseHeader, length core.ByteLength) error {
	value := length.Uint64()
	if header.Status == ProductStatusUpgradeRequired && value != 0 {
		return core.ErrControlPlaneUpgradeRequired
	}
	if header.Status != ProductStatusUpgradeRequired && value == 0 {
		return core.ErrControlPlaneResponseDocument
	}
	return nil
}

func validateResponseAttestation(
	commitment ResponseCommitment,
	envelope attest.Envelope[SigningDomain],
) error {
	if err := errors.Join(commitment.Validate(), envelope.Validate()); err != nil {
		return responseDocumentError(err)
	}
	if envelope.Domain != commitment.AttestationDomain() {
		return responseDocumentError(signingDomainError())
	}
	trusted, err := attest.NewTrustedKeys(attest.TrustedKeysRequest{
		Keys: []core.Ed25519PublicKey{envelope.Signer},
	})
	if err != nil {
		return responseDocumentError(err)
	}
	if _, err := attest.Verify(attest.VerifyRequest[SigningDomain]{
		Body: commitment, Envelope: envelope, TrustedKeys: trusted,
	}); err != nil {
		return responseDocumentError(err)
	}
	return nil
}

func responseJSONLimits(maximum uint64) (core.StrictJSONLimits, error) {
	count, err := core.NewByteCount(maximum)
	if err != nil {
		return core.StrictJSONLimits{}, responseDocumentError(err)
	}
	limits := core.DefaultStrictJSONLimits()
	limits.DocumentMaximumBytes = count
	return limits, nil
}

var (
	_ core.Validatable                    = ResponseCommitment{}
	_ core.Validatable                    = UpgradeRequiredIssuance{}
	_ core.ValidatedJSONMarshaler         = ResponseCommitment{}
	_ json.Unmarshaler                    = (*ResponseCommitment)(nil)
	_ attest.CanonicalBody[SigningDomain] = ResponseCommitment{}

	_ controlwire.AuthenticatedResponseProjection = ResponseProjection[ResponseCommitment]{}
	_ controlwire.AuthenticatedResponseDocument   = (*ResponseDocument[ResponseCommitment, *ResponseCommitment])(nil)
)
