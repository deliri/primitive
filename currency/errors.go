package currency

import (
	"errors"

	"github.com/deliri/primitive/v2026/core"
)

func contractError(message string) error {
	return errors.Join(core.ErrCurrencyContract, errors.New(message))
}

func mismatchError() error {
	return errors.Join(core.ErrCurrencyMismatch, errors.New("currency codes do not match"))
}

func overflowError() error {
	return errors.Join(core.ErrCurrencyOverflow, errors.New("currency arithmetic exceeded int64"))
}

func decimalError(rejection decimalRejection) error {
	return errors.Join(core.ErrCurrencyDecimal, rejection)
}

func jsonError(cause error) error {
	return errors.Join(core.ErrJSONContract, cause)
}
