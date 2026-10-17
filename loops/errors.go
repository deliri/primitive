package loops

import (
	"errors"

	"github.com/deliri/primitive/v2026/core"
)

func contractError(cause error) error       { return providerError(core.ErrLoopsContract, cause) }
func authenticationError(cause error) error { return providerError(core.ErrLoopsAuthentication, cause) }
func verificationError(cause error) error   { return providerError(core.ErrLoopsVerification, cause) }
func bindingError(cause error) error        { return providerError(core.ErrLoopsBinding, cause) }

func providerError(identity, cause error) error {
	if cause == nil {
		return identity
	}
	return errors.Join(identity, cause)
}
