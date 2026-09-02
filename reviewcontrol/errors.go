package reviewcontrol

import (
	"errors"

	"github.com/deliri/primitive/v2026/core"
)

func contractError(causes ...error) error {
	return errors.Join(append([]error{core.ErrReviewControlContract}, causes...)...)
}

func jsonError(causes ...error) error {
	return errors.Join(append([]error{core.ErrJSONContract, core.ErrReviewControlContract}, causes...)...)
}

func validateContract(causes ...error) error {
	if err := errors.Join(causes...); err != nil {
		return contractError(err)
	}
	return nil
}
