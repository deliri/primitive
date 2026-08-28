package controlplane

import (
	"crypto/ed25519"
	"errors"

	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/lease"
)

// CheckInCommitRequest is the exact input to an authority's usage transaction.
// Current is the watermark read inside that transaction. RequiredPolicy is the
// authority's selected policy cursor; Primitive compares it but never
// interprets what the policy means.
type CheckInCommitRequest struct {
	Current        UsageWatermark
	CheckIn        VerifiedCheckIn
	RequiredPolicy controlwire.PolicyCursor
}

// VerifiedCheckInCommit is the proof-carrying result of comparing one
// authenticated check-in with authoritative usage and policy facts. It owns no
// storage and performs no effect.
type VerifiedCheckInCommit struct {
	watermark   UsageWatermark
	request     CheckInCommitRequest
	disposition UsageDisposition
}

// CheckInResponsePreparation binds authority-owned response facts to one
// verified usage commit. Disposition and watermark are derived from Commit and
// cannot be repeated or substituted by the caller.
type CheckInResponsePreparation struct {
	Header ResponseHeader
	Lease  lease.Document
	Commit VerifiedCheckInCommit
}

// Validate closes all inputs before the authority uses them in its own
// transaction.
func (r CheckInCommitRequest) Validate() error {
	if err := errors.Join(
		r.CheckIn.Validate(), r.Current.Validate(), r.RequiredPolicy.Validate(),
	); err != nil {
		return checkInError(err)
	}
	request, err := r.CheckIn.Request()
	if err != nil {
		return err
	}
	if request.Payload.AppliedPolicy != r.RequiredPolicy {
		return checkInError(core.ErrControlWirePolicyCursor)
	}
	if request.Payload.PreviousWatermark.Subject != r.Current.Subject {
		return checkInError(consistencyError())
	}
	return nil
}

// CommitCheckIn resolves exactly three states from immutable facts: the window
// advances once, is the exact window already committed, or conflicts with a
// different authoritative watermark. It allocates no history and owns no
// persistence mechanism.
func (s Server) CommitCheckIn(request CheckInCommitRequest) (VerifiedCheckInCommit, error) {
	if err := s.Validate(); err != nil {
		return VerifiedCheckInCommit{}, checkInError(err)
	}
	watermark, disposition, err := resolveCheckInCommit(request)
	if err != nil {
		return VerifiedCheckInCommit{}, err
	}
	commit := VerifiedCheckInCommit{
		request: request, watermark: watermark, disposition: disposition,
	}
	return commit, commit.Validate()
}

func resolveCheckInCommit(
	commit CheckInCommitRequest,
) (UsageWatermark, UsageDisposition, error) {
	if err := commit.Validate(); err != nil {
		return UsageWatermark{}, UsageDispositionUnknown, err
	}
	request, err := commit.CheckIn.Request()
	if err != nil {
		return UsageWatermark{}, UsageDispositionUnknown, err
	}
	proposed, err := AdvanceUsageWatermark(
		request.Payload.PreviousWatermark,
		request.Payload.Window,
	)
	if err != nil {
		return UsageWatermark{}, UsageDispositionUnknown, checkInError(err)
	}
	switch {
	case commit.Current == request.Payload.PreviousWatermark:
		return proposed, UsageDispositionAccepted, nil
	case commit.Current == proposed:
		return commit.Current, UsageDispositionReplay, nil
	default:
		return commit.Current, UsageDispositionConflict, nil
	}
}

// Validate replays the complete comparison and refuses a manufactured proof.
func (c VerifiedCheckInCommit) Validate() error {
	watermark, disposition, err := resolveCheckInCommit(c.request)
	if err != nil {
		return err
	}
	if c.watermark != watermark || c.disposition != disposition {
		return checkInError()
	}
	return nil
}

// Watermark returns the exact watermark the authority must sign and, only for
// an accepted disposition, persist.
func (c VerifiedCheckInCommit) Watermark() (UsageWatermark, error) {
	if err := c.Validate(); err != nil {
		return UsageWatermark{}, err
	}
	return c.watermark, nil
}

// Disposition reports whether the immutable comparison accepted, replayed, or
// conflicted.
func (c VerifiedCheckInCommit) Disposition() (UsageDisposition, error) {
	if err := c.Validate(); err != nil {
		return UsageDispositionUnknown, err
	}
	return c.disposition, nil
}

// Validate closes the authority response against the exact authenticated
// request and commit result it answers.
func (p CheckInResponsePreparation) Validate() error {
	if err := errors.Join(p.Commit.Validate(), p.Header.Validate(), p.Lease.Validate()); err != nil {
		return checkInResponseError(err)
	}
	request, err := p.Commit.request.CheckIn.Request()
	if err != nil {
		return checkInResponseError(err)
	}
	expectation := ResponseExpectation{
		RequestNonce: request.Payload.RequestNonce,
		Account:      request.Certificate.Body.Account,
		Installation: request.Payload.Installation,
		Revision:     request.Payload.Revision,
		Family:       controlwire.RouteFamilyCheckIns,
		Offering:     request.Payload.Build.Offering(),
	}
	if err := p.Header.ValidateAgainst(expectation); err != nil {
		return checkInResponseError(err)
	}
	if p.Header.Policy != p.Commit.request.RequiredPolicy {
		return checkInResponseError(core.ErrControlWirePolicyCursor)
	}
	return nil
}

// PrepareCheckInResponse derives the only disposition and watermark the
// authenticated usage transaction permits, then closes the complete payload.
func (s Server) PrepareCheckInResponse(
	preparation CheckInResponsePreparation,
) (CheckInResponsePayload, error) {
	if err := s.Validate(); err != nil {
		return CheckInResponsePayload{}, checkInResponseError(err)
	}
	return prepareCheckInResponse(preparation)
}

func prepareCheckInResponse(
	preparation CheckInResponsePreparation,
) (CheckInResponsePayload, error) {
	if err := preparation.Validate(); err != nil {
		return CheckInResponsePayload{}, err
	}
	payload := CheckInResponsePayload{
		Header: preparation.Header, Disposition: preparation.Commit.disposition,
		Watermark: preparation.Commit.watermark, Lease: preparation.Lease,
	}
	if err := payload.Validate(); err != nil {
		return CheckInResponsePayload{}, err
	}
	return payload, nil
}

// IssueCommittedCheckInResponse signs the response derived from one verified
// authority commit. It is the direct server-side counterpart to the installed
// client's VerifyCheckInResponse.
func (s Server) IssueCommittedCheckInResponse(
	preparation CheckInResponsePreparation,
	key ed25519.PrivateKey,
) (CheckInResponseDocument, error) {
	if err := s.Validate(); err != nil {
		return CheckInResponseDocument{}, checkInResponseError(err)
	}
	payload, err := prepareCheckInResponse(preparation)
	if err != nil {
		return CheckInResponseDocument{}, err
	}
	return issueCheckInResponse(payload, key)
}

var (
	_ core.Validatable = CheckInCommitRequest{}
	_ core.Validatable = VerifiedCheckInCommit{}
	_ core.Validatable = CheckInResponsePreparation{}
)
