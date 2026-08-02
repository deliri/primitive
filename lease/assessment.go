package lease

import (
	"errors"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

const (
	// ClockRollbackToleranceNanoseconds is the exact accepted difference
	// between an observed wall reading and trusted time progress.
	ClockRollbackToleranceNanoseconds int64 = 5 * int64(temporal.NanosecondsPerMinute)
)

func stateDiagnostics() [stateLimit]string {
	return [...]string{
		StateNotYetValid: "not-yet-valid",
		StateCurrent:     "current",
		StateContinuity:  "continuity",
		StateExpired:     "expired",
		StateRefused:     "refused",
		StateRevoked:     "revoked",
	}
}

// State is the complete local status of one authentic decision.
type State uint8

const (
	StateUnknown State = iota
	StateNotYetValid
	StateCurrent
	StateContinuity
	StateExpired
	StateRefused
	StateRevoked
	stateLimit
)

// Validate rejects values outside the closed assessment-state domain.
func (s State) Validate() error {
	if !s.IsValid() {
		return contractError(errors.New("lease state is outside the closed domain"))
	}
	return nil
}

// IsValid reports membership in the assessment-state domain.
func (s State) IsValid() bool {
	return s > StateUnknown && s < stateLimit && stateDiagnostics()[s] != ""
}

// OffWireEnum declares State as a deliberate off-wire enum.
func (State) OffWireEnum() {}

// String returns one diagnostic label.
func (s State) String() string {
	if !s.IsValid() {
		return core.UnknownEnumDiagnostic
	}
	return stateDiagnostics()[s]
}

// ContactState is the local earliest-contact classification.
type ContactState uint8

const (
	ContactStateUnknown ContactState = iota
	ContactStateNotDue
	ContactStateDue
	ContactStateProhibited
	contactStateLimit
)

func contactStateDiagnostics() [contactStateLimit]string {
	return [...]string{
		ContactStateNotDue:     "not-due",
		ContactStateDue:        "due",
		ContactStateProhibited: "prohibited",
	}
}

// Validate rejects values outside the closed contact-state domain.
func (s ContactState) Validate() error {
	if !s.IsValid() {
		return contractError(errors.New("lease contact state is outside the closed domain"))
	}
	return nil
}

// IsValid reports membership in the contact-state domain.
func (s ContactState) IsValid() bool {
	return s > ContactStateUnknown && s < contactStateLimit &&
		contactStateDiagnostics()[s] != ""
}

// OffWireEnum declares ContactState as a deliberate off-wire enum.
func (ContactState) OffWireEnum() {}

// String returns one diagnostic label.
func (s ContactState) String() string {
	if !s.IsValid() {
		return core.UnknownEnumDiagnostic
	}
	return contactStateDiagnostics()[s]
}

// EvaluateRequest supplies real Temporal observations and the consumer's
// durably committed high-water. StartedAt and ObservedAt normally preserve
// Go's monotonic reading because the consumer obtains them from
// temporal.Observe.
type EvaluateRequest struct {
	StartedAt        temporal.Observation
	ObservedAt       temporal.Observation
	Decision         Verified
	DurableHighWater temporal.Instant
}

// Validate checks the complete pure evaluation ingress.
func (r EvaluateRequest) Validate() error {
	if err := r.Decision.Validate(); err != nil {
		return contractError(err)
	}
	if err := r.DurableHighWater.Validate(); err != nil {
		return contractError(err)
	}
	if err := r.StartedAt.Validate(); err != nil {
		return contractError(err)
	}
	if err := r.ObservedAt.Validate(); err != nil {
		return contractError(err)
	}
	if _, err := r.ObservedAt.Since(r.StartedAt); err != nil {
		return contractError(err)
	}
	return nil
}

// Assessment is one pure, fixed-size decision at one effective instant.
type Assessment struct {
	decision    Decision
	effectiveAt temporal.Instant
	state       State
	contact     ContactState
	valid       bool
}

// Evaluate advances trusted time with Go's monotonic elapsed duration,
// detects excessive wall rollback, and classifies the authentic decision.
func Evaluate(request EvaluateRequest) (Assessment, error) {
	if err := request.Validate(); err != nil {
		return Assessment{}, err
	}
	decision, err := request.Decision.Decision()
	if err != nil {
		return Assessment{}, err
	}
	header, err := decision.Header()
	if err != nil {
		return Assessment{}, err
	}
	floor, err := maximumInstant(header.IssuedAt, request.DurableHighWater)
	if err != nil {
		return Assessment{}, err
	}
	effective, err := advanceEffectiveTime(request, floor)
	if err != nil {
		return Assessment{}, err
	}
	state, contact, err := classifyDecision(decision, effective)
	if err != nil {
		return Assessment{}, err
	}
	result := Assessment{
		decision: decision, effectiveAt: effective,
		state: state, contact: contact, valid: true,
	}
	return result, result.Validate()
}

// advanceEffectiveTime derives trusted progress and reconciles it with the
// current wall reading.
//
// Trusted progress is the later of the floor (the signed issuance and the
// consumer's durable high-water) and the start observation advanced by Go's
// monotonic elapsed duration. The elapsed duration is anchored to the start
// observation's own wall reading, never to the floor: a start observation that
// precedes the floor is the ordinary state of any process whose uptime exceeds
// the interval it has already recorded, and adding its elapsed time to the
// floor would count that interval twice and inflate the effective clock on
// every evaluation.
//
// The single contradiction check compares the current wall reading against that
// trusted progress. A start observation older than the floor is not itself a
// contradiction; only a current reading that trails trusted progress is.
func advanceEffectiveTime(
	request EvaluateRequest,
	floor temporal.Instant,
) (temporal.Instant, error) {
	elapsed, err := request.ObservedAt.Since(request.StartedAt)
	if err != nil {
		return temporal.Instant{}, contractError(err)
	}
	startWall, err := request.StartedAt.Instant()
	if err != nil {
		return temporal.Instant{}, contractError(err)
	}
	advanced, err := startWall.Add(elapsed)
	if err != nil {
		return temporal.Instant{}, contractError(err)
	}
	trusted, err := maximumInstant(floor, advanced)
	if err != nil {
		return temporal.Instant{}, err
	}
	wall, err := request.ObservedAt.Instant()
	if err != nil {
		return temporal.Instant{}, contractError(err)
	}
	if err := rejectClockContradiction(wall, trusted); err != nil {
		return temporal.Instant{}, err
	}
	return maximumInstant(wall, trusted)
}

func rejectClockContradiction(observed, trusted temporal.Instant) error {
	comparison, err := observed.Compare(trusted)
	if err != nil {
		return contractError(err)
	}
	if comparison != core.ComparisonLess {
		return nil
	}
	difference, err := trusted.Since(observed)
	if err != nil {
		return errors.Join(newClockContradiction(observed, trusted), err)
	}
	if difference.Nanoseconds() > ClockRollbackToleranceNanoseconds {
		return newClockContradiction(observed, trusted)
	}
	return nil
}

func maximumInstant(left, right temporal.Instant) (temporal.Instant, error) {
	comparison, err := left.Compare(right)
	if err != nil {
		return temporal.Instant{}, contractError(err)
	}
	if comparison == core.ComparisonLess {
		return right, nil
	}
	return left, nil
}

func classifyDecision(
	decision Decision,
	effective temporal.Instant,
) (State, ContactState, error) {
	switch decision.outcome {
	case OutcomeGrant:
		state, err := classifyGrant(decision.grant, effective)
		if err != nil {
			return StateUnknown, ContactStateUnknown, err
		}
		contact, err := classifyContact(decision.grant.ContactAfter, effective)
		return state, contact, err
	case OutcomeRefusal:
		contact, err := classifyContact(decision.refusal.ContactAfter, effective)
		return StateRefused, contact, err
	case OutcomeRevocation:
		return StateRevoked, ContactStateProhibited, nil
	default:
		return StateUnknown, ContactStateUnknown,
			contractError(errors.New(decisionOutcomeUnsupportedText))
	}
}

func classifyGrant(grant Grant, effective temporal.Instant) (State, error) {
	beforeStart, err := effective.Compare(grant.NotBefore)
	if err != nil {
		return StateUnknown, contractError(err)
	}
	if beforeStart == core.ComparisonLess {
		return StateNotYetValid, nil
	}
	beforeEnd, err := effective.Compare(grant.NotAfter)
	if err != nil {
		return StateUnknown, contractError(err)
	}
	if beforeEnd == core.ComparisonLess {
		return StateCurrent, nil
	}
	beforeGoodUntil, err := effective.Compare(grant.GoodUntil)
	if err != nil {
		return StateUnknown, contractError(err)
	}
	if beforeGoodUntil == core.ComparisonLess {
		return StateContinuity, nil
	}
	return StateExpired, nil
}

func classifyContact(boundary, effective temporal.Instant) (ContactState, error) {
	comparison, err := effective.Compare(boundary)
	if err != nil {
		return ContactStateUnknown, contractError(err)
	}
	if comparison == core.ComparisonLess {
		return ContactStateNotDue, nil
	}
	return ContactStateDue, nil
}

// Validate rejects the zero result and recomputes its classification.
func (a Assessment) Validate() error {
	if !a.valid {
		return contractError(errors.New("lease assessment is unset"))
	}
	if err := a.decision.Validate(); err != nil {
		return contractError(err)
	}
	if err := a.effectiveAt.Validate(); err != nil {
		return contractError(err)
	}
	if err := a.state.Validate(); err != nil {
		return contractError(err)
	}
	if err := a.contact.Validate(); err != nil {
		return contractError(err)
	}
	state, contact, err := classifyDecision(a.decision, a.effectiveAt)
	if err != nil || state != a.state || contact != a.contact {
		return contractError(errors.New("lease assessment projection is inconsistent"), err)
	}
	return nil
}

// Decision returns the exact authentic decision being assessed.
func (a Assessment) Decision() (Decision, error) {
	if err := a.Validate(); err != nil {
		return Decision{}, err
	}
	return a.decision, nil
}

// EffectiveAt returns the monotonic/durable high-water to persist.
func (a Assessment) EffectiveAt() (temporal.Instant, error) {
	if err := a.Validate(); err != nil {
		return temporal.Instant{}, err
	}
	return a.effectiveAt, nil
}

// State returns the local work-state fact.
func (a Assessment) State() State {
	return a.state
}

// ContactState returns whether the signed earliest contact is due.
func (a Assessment) ContactState() ContactState {
	return a.contact
}

var (
	_ core.OffWireEnum = StateUnknown
	_ core.OffWireEnum = ContactStateUnknown
)
