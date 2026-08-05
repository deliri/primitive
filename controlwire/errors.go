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

// policyCursorError reports a rejected policy revision, activation, or cursor.
func policyCursorError(causes ...error) error {
	return scalarError(core.ErrControlWirePolicyCursor, causes...)
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

// routeError reports a rejected control-plane route contract.
func routeError(causes ...error) error {
	return scalarError(core.ErrControlWireRoute, causes...)
}

// exchangePolicyError reports a control-exchange policy that could not be
// assembled or a document ceiling that could not bound anything.
//
// It carries the package contract identity and no scalar. The bounds are
// published constants of this package, so a rejection here names a defect in
// the control wire itself rather than a value some caller got wrong, and
// borrowing the route scalar would send a reader looking at the wrong thing.
func exchangePolicyError(causes ...error) error {
	joined := make([]error, 0, len(causes)+1)
	joined = append(joined, core.ErrControlWireContract)
	for _, cause := range causes {
		if cause != nil {
			joined = append(joined, cause)
		}
	}
	return errors.Join(joined...)
}
