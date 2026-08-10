package paymentauth

import (
	"errors"

	"github.com/deliri/primitive/v2026/core"
)

func contractError(causes ...error) error {
	return errors.Join(append([]error{core.ErrControlPlaneContract}, causes...)...)
}

func bindingError(causes ...error) error {
	return contractError(append([]error{core.ErrControlPlaneResponseBinding}, causes...)...)
}

func jsonError(causes ...error) error {
	return errors.Join(core.ErrJSONContract, contractError(causes...))
}
