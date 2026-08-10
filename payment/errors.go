package payment

import (
	"errors"

	"github.com/deliri/primitive/v2026/core"
)

func contractError(causes ...error) error {
	return errors.Join(append([]error{core.ErrPaymentContract}, causes...)...)
}

func jsonError(causes ...error) error {
	return errors.Join(core.ErrJSONContract, contractError(causes...))
}

func verificationError(causes ...error) error {
	return errors.Join(append([]error{core.ErrPaymentVerification}, causes...)...)
}
