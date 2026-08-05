package controlplane

import (
	"errors"

	"github.com/deliri/primitive/v2026/core"
)

// signingDomainError reports a rejected signing domain.
func signingDomainError(causes ...error) error {
	return documentError(core.ErrControlPlaneSigningDomain, causes...)
}

// productStatusError reports a rejected commercial product status.
func productStatusError(causes ...error) error {
	return documentError(core.ErrControlPlaneProductStatus, causes...)
}

// usageWatermarkError reports a rejected usage watermark.
func usageWatermarkError(causes ...error) error {
	return documentError(core.ErrControlPlaneUsageWatermark, causes...)
}

// responseHeaderError reports a rejected response header.
func responseHeaderError(causes ...error) error {
	return documentError(core.ErrControlPlaneResponseHeader, causes...)
}

// registrationError reports a rejected registration document.
func registrationError(causes ...error) error {
	return documentError(core.ErrControlPlaneRegistration, causes...)
}

// installationBindingError reports an installation identity that its own device
// key does not derive.
func installationBindingError(causes ...error) error {
	return documentError(core.ErrControlPlaneInstallationBinding, causes...)
}

// consistencyError reports signed facts that disagree inside one document.
func consistencyError(causes ...error) error {
	return documentError(core.ErrControlPlaneDecisionConsistency, causes...)
}

// documentError joins the package identity, the exact document or field
// identity, and every non-nil cause. A nil cause is dropped so a rejection
// never widens into an unrelated identity a caller could match on.
func documentError(document core.ErrorIdentity, causes ...error) error {
	joined := make([]error, 0, len(causes)+2)
	joined = append(joined, core.ErrControlPlaneContract, document)
	for _, cause := range causes {
		if cause != nil {
			joined = append(joined, cause)
		}
	}
	return errors.Join(joined...)
}

// jsonError marks a rejection that happened at the JSON boundary.
func jsonError(cause error) error {
	return errors.Join(core.ErrJSONContract, cause)
}

// checkInError reports a rejected check-in request document.
func checkInError(causes ...error) error {
	return documentError(core.ErrControlPlaneCheckIn, causes...)
}

// checkInResponseError reports a rejected check-in response document.
func checkInResponseError(causes ...error) error {
	return documentError(core.ErrControlPlaneCheckInResponse, causes...)
}

// usageWindowError reports a rejected reported usage window.
func usageWindowError(causes ...error) error {
	return documentError(core.ErrControlPlaneUsageWindow, causes...)
}
