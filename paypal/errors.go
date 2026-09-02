package paypal

import (
	"errors"

	"github.com/deliri/primitive/v2026/core"
)

func contractError(cause error) error { return providerError(core.ErrPayPalContract, cause) }
func authenticationError(cause error) error {
	return providerError(core.ErrPayPalAuthentication, cause)
}
func verificationError(cause error) error { return providerError(core.ErrPayPalVerification, cause) }

func providerError(identity, cause error) error {
	if cause == nil {
		return identity
	}
	return errors.Join(identity, cause)
}
func bindingError(cause error) error { return errors.Join(core.ErrPayPalBinding, cause) }
