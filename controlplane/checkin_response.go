package controlplane

import (
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"io"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/lease"
)

const (
	// CheckInResponsePayloadJSONMaximumBytes bounds an accepted response payload.
	CheckInResponsePayloadJSONMaximumBytes = 48 << 10
	// CheckInResponseDocumentJSONMaximumBytes bounds a complete signed response.
	CheckInResponseDocumentJSONMaximumBytes = 64 << 10

	// UsageDispositionAcceptedToken names a window committed for the first time.
	UsageDispositionAcceptedToken = "accepted"
	// UsageDispositionReplayToken names a window already committed.
	UsageDispositionReplayToken = "replay"
	// UsageDispositionConflictToken names a window refused against the current
	// watermark.
	UsageDispositionConflictToken = "conflict"
)

// UsageDisposition is the authority-owned result of attempting to commit one
// immutable usage window.
//
// Replay and conflict are distinct on purpose. Replay is the same window
// arriving twice and is not an error: a client that lost the response must be
// able to retry without being told its accounting is wrong. Conflict is a
// different window presented against a watermark that has already moved, and
// the authority never advances on it.
type UsageDisposition uint8

const (
	// UsageDispositionUnknown is the unset disposition and names no result.
	UsageDispositionUnknown UsageDisposition = iota
	// UsageDispositionAccepted commits the window and advances the watermark.
	UsageDispositionAccepted
	// UsageDispositionReplay returns the watermark the original acceptance set.
	UsageDispositionReplay
	// UsageDispositionConflict returns the current watermark and advances nothing.
	UsageDispositionConflict
	usageDispositionLimit
)

func usageDispositionTokens() [usageDispositionLimit]string {
	return [...]string{
		UsageDispositionUnknown:  "",
		UsageDispositionAccepted: UsageDispositionAcceptedToken,
		UsageDispositionReplay:   UsageDispositionReplayToken,
		UsageDispositionConflict: UsageDispositionConflictToken,
	}
}

// Validate rejects the unset disposition and every disposition outside the set.
func (d UsageDisposition) Validate() error {
	if d <= UsageDispositionUnknown || d >= usageDispositionLimit || usageDispositionTokens()[d] == "" {
		return checkInResponseError()
	}
	return nil
}

// IsValid reports whether d is a published commit result.
func (d UsageDisposition) IsValid() bool { return d.Validate() == nil }

// String returns the canonical token, or empty text when unset.
func (d UsageDisposition) String() string {
	if d >= usageDispositionLimit {
		return ""
	}
	return usageDispositionTokens()[d]
}

// AdvancesWatermark reports whether the authority moved the watermark.
//
// Only an acceptance does. A client that advanced its own watermark on replay
// or conflict would report its next window against a generation the authority
// never issued, and every later check-in would conflict.
func (d UsageDisposition) AdvancesWatermark() bool { return d == UsageDispositionAccepted }

// ParseUsageDisposition accepts one exact published token.
func ParseUsageDisposition(value string) (UsageDisposition, error) {
	tokens := usageDispositionTokens()
	for candidate := UsageDispositionUnknown + 1; candidate < usageDispositionLimit; candidate++ {
		if tokens[candidate] != "" && tokens[candidate] == value {
			return candidate, nil
		}
	}
	return UsageDispositionUnknown, checkInResponseError()
}

// MarshalJSON emits the canonical token and refuses an unset disposition.
func (d UsageDisposition) MarshalJSON() ([]byte, error) {
	if err := d.Validate(); err != nil {
		return nil, jsonError(err)
	}
	encoded, err := core.MarshalCanonicalJSONString(d.String())
	if err != nil {
		return nil, jsonError(checkInResponseError(err))
	}
	return encoded, nil
}

// UnmarshalJSON accepts only a published token and leaves d unchanged on every
// rejection.
func (d *UsageDisposition) UnmarshalJSON(data []byte) error {
	if d == nil {
		return jsonError(checkInResponseError())
	}
	token, err := core.DecodeJSONStringToken(data)
	if err != nil {
		return jsonError(checkInResponseError(err))
	}
	parsed, err := ParseUsageDisposition(token)
	if err != nil {
		return jsonError(err)
	}
	*d = parsed
	return nil
}

// CheckInResponsePayload is the complete signed authority result for one
// check-in.
type CheckInResponsePayload struct {
	Header      ResponseHeader   `json:"header"`
	Disposition UsageDisposition `json:"disposition"`
	Watermark   UsageWatermark   `json:"watermark"`
	Lease       lease.Document   `json:"lease"`
}

// CheckInResponseDocument is the response payload with its authority signature.
type CheckInResponseDocument struct {
	Payload     CheckInResponsePayload         `json:"payload"`
	Attestation attest.Envelope[SigningDomain] `json:"attestation"`
}

// CheckInResponseVerification is the complete input one caller supplies to
// authenticate a response against the request that produced it.
type CheckInResponseVerification struct {
	Document    CheckInResponseDocument
	Expected    ResponseExpectation
	TrustedKeys attest.TrustedKeys
}

// VerifiedCheckInResponse is proof that a response authenticated. Its fields
// are unexported so the type cannot be manufactured by a caller that skipped
// verification.
type VerifiedCheckInResponse struct {
	payload       CheckInResponsePayload
	responseProof attest.Verified[SigningDomain]
	leaseProof    lease.Verified
}

type (
	checkInResponsePayloadWire  CheckInResponsePayload
	checkInResponseDocumentWire CheckInResponseDocument
)

// Validate closes the payload and proves its signed facts agree with each
// other.
//
// The lease decision and the header are signed together, so a document whose
// halves disagree is either an authority defect or a forgery assembled from two
// real responses. Either way it is refused rather than acted on in whichever
// half the reader happens to consult.
func (p CheckInResponsePayload) Validate() error {
	if err := errors.Join(
		p.Header.Validate(), p.Disposition.Validate(),
		p.Watermark.Validate(), p.Lease.Validate(),
	); err != nil {
		return checkInResponseError(err)
	}
	if err := p.validateDecisionAgreement(); err != nil {
		return err
	}
	return p.Header.Status.ValidateOutcome(p.Header.Offering, p.Lease.Decision.Outcome())
}

func (p CheckInResponsePayload) validateDecisionAgreement() error {
	header, err := p.Lease.Decision.Header()
	if err != nil {
		return checkInResponseError(err)
	}
	product, err := lease.ProductForOffering(p.Header.Offering)
	if err != nil || header.Subject.Product != product ||
		header.Subject != p.Watermark.Subject ||
		header.Subject.DeviceID != p.Header.Installation ||
		header.Generation != p.Watermark.Generation ||
		header.IssuedAt != p.Header.ProviderTime {
		return consistencyError(err)
	}
	return nil
}

// AttestationDomain names the namespace an authority signs this payload under.
func (CheckInResponsePayload) AttestationDomain() SigningDomain {
	return SigningDomainCheckInResponseV1
}

// WriteCanonical writes one validated compact response payload.
func (p CheckInResponsePayload) WriteCanonical(destination io.Writer) error {
	encoded, err := p.MarshalJSON()
	if err != nil {
		return err
	}
	return writeCanonical(destination, encoded)
}

// MarshalJSON emits one bounded canonical payload.
func (p CheckInResponsePayload) MarshalJSON() ([]byte, error) {
	if err := p.Validate(); err != nil {
		return nil, jsonError(err)
	}
	encoded, err := core.MarshalCanonicalJSONDocument(checkInResponsePayloadWire(p))
	if err != nil || len(encoded) > CheckInResponsePayloadJSONMaximumBytes {
		return nil, jsonError(checkInResponseError(err))
	}
	return encoded, nil
}

// UnmarshalJSON strictly decodes without mutating the receiver on rejection.
func (p *CheckInResponsePayload) UnmarshalJSON(data []byte) error {
	if p == nil {
		return jsonError(checkInResponseError())
	}
	limits, err := documentJSONLimits(CheckInResponsePayloadJSONMaximumBytes)
	if err != nil {
		return jsonError(checkInResponseError(err))
	}
	wire, err := core.DecodeStrictJSONStructure[checkInResponsePayloadWire](data, limits)
	if err != nil {
		return jsonError(checkInResponseError(err))
	}
	candidate := CheckInResponsePayload(wire)
	if err := candidate.Validate(); err != nil {
		return jsonError(err)
	}
	*p = candidate
	return nil
}

// Validate closes the signed response and binds its envelope to this payload's
// own domain.
func (d CheckInResponseDocument) Validate() error {
	if err := errors.Join(d.Payload.Validate(), d.Attestation.Validate()); err != nil {
		return checkInResponseError(err)
	}
	if d.Attestation.Domain != d.Payload.AttestationDomain() {
		return signingDomainError()
	}
	return nil
}

// MarshalJSON emits one bounded canonical signed response.
func (d CheckInResponseDocument) MarshalJSON() ([]byte, error) {
	if err := d.Validate(); err != nil {
		return nil, jsonError(err)
	}
	encoded, err := core.MarshalCanonicalJSONDocument(checkInResponseDocumentWire(d))
	if err != nil || len(encoded) > CheckInResponseDocumentJSONMaximumBytes {
		return nil, jsonError(checkInResponseError(err))
	}
	return encoded, nil
}

// UnmarshalJSON strictly decodes without mutating the receiver on rejection.
func (d *CheckInResponseDocument) UnmarshalJSON(data []byte) error {
	if d == nil {
		return jsonError(checkInResponseError())
	}
	limits, err := documentJSONLimits(CheckInResponseDocumentJSONMaximumBytes)
	if err != nil {
		return jsonError(checkInResponseError(err))
	}
	wire, err := core.DecodeStrictJSONStructure[checkInResponseDocumentWire](data, limits)
	if err != nil {
		return jsonError(checkInResponseError(err))
	}
	candidate := CheckInResponseDocument(wire)
	if err := candidate.Validate(); err != nil {
		return jsonError(err)
	}
	*d = candidate
	return nil
}

// IssueCheckInResponse signs one validated response payload. It is the
// authority half of the exchange and is exercised by every client test, so the
// bytes a client verifies are produced by the same code a server runs.
func IssueCheckInResponse(payload CheckInResponsePayload, key ed25519.PrivateKey) (CheckInResponseDocument, error) {
	if err := payload.Validate(); err != nil {
		return CheckInResponseDocument{}, err
	}
	envelope, err := attest.Sign(attest.SignRequest[SigningDomain]{Body: payload, Signer: key})
	if err != nil {
		return CheckInResponseDocument{}, checkInResponseError(err)
	}
	document := CheckInResponseDocument{Payload: payload, Attestation: envelope}
	return document, document.Validate()
}

// Validate closes the complete verification input.
func (v CheckInResponseVerification) Validate() error {
	if err := errors.Join(
		v.Document.Validate(), v.Expected.Validate(), v.TrustedKeys.Validate(),
	); err != nil {
		return checkInResponseError(err)
	}
	return nil
}

// VerifyCheckInResponse authenticates a response and binds it to the exact
// request that produced it.
//
// The binding check runs before the signature check on purpose. A correctly
// signed response to somebody else's request is the interesting attack, and
// naming that failure as a binding error rather than a signature error is what
// lets a caller tell a replayed response from a forged one.
func VerifyCheckInResponse(verification CheckInResponseVerification) (VerifiedCheckInResponse, error) {
	if err := verification.Validate(); err != nil {
		return VerifiedCheckInResponse{}, err
	}
	if err := verification.Document.Payload.Header.ValidateAgainst(verification.Expected); err != nil {
		return VerifiedCheckInResponse{}, err
	}
	responseProof, err := attest.Verify(attest.VerifyRequest[SigningDomain]{
		Body:        verification.Document.Payload,
		Envelope:    verification.Document.Attestation,
		TrustedKeys: verification.TrustedKeys,
	})
	if err != nil {
		return VerifiedCheckInResponse{}, checkInResponseError(err)
	}
	leaseProof, err := verifyCheckInResponseLease(verification.Document.Payload, verification.TrustedKeys)
	if err != nil {
		return VerifiedCheckInResponse{}, err
	}
	verified := VerifiedCheckInResponse{
		payload: verification.Document.Payload, responseProof: responseProof, leaseProof: leaseProof,
	}
	return verified, verified.Validate()
}

func verifyCheckInResponseLease(payload CheckInResponsePayload, trusted attest.TrustedKeys) (lease.Verified, error) {
	verified, err := lease.Verify(lease.VerifyRequest{
		Document: payload.Lease, TrustedKeys: trusted, ExpectedSubject: payload.Watermark.Subject,
	})
	if err != nil {
		return lease.Verified{}, checkInResponseError(err)
	}
	return verified, nil
}

// Validate revalidates every fact the proof claims to hold.
func (v VerifiedCheckInResponse) Validate() error {
	if err := errors.Join(
		v.payload.Validate(), v.responseProof.Validate(), v.leaseProof.Validate(),
	); err != nil {
		return checkInResponseError(err)
	}
	return nil
}

// Payload returns the authenticated payload, revalidating first so a zero value
// cannot be presented as a verified one.
func (v VerifiedCheckInResponse) Payload() (CheckInResponsePayload, error) {
	if err := v.Validate(); err != nil {
		return CheckInResponsePayload{}, err
	}
	return v.payload, nil
}

// Lease returns the authenticated lease proof.
func (v VerifiedCheckInResponse) Lease() (lease.Verified, error) {
	if err := v.Validate(); err != nil {
		return lease.Verified{}, err
	}
	return v.leaseProof, nil
}

var (
	_ core.Validatable = UsageDisposition(UsageDispositionUnknown)
	_ core.Validatable = CheckInResponsePayload{}
	_ core.Validatable = CheckInResponseDocument{}
	_ core.Validatable = CheckInResponseVerification{}
	_ core.Validatable = VerifiedCheckInResponse{}

	_ core.ValidatedJSONMarshaler = UsageDisposition(UsageDispositionUnknown)
	_ core.ValidatedJSONMarshaler = CheckInResponsePayload{}
	_ core.ValidatedJSONMarshaler = CheckInResponseDocument{}

	_ json.Unmarshaler = (*UsageDisposition)(nil)
	_ json.Unmarshaler = (*CheckInResponsePayload)(nil)
	_ json.Unmarshaler = (*CheckInResponseDocument)(nil)

	_ attest.CanonicalBody[SigningDomain] = CheckInResponsePayload{}
)
