package contextstate

import "github.com/deliri/primitive/v2026/core"

const (
	stateUnknownDiagnostic          = "unknown"
	stateNoneDiagnostic             = "none"
	stateCancelledDiagnostic        = "cancelled"
	stateDeadlineExceededDiagnostic = "deadline exceeded"
)

// State is the closed set of observable context terminal states.
type State uint8

const (
	// StateUnknown is the invalid zero state.
	StateUnknown State = iota
	// StateNone means no standard terminal state was observed.
	StateNone
	// StateCancelled means cancellation was observed.
	StateCancelled
	// StateDeadlineExceeded means deadline expiration was observed.
	StateDeadlineExceeded
	stateLimit
)

// Validate rejects states outside the closed domain.
func (s State) Validate() error {
	if s <= StateUnknown || s >= stateLimit {
		return core.ErrContextStateContract
	}
	return nil
}

// IsValid reports whether s belongs to the closed State domain.
func (s State) IsValid() bool {
	return s.Validate() == nil
}

// OffWireEnum declares State as an off-wire enum. The declaration binds State
// to core.OffWireEnum below, so the marker is compiler-checked rather than a
// bare name the pinned doctrine analyzer matches by convention. The
// standard-interface absence is proved independently in tests.
func (State) OffWireEnum() {}

// String returns a diagnostic projection of s. It is not a wire format, and
// State deliberately implements no marshaler or unmarshaler. That absence is
// asserted directly in state_external_test rather than inferred from the
// OffWireEnum declaration.
func (s State) String() string {
	switch s {
	case StateNone:
		return stateNoneDiagnostic
	case StateCancelled:
		return stateCancelledDiagnostic
	case StateDeadlineExceeded:
		return stateDeadlineExceededDiagnostic
	default:
		return stateUnknownDiagnostic
	}
}

var (
	_ core.Validatable = StateUnknown
	_ core.OffWireEnum = StateUnknown
)
