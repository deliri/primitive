package providerwire

import (
	"errors"

	"github.com/deliri/primitive/v2026/core"
)

func contractError(cause error) error {
	return errors.Join(core.ErrProviderWireContract, cause)
}

func authenticationError(cause error) error {
	return errors.Join(core.ErrProviderWireAuthentication, cause)
}

func verificationError(cause error) error {
	return errors.Join(core.ErrProviderWireVerification, cause)
}

func bindingError(cause error) error {
	return errors.Join(core.ErrProviderWireBinding, cause)
}
