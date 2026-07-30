package lease

import (
	"errors"

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

// ScopeMismatch reports an authentic decision whose subject differs from the
// subject the caller asked Verify to accept.
type ScopeMismatch struct {
	Expected Subject
	Actual   Subject
}

// Error returns a non-sensitive diagnostic.
func (ScopeMismatch) Error() string {
	return core.ErrLeaseScope.Error()
}

// Unwrap preserves the stable Core identity.
func (ScopeMismatch) Unwrap() error {
	return core.ErrLeaseScope
}

// ClockContradiction reports a wall reading that trails trusted progress by
// more than ClockRollbackTolerance.
type ClockContradiction struct {
	Observed temporal.Instant
	Trusted  temporal.Instant
}

// Error returns a non-sensitive diagnostic.
func (ClockContradiction) Error() string {
	return core.ErrLeaseClock.Error()
}

// Unwrap preserves the stable Core identity.
func (ClockContradiction) Unwrap() error {
	return core.ErrLeaseClock
}
