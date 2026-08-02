package receipt

import (
	"errors"

	"github.com/deliri/primitive/v2026/core"
)

func contractError(causes ...error) error {
	return errors.Join(append([]error{core.ErrReceiptContract}, causes...)...)
}

func jsonError(causes ...error) error {
	return errors.Join(append(
		[]error{core.ErrReceiptContract, core.ErrJSONContract},
		causes...,
	)...)
}

func verificationError(causes ...error) error {
	return errors.Join(append([]error{core.ErrReceiptVerification}, causes...)...)
}

func rollbackError() error {
	return errors.Join(core.ErrReceiptRollback)
}

// ScopeMismatch is a sealed report that authentic evidence describes a scope
// the caller did not ask for.
//
// Only Receipt can implement it. Its package-private implementation is a value
// that always carries the exact field that differed, so a caller-built zero or
// typed-nil carrier cannot claim Receipt's scope identity without naming the
// authenticated fact that refused it.
type ScopeMismatch interface {
	error
	Validate() error
	Field() (ScopeField, error)
	receiptScopeMismatch()
}

type scopeMismatch struct {
	field ScopeField
}

func newScopeMismatch(field ScopeField) error {
	mismatch := scopeMismatch{field: field}
	if err := mismatch.Validate(); err != nil {
		return err
	}
	return mismatch
}

// Validate rejects a mismatch that names no admitted field.
func (e scopeMismatch) Validate() error { return e.field.Validate() }

// Error returns the specialized diagnostic only for a proved mismatch.
func (e scopeMismatch) Error() string {
	if e.Validate() != nil {
		return core.ErrReceiptContract.Error()
	}
	return core.ErrReceiptScope.Error()
}

// Unwrap preserves the specialized Core identity only for a proved mismatch.
func (e scopeMismatch) Unwrap() error {
	if e.Validate() != nil {
		return core.ErrReceiptContract
	}
	return core.ErrReceiptScope
}

// Field returns the exact authenticated fact that differed from caller intent.
func (e scopeMismatch) Field() (ScopeField, error) {
	if err := e.Validate(); err != nil {
		return ScopeFieldUnknown, err
	}
	return e.field, nil
}

func (scopeMismatch) receiptScopeMismatch() {}

// WatermarkConflict is a sealed report that two watermarks cannot be
// reconciled.
//
// Only Receipt can implement it. Like ScopeMismatch it is a value carrying the
// exact reason, so a rejected advance always says which invariant refused it
// rather than collapsing distinct causes into one opaque sentinel.
type WatermarkConflict interface {
	error
	Validate() error
	Reason() (ConflictReason, error)
	receiptWatermarkConflict()
}

type watermarkConflict struct {
	reason ConflictReason
}

func conflictError(reason ConflictReason) error {
	conflict := watermarkConflict{reason: reason}
	if err := conflict.Validate(); err != nil {
		return err
	}
	return conflict
}

// Validate rejects a conflict that names no admitted reason.
func (e watermarkConflict) Validate() error { return e.reason.Validate() }

// Error returns the specialized diagnostic only for a proved conflict.
func (e watermarkConflict) Error() string {
	if e.Validate() != nil {
		return core.ErrReceiptContract.Error()
	}
	return core.ErrReceiptConflict.Error()
}

// Unwrap preserves the specialized Core identity only for a proved conflict.
func (e watermarkConflict) Unwrap() error {
	if e.Validate() != nil {
		return core.ErrReceiptContract
	}
	return core.ErrReceiptConflict
}

// Reason returns the exact invariant that refused the advance.
func (e watermarkConflict) Reason() (ConflictReason, error) {
	if err := e.Validate(); err != nil {
		return ConflictReasonUnknown, err
	}
	return e.reason, nil
}

func (watermarkConflict) receiptWatermarkConflict() {}

var (
	_ ScopeMismatch     = scopeMismatch{}
	_ WatermarkConflict = watermarkConflict{}
	_ core.Validatable  = scopeMismatch{}
	_ core.Validatable  = watermarkConflict{}
)
