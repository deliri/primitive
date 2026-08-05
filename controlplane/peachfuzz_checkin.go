package controlplane

import (
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"io"
	"math"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/lease"
	"github.com/deliri/primitive/v2026/temporal"
)

const (
	// PeachfuzzUsageWindowJSONMaximumBytes bounds one reported usage window.
	PeachfuzzUsageWindowJSONMaximumBytes = 32 << 10
	// PeachfuzzCheckInPayloadJSONMaximumBytes bounds the device-signed body.
	PeachfuzzCheckInPayloadJSONMaximumBytes = 64 << 10
	// PeachfuzzCheckInRequestJSONMaximumBytes bounds a complete check-in request.
	PeachfuzzCheckInRequestJSONMaximumBytes = 96 << 10
	// PeachfuzzUsageCountMaximumEntries bounds either count list. The categories
	// are closed, so a longer list is a repetition or a forgery rather than a
	// larger truth.
	PeachfuzzUsageCountMaximumEntries = 16
)

// These are the exact command category tokens a usage window may report.
const (
	PeachfuzzCommandRunToken       = "run"
	PeachfuzzCommandOnceToken      = "once"
	PeachfuzzCommandDiscoverToken  = "discover"
	PeachfuzzCommandReportToken    = "report"
	PeachfuzzCommandReproduceToken = "reproduce"
	PeachfuzzCommandUnitToken      = "unit"
	PeachfuzzCommandActivityToken  = "activity"
)

// These are the exact outcome category tokens a usage window may report.
const (
	PeachfuzzOutcomeCompletedToken           = "completed"
	PeachfuzzOutcomeCandidateFoundToken      = "candidate-found"
	PeachfuzzOutcomeSeedFailureToken         = "seed-failure"
	PeachfuzzOutcomeBuildFailedToken         = "build-failed"
	PeachfuzzOutcomeTimedOutToken            = "timed-out"
	PeachfuzzOutcomeInterruptedToken         = "interrupted"
	PeachfuzzOutcomeInfrastructureErrorToken = "infrastructure-error"
	PeachfuzzOutcomeUnsupportedToken         = "unsupported"
)

// PeachfuzzCommand is the closed set of command categories usage may report.
//
// It is a category, not a command line. What the customer fuzzed, what it was
// called, and what it found never leave the machine, so this names only which
// kind of work ran.
type PeachfuzzCommand uint8

const (
	// PeachfuzzCommandUnknown is the unset category and reports nothing.
	PeachfuzzCommandUnknown PeachfuzzCommand = iota
	PeachfuzzCommandRun
	PeachfuzzCommandOnce
	PeachfuzzCommandDiscover
	PeachfuzzCommandReport
	PeachfuzzCommandReproduce
	PeachfuzzCommandUnit
	PeachfuzzCommandActivity
	peachfuzzCommandLimit
)

func peachfuzzCommandTokens() [peachfuzzCommandLimit]string {
	return [...]string{
		PeachfuzzCommandUnknown:   "",
		PeachfuzzCommandRun:       PeachfuzzCommandRunToken,
		PeachfuzzCommandOnce:      PeachfuzzCommandOnceToken,
		PeachfuzzCommandDiscover:  PeachfuzzCommandDiscoverToken,
		PeachfuzzCommandReport:    PeachfuzzCommandReportToken,
		PeachfuzzCommandReproduce: PeachfuzzCommandReproduceToken,
		PeachfuzzCommandUnit:      PeachfuzzCommandUnitToken,
		PeachfuzzCommandActivity:  PeachfuzzCommandActivityToken,
	}
}

// Validate rejects the unset category and every category outside the set.
func (c PeachfuzzCommand) Validate() error {
	if c <= PeachfuzzCommandUnknown || c >= peachfuzzCommandLimit || peachfuzzCommandTokens()[c] == "" {
		return usageWindowError()
	}
	return nil
}

// IsValid reports whether c is a published command category.
func (c PeachfuzzCommand) IsValid() bool { return c.Validate() == nil }

// String returns the canonical token, or empty text when unset.
func (c PeachfuzzCommand) String() string {
	if c >= peachfuzzCommandLimit {
		return ""
	}
	return peachfuzzCommandTokens()[c]
}

// ParsePeachfuzzCommand accepts one exact published token.
func ParsePeachfuzzCommand(value string) (PeachfuzzCommand, error) {
	tokens := peachfuzzCommandTokens()
	for candidate := PeachfuzzCommandUnknown + 1; candidate < peachfuzzCommandLimit; candidate++ {
		if tokens[candidate] != "" && tokens[candidate] == value {
			return candidate, nil
		}
	}
	return PeachfuzzCommandUnknown, usageWindowError()
}

// MarshalJSON emits the canonical token and refuses an unset category.
func (c PeachfuzzCommand) MarshalJSON() ([]byte, error) {
	if err := c.Validate(); err != nil {
		return nil, jsonError(err)
	}
	encoded, err := core.MarshalCanonicalJSONString(c.String())
	if err != nil {
		return nil, jsonError(usageWindowError(err))
	}
	return encoded, nil
}

// UnmarshalJSON accepts only a published token and leaves c unchanged on every
// rejection.
func (c *PeachfuzzCommand) UnmarshalJSON(data []byte) error {
	if c == nil {
		return jsonError(usageWindowError())
	}
	token, err := core.DecodeJSONStringToken(data)
	if err != nil {
		return jsonError(usageWindowError(err))
	}
	parsed, err := ParsePeachfuzzCommand(token)
	if err != nil {
		return jsonError(err)
	}
	*c = parsed
	return nil
}

// PeachfuzzOutcome is the closed set of result classifications usage may
// report. Like the command category it names a kind, never a finding.
type PeachfuzzOutcome uint8

const (
	// PeachfuzzOutcomeUnknown is the unset classification and reports nothing.
	PeachfuzzOutcomeUnknown PeachfuzzOutcome = iota
	PeachfuzzOutcomeCompleted
	PeachfuzzOutcomeCandidateFound
	PeachfuzzOutcomeSeedFailure
	PeachfuzzOutcomeBuildFailed
	PeachfuzzOutcomeTimedOut
	PeachfuzzOutcomeInterrupted
	PeachfuzzOutcomeInfrastructureError
	PeachfuzzOutcomeUnsupported
	peachfuzzOutcomeLimit
)

func peachfuzzOutcomeTokens() [peachfuzzOutcomeLimit]string {
	return [...]string{
		PeachfuzzOutcomeUnknown:             "",
		PeachfuzzOutcomeCompleted:           PeachfuzzOutcomeCompletedToken,
		PeachfuzzOutcomeCandidateFound:      PeachfuzzOutcomeCandidateFoundToken,
		PeachfuzzOutcomeSeedFailure:         PeachfuzzOutcomeSeedFailureToken,
		PeachfuzzOutcomeBuildFailed:         PeachfuzzOutcomeBuildFailedToken,
		PeachfuzzOutcomeTimedOut:            PeachfuzzOutcomeTimedOutToken,
		PeachfuzzOutcomeInterrupted:         PeachfuzzOutcomeInterruptedToken,
		PeachfuzzOutcomeInfrastructureError: PeachfuzzOutcomeInfrastructureErrorToken,
		PeachfuzzOutcomeUnsupported:         PeachfuzzOutcomeUnsupportedToken,
	}
}

// Validate rejects the unset classification and every one outside the set.
func (o PeachfuzzOutcome) Validate() error {
	if o <= PeachfuzzOutcomeUnknown || o >= peachfuzzOutcomeLimit || peachfuzzOutcomeTokens()[o] == "" {
		return usageWindowError()
	}
	return nil
}

// IsValid reports whether o is a published outcome classification.
func (o PeachfuzzOutcome) IsValid() bool { return o.Validate() == nil }

// String returns the canonical token, or empty text when unset.
func (o PeachfuzzOutcome) String() string {
	if o >= peachfuzzOutcomeLimit {
		return ""
	}
	return peachfuzzOutcomeTokens()[o]
}

// ParsePeachfuzzOutcome accepts one exact published token.
func ParsePeachfuzzOutcome(value string) (PeachfuzzOutcome, error) {
	tokens := peachfuzzOutcomeTokens()
	for candidate := PeachfuzzOutcomeUnknown + 1; candidate < peachfuzzOutcomeLimit; candidate++ {
		if tokens[candidate] != "" && tokens[candidate] == value {
			return candidate, nil
		}
	}
	return PeachfuzzOutcomeUnknown, usageWindowError()
}

// MarshalJSON emits the canonical token and refuses an unset classification.
func (o PeachfuzzOutcome) MarshalJSON() ([]byte, error) {
	if err := o.Validate(); err != nil {
		return nil, jsonError(err)
	}
	encoded, err := core.MarshalCanonicalJSONString(o.String())
	if err != nil {
		return nil, jsonError(usageWindowError(err))
	}
	return encoded, nil
}

// UnmarshalJSON accepts only a published token and leaves o unchanged on every
// rejection.
func (o *PeachfuzzOutcome) UnmarshalJSON(data []byte) error {
	if o == nil {
		return jsonError(usageWindowError())
	}
	token, err := core.DecodeJSONStringToken(data)
	if err != nil {
		return jsonError(usageWindowError(err))
	}
	parsed, err := ParsePeachfuzzOutcome(token)
	if err != nil {
		return jsonError(err)
	}
	*o = parsed
	return nil
}

// PeachfuzzCommandCount is one command category and how often it ran.
type PeachfuzzCommandCount struct {
	Command PeachfuzzCommand `json:"command"`
	Count   uint64           `json:"count"`
}

// Validate rejects an unknown category and a zero count. A category that ran
// zero times is simply absent from the list rather than reported as nothing.
func (c PeachfuzzCommandCount) Validate() error {
	if err := c.Command.Validate(); err != nil {
		return usageWindowError(err)
	}
	if c.Count == 0 {
		return usageWindowError()
	}
	return nil
}

// PeachfuzzOutcomeCount is one outcome classification and how often it occurred.
type PeachfuzzOutcomeCount struct {
	Outcome PeachfuzzOutcome `json:"outcome"`
	Count   uint64           `json:"count"`
}

// Validate rejects an unknown classification and a zero count.
func (c PeachfuzzOutcomeCount) Validate() error {
	if err := c.Outcome.Validate(); err != nil {
		return usageWindowError(err)
	}
	if c.Count == 0 {
		return usageWindowError()
	}
	return nil
}

// PeachfuzzUsageWindow is the complete bounded aggregate one installation
// reports for one interval.
//
// It carries only counts and durations. There is no name, path, source, input,
// output, or finding anywhere in it, and there is no field one could be smuggled
// through, because the type is closed and every member is a number or an
// instant. That is the agreement with the customer expressed as a struct rather
// than as a promise.
type PeachfuzzUsageWindow struct {
	Commands           []PeachfuzzCommandCount `json:"commands"`
	Outcomes           []PeachfuzzOutcomeCount `json:"outcomes"`
	Bounds             temporal.IntervalBounds `json:"bounds"`
	Freshness          temporal.Instant        `json:"freshness"`
	Slices             uint64                  `json:"slices"`
	CPU                temporal.Duration       `json:"cpu"`
	Execution          temporal.Duration       `json:"execution"`
	Targets            uint64                  `json:"target_count"`
	CandidateSightings uint64                  `json:"candidate_sightings"`
	Candidates         uint64                  `json:"candidate_count"`
	Artifacts          uint64                  `json:"artifact_count"`
	Receipts           uint64                  `json:"receipt_count"`
	QueueDepth         uint64                  `json:"queue_depth"`
	BacklogDepth       uint64                  `json:"backlog_depth"`
}

type peachfuzzUsageWindowWire PeachfuzzUsageWindow

// Validate closes every reported fact and every relationship between them.
func (w PeachfuzzUsageWindow) Validate() error {
	if err := errors.Join(
		w.Bounds.Validate(), w.Freshness.Validate(),
		w.Execution.Validate(), w.CPU.Validate(),
	); err != nil {
		return usageWindowError(err)
	}
	if err := validateUsageFreshness(w.Bounds, w.Freshness); err != nil {
		return err
	}
	if err := validatePeachfuzzCommandCounts(w.Commands); err != nil {
		return err
	}
	if err := validatePeachfuzzOutcomeCounts(w.Outcomes, w.Slices); err != nil {
		return err
	}
	return w.validateAggregateAgreement()
}

// validateAggregateAgreement rejects totals that contradict each other.
//
// More distinct candidates than sightings of one, or work with nothing to work
// on, describe no run that could have happened. An authority that accepted them
// would be billing against arithmetic nobody can reproduce.
func (w PeachfuzzUsageWindow) validateAggregateAgreement() error {
	if w.Candidates > w.CandidateSightings {
		return usageWindowError()
	}
	if w.Slices > 0 && w.Targets == 0 {
		return usageWindowError()
	}
	return nil
}

// validatePeachfuzzCommandCounts bounds the list and requires strictly
// ascending categories, which makes a repeated category a decode failure rather
// than a silent double count.
func validatePeachfuzzCommandCounts(counts []PeachfuzzCommandCount) error {
	if len(counts) > PeachfuzzUsageCountMaximumEntries {
		return usageWindowError()
	}
	for index, count := range counts {
		if err := count.Validate(); err != nil {
			return err
		}
		if index > 0 && counts[index-1].Command >= count.Command {
			return usageWindowError()
		}
	}
	return nil
}

// validatePeachfuzzOutcomeCounts additionally proves the classifications
// account for exactly the slices claimed, so no slice is unclassified and none
// is counted twice.
func validatePeachfuzzOutcomeCounts(counts []PeachfuzzOutcomeCount, slices uint64) error {
	if len(counts) > PeachfuzzUsageCountMaximumEntries {
		return usageWindowError()
	}
	var total uint64
	for index, count := range counts {
		if err := count.Validate(); err != nil {
			return err
		}
		if index > 0 && counts[index-1].Outcome >= count.Outcome {
			return usageWindowError()
		}
		if math.MaxUint64-total < count.Count {
			return usageWindowError()
		}
		total += count.Count
	}
	if total != slices {
		return usageWindowError()
	}
	return nil
}

// MarshalJSON emits one bounded canonical usage window.
func (w PeachfuzzUsageWindow) MarshalJSON() ([]byte, error) {
	if err := w.Validate(); err != nil {
		return nil, jsonError(err)
	}
	encoded, err := core.MarshalCanonicalJSONDocument(peachfuzzUsageWindowWire(w))
	if err != nil || len(encoded) > PeachfuzzUsageWindowJSONMaximumBytes {
		return nil, jsonError(usageWindowError(err))
	}
	return encoded, nil
}

// UnmarshalJSON strictly decodes without mutating the receiver on rejection.
func (w *PeachfuzzUsageWindow) UnmarshalJSON(data []byte) error {
	if w == nil {
		return jsonError(usageWindowError())
	}
	limits, err := documentJSONLimits(PeachfuzzUsageWindowJSONMaximumBytes)
	if err != nil {
		return jsonError(usageWindowError(err))
	}
	wire, err := core.DecodeStrictJSONStructure[peachfuzzUsageWindowWire](data, limits)
	if err != nil {
		return jsonError(usageWindowError(err))
	}
	candidate := PeachfuzzUsageWindow(wire)
	if err := candidate.Validate(); err != nil {
		return jsonError(err)
	}
	*w = candidate
	return nil
}

// PeachfuzzCheckInPayload is the exact device-signed check-in body.
type PeachfuzzCheckInPayload struct {
	Window            PeachfuzzUsageWindow     `json:"window"`
	PreviousWatermark UsageWatermark           `json:"previous_watermark"`
	LeaseGeneration   lease.Generation         `json:"lease_generation"`
	Build             core.BuildIdentity       `json:"build"`
	Revision          controlwire.Revision     `json:"revision"`
	RequestNonce      controlwire.RequestNonce `json:"request_nonce"`
	Installation      lease.DeviceID           `json:"installation"`
	AppliedPolicy     controlwire.PolicyCursor `json:"applied_policy"`
}

type peachfuzzCheckInPayloadWire PeachfuzzCheckInPayload

// Validate closes every signed fact and proves the payload describes this
// offering, this installation, and the generation it claims to continue.
func (p PeachfuzzCheckInPayload) Validate() error {
	if err := errors.Join(
		p.RequestNonce.Validate(), p.Installation.Validate(), p.Build.Validate(),
		p.Revision.Validate(), p.LeaseGeneration.Validate(),
		p.PreviousWatermark.Validate(), p.Window.Validate(), p.AppliedPolicy.Validate(),
	); err != nil {
		return checkInError(err)
	}
	if p.Build.Offering() != core.OfferingPeachfuzz ||
		p.PreviousWatermark.Generation != p.LeaseGeneration {
		return consistencyError()
	}
	return p.checkInBinding().Validate()
}

func (p PeachfuzzCheckInPayload) checkInBinding() checkInBinding {
	return checkInBinding{
		Build: p.Build, Subject: p.PreviousWatermark.Subject,
		Installation: p.Installation, RequestNonce: p.RequestNonce,
	}
}

// AttestationDomain names the namespace a device signs this payload under.
func (PeachfuzzCheckInPayload) AttestationDomain() SigningDomain {
	return SigningDomainCheckInV1
}

// WriteCanonical writes one validated compact payload.
func (p PeachfuzzCheckInPayload) WriteCanonical(destination io.Writer) error {
	encoded, err := p.MarshalJSON()
	if err != nil {
		return err
	}
	return writeCanonical(destination, encoded)
}

// MarshalJSON emits one bounded canonical payload.
func (p PeachfuzzCheckInPayload) MarshalJSON() ([]byte, error) {
	if err := p.Validate(); err != nil {
		return nil, jsonError(err)
	}
	encoded, err := core.MarshalCanonicalJSONDocument(peachfuzzCheckInPayloadWire(p))
	if err != nil || len(encoded) > PeachfuzzCheckInPayloadJSONMaximumBytes {
		return nil, jsonError(checkInError(err))
	}
	return encoded, nil
}

// UnmarshalJSON strictly decodes without mutating the receiver on rejection.
func (p *PeachfuzzCheckInPayload) UnmarshalJSON(data []byte) error {
	if p == nil {
		return jsonError(checkInError())
	}
	limits, err := documentJSONLimits(PeachfuzzCheckInPayloadJSONMaximumBytes)
	if err != nil {
		return jsonError(checkInError(err))
	}
	wire, err := core.DecodeStrictJSONStructure[peachfuzzCheckInPayloadWire](data, limits)
	if err != nil {
		return jsonError(checkInError(err))
	}
	candidate := PeachfuzzCheckInPayload(wire)
	if err := candidate.Validate(); err != nil {
		return jsonError(err)
	}
	*p = candidate
	return nil
}

// PeachfuzzCheckInRequest is the complete body one installation sends: what it
// did, the credential proving it may, and its signature over both.
type PeachfuzzCheckInRequest struct {
	Payload     PeachfuzzCheckInPayload         `json:"payload"`
	Certificate InstallationCertificateDocument `json:"certificate"`
	Attestation attest.Envelope[SigningDomain]  `json:"attestation"`
}

type peachfuzzCheckInRequestWire PeachfuzzCheckInRequest

// PeachfuzzCheckInVerification carries the authority's trusted keys into exact
// credential and device-request authentication.
type PeachfuzzCheckInVerification struct {
	Request     PeachfuzzCheckInRequest
	TrustedKeys attest.TrustedKeys
}

// VerifiedPeachfuzzCheckIn is proof that a check-in authenticated. Its fields
// are unexported so it cannot be manufactured without verifying.
type VerifiedPeachfuzzCheckIn struct {
	request          PeachfuzzCheckInRequest
	certificateProof attest.Verified[SigningDomain]
	requestProof     attest.Verified[SigningDomain]
}

// Validate closes the request and binds its payload to the credential it
// presents.
func (r PeachfuzzCheckInRequest) Validate() error {
	if err := r.Payload.Validate(); err != nil {
		return checkInError(err)
	}
	return validateCheckInDocument(
		r.Payload.checkInBinding(), r.Certificate, r.Attestation, r.Payload.AttestationDomain(),
	)
}

// MarshalJSON emits one bounded canonical request.
func (r PeachfuzzCheckInRequest) MarshalJSON() ([]byte, error) {
	if err := r.Validate(); err != nil {
		return nil, jsonError(err)
	}
	encoded, err := core.MarshalCanonicalJSONDocument(peachfuzzCheckInRequestWire(r))
	if err != nil || len(encoded) > PeachfuzzCheckInRequestJSONMaximumBytes {
		return nil, jsonError(checkInError(err))
	}
	return encoded, nil
}

// UnmarshalJSON strictly decodes without mutating the receiver on rejection.
func (r *PeachfuzzCheckInRequest) UnmarshalJSON(data []byte) error {
	if r == nil {
		return jsonError(checkInError())
	}
	limits, err := documentJSONLimits(PeachfuzzCheckInRequestJSONMaximumBytes)
	if err != nil {
		return jsonError(checkInError(err))
	}
	wire, err := core.DecodeStrictJSONStructure[peachfuzzCheckInRequestWire](data, limits)
	if err != nil {
		return jsonError(checkInError(err))
	}
	candidate := PeachfuzzCheckInRequest(wire)
	if err := candidate.Validate(); err != nil {
		return jsonError(err)
	}
	*r = candidate
	return nil
}

// IssuePeachfuzzCheckIn signs one validated payload with the device key and
// attaches the credential the authority issued for that device.
func IssuePeachfuzzCheckIn(
	payload PeachfuzzCheckInPayload,
	key ed25519.PrivateKey,
	certificate InstallationCertificateDocument,
) (PeachfuzzCheckInRequest, error) {
	if err := errors.Join(payload.Validate(), certificate.Validate()); err != nil {
		return PeachfuzzCheckInRequest{}, checkInError(err)
	}
	envelope, err := attest.Sign(attest.SignRequest[SigningDomain]{Body: payload, Signer: key})
	if err != nil {
		return PeachfuzzCheckInRequest{}, checkInError(err)
	}
	request := PeachfuzzCheckInRequest{Payload: payload, Certificate: certificate, Attestation: envelope}
	return request, request.Validate()
}

// Validate closes the complete verification input.
func (v PeachfuzzCheckInVerification) Validate() error {
	if err := errors.Join(v.Request.Validate(), v.TrustedKeys.Validate()); err != nil {
		return checkInError(err)
	}
	return nil
}

// VerifyPeachfuzzCheckIn authenticates the authority-issued credential first,
// then uses the exact device key it names as the sole authority for the request.
func VerifyPeachfuzzCheckIn(verification PeachfuzzCheckInVerification) (VerifiedPeachfuzzCheckIn, error) {
	if err := verification.Validate(); err != nil {
		return VerifiedPeachfuzzCheckIn{}, err
	}
	certificateProof, deviceKeys, err := verifyCheckInCertificate(
		verification.Request.Certificate, verification.TrustedKeys,
	)
	if err != nil {
		return VerifiedPeachfuzzCheckIn{}, err
	}
	requestProof, err := attest.Verify(attest.VerifyRequest[SigningDomain]{
		Body:        verification.Request.Payload,
		Envelope:    verification.Request.Attestation,
		TrustedKeys: deviceKeys,
	})
	if err != nil {
		return VerifiedPeachfuzzCheckIn{}, checkInError(err)
	}
	result := VerifiedPeachfuzzCheckIn{
		request: verification.Request, certificateProof: certificateProof, requestProof: requestProof,
	}
	return result, result.Validate()
}

// Validate revalidates every proof the type claims to hold.
func (v VerifiedPeachfuzzCheckIn) Validate() error {
	return validateCheckInProofs(v.request.Validate(), v.certificateProof, v.requestProof)
}

// Request returns the authenticated request, revalidating first.
func (v VerifiedPeachfuzzCheckIn) Request() (PeachfuzzCheckInRequest, error) {
	if err := v.Validate(); err != nil {
		return PeachfuzzCheckInRequest{}, err
	}
	return v.request, nil
}

var (
	_ core.Validatable = PeachfuzzCommand(PeachfuzzCommandUnknown)
	_ core.Validatable = PeachfuzzOutcome(PeachfuzzOutcomeUnknown)
	_ core.Validatable = PeachfuzzCommandCount{}
	_ core.Validatable = PeachfuzzOutcomeCount{}
	_ core.Validatable = PeachfuzzUsageWindow{}
	_ core.Validatable = PeachfuzzCheckInPayload{}
	_ core.Validatable = PeachfuzzCheckInRequest{}
	_ core.Validatable = PeachfuzzCheckInVerification{}
	_ core.Validatable = VerifiedPeachfuzzCheckIn{}

	_ core.ValidatedJSONMarshaler = PeachfuzzCommand(PeachfuzzCommandUnknown)
	_ core.ValidatedJSONMarshaler = PeachfuzzOutcome(PeachfuzzOutcomeUnknown)
	_ core.ValidatedJSONMarshaler = PeachfuzzUsageWindow{}
	_ core.ValidatedJSONMarshaler = PeachfuzzCheckInPayload{}
	_ core.ValidatedJSONMarshaler = PeachfuzzCheckInRequest{}

	_ json.Unmarshaler = (*PeachfuzzCommand)(nil)
	_ json.Unmarshaler = (*PeachfuzzOutcome)(nil)
	_ json.Unmarshaler = (*PeachfuzzUsageWindow)(nil)
	_ json.Unmarshaler = (*PeachfuzzCheckInPayload)(nil)
	_ json.Unmarshaler = (*PeachfuzzCheckInRequest)(nil)

	_ attest.CanonicalBody[SigningDomain] = PeachfuzzCheckInPayload{}
)
