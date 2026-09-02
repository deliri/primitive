package stripe

import (
	"errors"

	"github.com/deliri/primitive/v2026/core"
)

func contractError(cause error) error { return providerError(core.ErrStripeContract, cause) }
func authenticationError(cause error) error {
	return providerError(core.ErrStripeAuthentication, cause)
}
func verificationError(cause error) error { return providerError(core.ErrStripeVerification, cause) }

func providerError(identity, cause error) error {
	if cause == nil {
		return identity
	}
	return errors.Join(identity, cause)
}
