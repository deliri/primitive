package release

import (
	"errors"
	"fmt"

	"github.com/deliri/primitive/v2026/core"
)

const canonicalDestinationNilDiagnostic = "canonical destination is nil"

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
	mismatch, err := core.NewReleaseOfferingMismatchError(observed, expected)
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
