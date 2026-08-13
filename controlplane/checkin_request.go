package controlplane

import (
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"io"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/lease"
)

const (
	// CheckInPayloadJSONMaximumBytes bounds the device-signed body.
	CheckInPayloadJSONMaximumBytes = 64 << 10
	// CheckInRequestJSONMaximumBytes bounds a complete check-in request.
	CheckInRequestJSONMaximumBytes = 96 << 10
)

// CheckInPayload is the exact device-signed check-in body.
//
// One shape, every product. Which product is asking is a field on the build
// identity the payload already carries, so an authority reads it the way a
// border agent reads a nationality: after admitting the document, not before,
// and without a second reader for the next product.
type CheckInPayload struct {
	Window            UsageWindow              `json:"window"`
	PreviousWatermark UsageWatermark           `json:"previous_watermark"`
	LeaseGeneration   lease.Generation         `json:"lease_generation"`
	Build             core.BuildIdentity       `json:"build"`
	Revision          controlwire.Revision     `json:"revision"`
	RequestNonce      controlwire.RequestNonce `json:"request_nonce"`
	Installation      lease.DeviceID           `json:"installation"`
	AppliedPolicy     controlwire.PolicyCursor `json:"applied_policy"`
}

type checkInPayloadWire CheckInPayload

// Validate closes every signed fact and proves the payload describes this
// installation and the generation it claims to continue.
//
// The offering is not compared against a constant. The binding re-derives the
// product from the offering the build declares and requires the watermark's
// subject to name that same product, which refuses a mismatched document without
// this package having to know which products exist.
func (p CheckInPayload) Validate() error {
	if err := errors.Join(
		p.RequestNonce.Validate(), p.Installation.Validate(), p.Build.Validate(),
		p.Revision.Validate(), p.LeaseGeneration.Validate(),
		p.PreviousWatermark.Validate(), p.Window.Validate(), p.AppliedPolicy.Validate(),
	); err != nil {
		return checkInError(err)
	}
	if p.PreviousWatermark.Generation != p.LeaseGeneration {
		return consistencyError()
	}
	return p.checkInBinding().Validate()
}

func (p CheckInPayload) checkInBinding() checkInBinding {
	return checkInBinding{
		Build: p.Build, Subject: p.PreviousWatermark.Subject,
		Installation: p.Installation, RequestNonce: p.RequestNonce,
	}
}

// AttestationDomain names the namespace a device signs this payload under.
func (CheckInPayload) AttestationDomain() SigningDomain {
	return SigningDomainCheckInV1
}

// WriteCanonical writes one validated compact payload.
func (p CheckInPayload) WriteCanonical(destination io.Writer) error {
	encoded, err := p.MarshalJSON()
	if err != nil {
		return err
	}
	return writeCanonical(destination, encoded)
}

// MarshalJSON emits one bounded canonical payload.
func (p CheckInPayload) MarshalJSON() ([]byte, error) {
	if err := p.Validate(); err != nil {
		return nil, jsonError(err)
	}
	encoded, err := core.MarshalCanonicalJSONDocument(checkInPayloadWire(p))
	if err != nil || len(encoded) > CheckInPayloadJSONMaximumBytes {
		return nil, jsonError(checkInError(err))
	}
	return encoded, nil
}

// UnmarshalJSON strictly decodes without mutating the receiver on rejection.
func (p *CheckInPayload) UnmarshalJSON(data []byte) error {
	if p == nil {
		return jsonError(checkInError())
	}
	limits, err := documentJSONLimits(CheckInPayloadJSONMaximumBytes)
	if err != nil {
		return jsonError(checkInError(err))
	}
	wire, err := core.DecodeStrictJSONStructure[checkInPayloadWire](data, limits)
	if err != nil {
		return jsonError(checkInError(err))
	}
	candidate := CheckInPayload(wire)
	if err := candidate.Validate(); err != nil {
		return jsonError(err)
	}
	*p = candidate
	return nil
}

// CheckInRequest is the complete body one installation sends: what it did, the
// credential proving it may, and its signature over both.
type CheckInRequest struct {
	Payload     CheckInPayload                  `json:"payload"`
	Certificate InstallationCertificateDocument `json:"certificate"`
	Attestation attest.Envelope[SigningDomain]  `json:"attestation"`
}

type checkInRequestWire CheckInRequest

// CheckInVerification carries the authority's trusted keys into exact credential
// and device-request authentication.
type CheckInVerification struct {
	Request     CheckInRequest
	TrustedKeys attest.TrustedKeys
}

// VerifiedCheckIn is proof that a check-in authenticated. Its fields are
// unexported so it cannot be manufactured without verifying.
type VerifiedCheckIn struct {
	request          CheckInRequest
	certificateProof attest.Verified[SigningDomain]
	requestProof     attest.Verified[SigningDomain]
}

// Validate closes the request and binds its payload to the credential it
// presents.
func (r CheckInRequest) Validate() error {
	if err := r.Payload.Validate(); err != nil {
		return checkInError(err)
	}
	return validateCheckInDocument(checkInDocumentValidation{
		binding: r.Payload.checkInBinding(), certificate: r.Certificate,
		attestation: r.Attestation, domain: r.Payload.AttestationDomain(),
	})
}

// ControlRoute projects the only route this document may address.
func (r CheckInRequest) ControlRoute() (controlwire.RouteContract, error) {
	return controlwire.NewRouteContract(r.Payload.Build.Offering(), controlwire.RouteFamilyCheckIns)
}

// ControlRevision projects the exact signed revision carried by this request.
func (r CheckInRequest) ControlRevision() controlwire.Revision { return r.Payload.Revision }

// ControlNonce projects the request identity already carried in the signed payload.
func (r CheckInRequest) ControlNonce() controlwire.RequestNonce { return r.Payload.RequestNonce }

func (CheckInRequest) ControlRequestBodyLimit() (core.ByteCount, error) {
	return core.NewByteCount(CheckInRequestJSONMaximumBytes)
}

// MarshalJSON emits one bounded canonical request.
func (r CheckInRequest) MarshalJSON() ([]byte, error) {
	if err := r.Validate(); err != nil {
		return nil, jsonError(err)
	}
	encoded, err := core.MarshalCanonicalJSONDocument(checkInRequestWire(r))
	if err != nil || len(encoded) > CheckInRequestJSONMaximumBytes {
		return nil, jsonError(checkInError(err))
	}
	return encoded, nil
}

// UnmarshalJSON strictly decodes without mutating the receiver on rejection.
func (r *CheckInRequest) UnmarshalJSON(data []byte) error {
	if r == nil {
		return jsonError(checkInError())
	}
	limits, err := documentJSONLimits(CheckInRequestJSONMaximumBytes)
	if err != nil {
		return jsonError(checkInError(err))
	}
	wire, err := core.DecodeStrictJSONStructure[checkInRequestWire](data, limits)
	if err != nil {
		return jsonError(checkInError(err))
	}
	candidate := CheckInRequest(wire)
	if err := candidate.Validate(); err != nil {
		return jsonError(err)
	}
	*r = candidate
	return nil
}

// IssueCheckIn signs one validated payload with the device key and attaches the
// credential the authority issued for that device.
func IssueCheckIn(
	payload CheckInPayload,
	key ed25519.PrivateKey,
	certificate InstallationCertificateDocument,
) (CheckInRequest, error) {
	if err := errors.Join(payload.Validate(), certificate.Validate()); err != nil {
		return CheckInRequest{}, checkInError(err)
	}
	envelope, err := attest.Sign(attest.SignRequest[SigningDomain]{Body: payload, Signer: key})
	if err != nil {
		return CheckInRequest{}, checkInError(err)
	}
	request := CheckInRequest{Payload: payload, Certificate: certificate, Attestation: envelope}
	return request, request.Validate()
}

// Validate closes the complete verification input.
func (v CheckInVerification) Validate() error {
	if err := errors.Join(v.Request.Validate(), v.TrustedKeys.Validate()); err != nil {
		return checkInError(err)
	}
	return nil
}

// VerifyCheckIn authenticates the authority-issued credential first, then uses
// the exact device key it names as the sole authority for the request.
func VerifyCheckIn(verification CheckInVerification) (VerifiedCheckIn, error) {
	if err := verification.Validate(); err != nil {
		return VerifiedCheckIn{}, err
	}
	certificateProof, deviceKeys, err := verifyCheckInCertificate(
		verification.Request.Certificate, verification.TrustedKeys,
	)
	if err != nil {
		return VerifiedCheckIn{}, err
	}
	requestProof, err := attest.Verify(attest.VerifyRequest[SigningDomain]{
		Body:        verification.Request.Payload,
		Envelope:    verification.Request.Attestation,
		TrustedKeys: deviceKeys,
	})
	if err != nil {
		return VerifiedCheckIn{}, checkInError(err)
	}
	result := VerifiedCheckIn{
		request: verification.Request, certificateProof: certificateProof, requestProof: requestProof,
	}
	return result, result.Validate()
}

// Validate revalidates every proof the type claims to hold.
func (v VerifiedCheckIn) Validate() error {
	return validateCheckInProofs(v.request.Validate(), v.certificateProof, v.requestProof)
}

// Request returns the authenticated request, revalidating first.
func (v VerifiedCheckIn) Request() (CheckInRequest, error) {
	if err := v.Validate(); err != nil {
		return CheckInRequest{}, err
	}
	return v.request, nil
}

var (
	_ controlwire.RoutedJSONRequest = CheckInRequest{}
	_ core.Validatable              = CheckInPayload{}
	_ core.Validatable              = CheckInRequest{}
	_ core.Validatable              = CheckInVerification{}
	_ core.Validatable              = VerifiedCheckIn{}

	_ core.ValidatedJSONMarshaler = CheckInPayload{}
	_ core.ValidatedJSONMarshaler = CheckInRequest{}

	_ json.Unmarshaler = (*CheckInPayload)(nil)
	_ json.Unmarshaler = (*CheckInRequest)(nil)

	_ attest.CanonicalBody[SigningDomain] = CheckInPayload{}
)
