package lease

import (
	"errors"
	"math"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

const (
	decisionOutcomeUnsupportedText = "decision outcome is unsupported"
	enumJSONExtentInvalidText      = "lease enum JSON extent is invalid"
	unknownDiagnostic              = "unknown"
)

func contractError(causes ...error) error {
	return errors.Join(append([]error{core.ErrLeaseContract}, causes...)...)
}

func jsonError(causes ...error) error {
	return errors.Join(append(
		[]error{core.ErrLeaseContract, core.ErrJSONContract},
		causes...,
	)...)
}

func verificationError(causes ...error) error {
	return errors.Join(append(
		[]error{core.ErrLeaseVerification},
		causes...,
	)...)
}

func rollbackError(causes ...error) error {
	return errors.Join(append([]error{core.ErrLeaseRollback}, causes...)...)
}

func conflictError(causes ...error) error {
	return errors.Join(append([]error{core.ErrLeaseConflict}, causes...)...)
}

// ScopeMismatch is a sealed report that an authentic decision names a subject
// other than the one the caller asked Verify to accept.
//
// Only Lease can implement it. A caller therefore cannot forge the stable
// scope identity from unset or equal subjects.
type ScopeMismatch interface {
	error
	Validate() error
	Subjects() (Subject, Subject, error)
	leaseScopeMismatch()
}

type scopeMismatch struct {
	expected Subject
	actual   Subject
}

func newScopeMismatch(expected, actual Subject) error {
	mismatch := scopeMismatch{expected: expected, actual: actual}
	if err := mismatch.Validate(); err != nil {
		return err
	}
	return mismatch
}

// Validate proves both subjects are admitted and differ.
func (e scopeMismatch) Validate() error {
	if err := e.expected.Validate(); err != nil {
		return contractError(err)
	}
	if err := e.actual.Validate(); err != nil {
		return contractError(err)
	}
	if e.expected == e.actual {
		return contractError(errors.New("scope mismatch subjects are equal"))
	}
	return nil
}

// Error returns a non-sensitive diagnostic.
func (e scopeMismatch) Error() string {
	if e.Validate() != nil {
		return core.ErrLeaseContract.Error()
	}
	return core.ErrLeaseScope.Error()
}

// Unwrap preserves the stable Core identity only for a proved mismatch.
func (e scopeMismatch) Unwrap() error {
	if e.Validate() != nil {
		return core.ErrLeaseContract
	}
	return core.ErrLeaseScope
}

// Subjects returns the caller's expected subject and the authentic subject.
func (e scopeMismatch) Subjects() (Subject, Subject, error) {
	if err := e.Validate(); err != nil {
		return Subject{}, Subject{}, err
	}
	return e.expected, e.actual, nil
}

func (scopeMismatch) leaseScopeMismatch() {}

// ClockContradiction is a sealed report that a wall reading trails trusted
// progress by more than ClockRollbackToleranceNanoseconds.
//
// Only Lease can implement it. A caller therefore cannot forge the stable
// clock identity from unset, reordered, or merely tolerated instants.
type ClockContradiction interface {
	error
	Validate() error
	Instants() (temporal.Instant, temporal.Instant, error)
	leaseClockContradiction()
}

type clockContradiction struct {
	observed temporal.Instant
	trusted  temporal.Instant
}

func newClockContradiction(observed, trusted temporal.Instant) error {
	contradiction := clockContradiction{observed: observed, trusted: trusted}
	if err := contradiction.Validate(); err != nil {
		return err
	}
	return contradiction
}

// Validate proves the wall reading trails trusted progress beyond tolerance.
func (e clockContradiction) Validate() error {
	observed, err := e.observed.Nanoseconds()
	if err != nil {
		return contractError(err)
	}
	trusted, err := e.trusted.Nanoseconds()
	if err != nil {
		return contractError(err)
	}
	if observed >= trusted {
		return contractError(errors.New("clock contradiction does not trail trusted progress"))
	}
	if trusted < math.MinInt64+ClockRollbackToleranceNanoseconds ||
		observed >= trusted-ClockRollbackToleranceNanoseconds {
		return contractError(errors.New("clock contradiction is within rollback tolerance"))
	}
	return nil
}

// Error returns a non-sensitive diagnostic.
func (e clockContradiction) Error() string {
	if e.Validate() != nil {
		return core.ErrLeaseContract.Error()
	}
	return core.ErrLeaseClock.Error()
}

// Unwrap preserves the stable Core identity only for a proved contradiction.
func (e clockContradiction) Unwrap() error {
	if e.Validate() != nil {
		return core.ErrLeaseContract
	}
	return core.ErrLeaseClock
}

// Instants returns the observed wall reading and the trusted high water.
func (e clockContradiction) Instants() (temporal.Instant, temporal.Instant, error) {
	if err := e.Validate(); err != nil {
		return temporal.Instant{}, temporal.Instant{}, err
	}
	return e.observed, e.trusted, nil
}

func (clockContradiction) leaseClockContradiction() {}

var (
	_ ScopeMismatch      = scopeMismatch{}
	_ ClockContradiction = clockContradiction{}
	_ core.Validatable   = scopeMismatch{}
	_ core.Validatable   = clockContradiction{}
)
