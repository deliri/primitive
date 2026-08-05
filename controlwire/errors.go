package controlwire

import (
	"errors"

	"github.com/deliri/primitive/v2026/core"
)

// revisionError reports a rejected wire revision under both the package
// contract identity and the exact scalar that refused the value.
func revisionError(causes ...error) error {
	return scalarError(core.ErrControlWireRevision, causes...)
}

// nonceError reports a rejected request nonce.
func nonceError(causes ...error) error {
	return scalarError(core.ErrControlWireNonce, causes...)
}

// tokenError reports a rejected registration token or verifier.
func tokenError(causes ...error) error {
	return scalarError(core.ErrControlWireToken, causes...)
}

// scalarError joins the package identity, the scalar identity, and every
// non-nil cause. A nil cause is dropped so a rejection never widens into an
// unrelated identity a caller could match on.
func scalarError(scalar core.ErrorIdentity, causes ...error) error {
	joined := make([]error, 0, len(causes)+2)
	joined = append(joined, core.ErrControlWireContract, scalar)
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
