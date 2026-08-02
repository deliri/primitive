package release

import (
	"errors"
	"fmt"

	"github.com/deliri/primitive/v2026/core"
)

const canonicalDestinationNilDiagnostic = "canonical destination is nil"

// OfferingMismatchError carries the exact observed and expected offering facts
// when authenticated Release input names the wrong stream.
type OfferingMismatchError struct {
	observed core.Offering
	expected core.Offering
}

func newOfferingMismatchError(observed, expected core.Offering) (OfferingMismatchError, error) {
	mismatch := OfferingMismatchError{observed: observed, expected: expected}
	if err := mismatch.Validate(); err != nil {
		return OfferingMismatchError{}, err
	}
	return mismatch, nil
}

// Validate proves both offerings and their contradiction.
func (e OfferingMismatchError) Validate() error {
	if err := e.observed.Validate(); err != nil {
		return contractError(errors.New("observed release offering is invalid"), err)
	}
	if err := e.expected.Validate(); err != nil {
		return contractError(errors.New("expected release offering is invalid"), err)
	}
	if e.observed == e.expected {
		return contractError(errors.New("release offering mismatch names equal offerings"))
	}
	return nil
}

// Error returns the operator-facing offering contradiction.
func (e OfferingMismatchError) Error() string {
	if e.Validate() != nil {
		return "release offering mismatch is invalid"
	}
	return "release offering " + e.observed.String() +
		" differs from expected " + e.expected.String()
}

// Unwrap preserves the stable Release verification identity.
func (OfferingMismatchError) Unwrap() error { return core.ErrReleaseVerification }

// Observed returns the offering carried by the authenticated document.
func (e OfferingMismatchError) Observed() core.Offering { return e.observed }

// Expected returns the caller-selected offering.
func (e OfferingMismatchError) Expected() core.Offering { return e.expected }

func contractError(causes ...error) error {
	return releaseError(core.ErrReleaseContract, causes...)
}

func manifestError(causes ...error) error {
	return releaseError(core.ErrReleaseManifest, causes...)
}

func verificationError(causes ...error) error {
	return releaseError(core.ErrReleaseVerification, causes...)
}

func offeringMismatchError(observed, expected core.Offering) error {
	mismatch, err := newOfferingMismatchError(observed, expected)
	if err != nil {
		return verificationError(err)
	}
	return verificationError(mismatch)
}

func latestError(causes ...error) error {
	return releaseError(core.ErrReleaseLatest, causes...)
}

func rollbackError(causes ...error) error {
	return releaseError(core.ErrReleaseRollback, causes...)
}

func conflictError(causes ...error) error {
	return releaseError(core.ErrReleaseConflict, causes...)
}

func jsonError(causes ...error) error {
	return releaseError(core.ErrReleaseContract, append([]error{core.ErrJSONContract}, causes...)...)
}

func releaseError(identity error, causes ...error) error {
	all := append([]error{identity}, causes...)
	return fmt.Errorf("release: %w", errors.Join(all...))
}
