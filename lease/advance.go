package lease

import (
	"errors"

	"github.com/deliri/primitive/v2026/core"
)

// AdvanceState reports whether the current authentic decision changed.
type AdvanceState uint8

const (
	AdvanceStateUnknown AdvanceState = iota
	AdvanceStateUnchanged
	AdvanceStateAdvanced
	advanceStateLimit
)

// Validate rejects values outside the closed advance-state domain.
func (s AdvanceState) Validate() error {
	if s <= AdvanceStateUnknown || s >= advanceStateLimit {
		return contractError(errors.New("lease advance state is outside the closed domain"))
	}
	return nil
}

// IsValid reports membership in the advance-state domain.
func (s AdvanceState) IsValid() bool { return s.Validate() == nil }

// OffWireEnum declares AdvanceState as a deliberate off-wire enum.
func (AdvanceState) OffWireEnum() {}

// String returns one diagnostic label.
func (s AdvanceState) String() string {
	switch s {
	case AdvanceStateUnchanged:
		return "unchanged"
	case AdvanceStateAdvanced:
		return "advanced"
	default:
		return unknownDiagnostic
	}
}

// AdvanceRequest compares one current and one candidate authentic decision.
type AdvanceRequest struct {
	Current   Verified
	Candidate Verified
}

// Validate closes both authentic inputs.
func (r AdvanceRequest) Validate() error {
	if err := r.Current.Validate(); err != nil {
		return contractError(err)
	}
	if err := r.Candidate.Validate(); err != nil {
		return contractError(err)
	}
	return nil
}

// AdvanceResult selects exactly one authentic decision.
type AdvanceResult struct {
	selected Verified
	state    AdvanceState
}

// Advance accepts an identical replay or a strictly newer, non-regressing
// decision for the same exact subject.
//
// Lease identity is decided before sequence. Two decisions that name different
// subjects or different revisions never share one generation sequence, so
// their generation order carries no meaning and every such pair conflicts.
func Advance(request AdvanceRequest) (AdvanceResult, error) {
	if err := request.Validate(); err != nil {
		return AdvanceResult{}, err
	}
	current := request.Current.decision
	candidate := request.Candidate.decision
	if err := requireSameLeaseIdentity(current.header, candidate.header); err != nil {
		return AdvanceResult{}, err
	}
	switch {
	case candidate.header.Generation.value < current.header.Generation.value:
		return AdvanceResult{}, rollbackError()
	case candidate.header.Generation.value == current.header.Generation.value:
		return sameGeneration(request.Current, current, candidate)
	}
	if err := requireNotAfter(current.header.IssuedAt, candidate.header.IssuedAt); err != nil {
		return AdvanceResult{}, rollbackError(err)
	}
	if current.outcome == OutcomeGrant && candidate.outcome == OutcomeGrant {
		if err := validateGrantAdvance(current.grant, candidate.grant); err != nil {
			return AdvanceResult{}, err
		}
	}
	result := AdvanceResult{
		selected: request.Candidate,
		state:    AdvanceStateAdvanced,
	}
	return result, result.Validate()
}

func requireSameLeaseIdentity(current, candidate Header) error {
	if current.Subject != candidate.Subject ||
		current.Revision != candidate.Revision {
		return conflictError()
	}
	return nil
}

func sameGeneration(
	current Verified,
	currentDecision, candidateDecision Decision,
) (AdvanceResult, error) {
	if currentDecision != candidateDecision {
		return AdvanceResult{}, conflictError()
	}
	result := AdvanceResult{
		selected: current,
		state:    AdvanceStateUnchanged,
	}
	return result, result.Validate()
}

func validateGrantAdvance(current, candidate Grant) error {
	if err := requireNotAfter(current.NotBefore, candidate.NotBefore); err != nil {
		return rollbackError(err)
	}
	if err := requireNotAfter(current.NotAfter, candidate.NotAfter); err != nil {
		return rollbackError(err)
	}
	if err := requireNotAfter(current.GoodUntil, candidate.GoodUntil); err != nil {
		return rollbackError(err)
	}
	return nil
}

// Validate rejects a zero or internally contradictory result.
func (r AdvanceResult) Validate() error {
	if err := r.state.Validate(); err != nil {
		return contractError(err)
	}
	if err := r.selected.Validate(); err != nil {
		return contractError(err)
	}
	return nil
}

// State returns whether selection changed.
func (r AdvanceResult) State() AdvanceState {
	return r.state
}

// Verified returns the selected authentic decision.
func (r AdvanceResult) Verified() (Verified, error) {
	if err := r.Validate(); err != nil {
		return Verified{}, err
	}
	return r.selected, nil
}

var _ core.OffWireEnum = AdvanceStateUnknown
