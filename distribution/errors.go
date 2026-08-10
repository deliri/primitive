package distribution

import (
	"errors"

	"github.com/deliri/primitive/v2026/core"
)

func contractError(causes ...error) error {
	values := []error{core.ErrDistributionContract, core.ErrPrimitiveContract}
	return errors.Join(append(values, causes...)...)
}

func verificationError(causes ...error) error {
	return contractError(append([]error{core.ErrDistributionVerification}, causes...)...)
}

func bindingError(causes ...error) error {
	return contractError(append([]error{core.ErrDistributionBinding}, causes...)...)
}

func jsonError(causes ...error) error {
	return errors.Join(core.ErrJSONContract, contractError(causes...))
}
