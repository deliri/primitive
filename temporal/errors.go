package temporal

import (
	"errors"

	"github.com/deliri/primitive/v2026/core"
)

func contractError(reason string, causes ...error) error {
	joined := []error{core.ErrTemporalContract, errors.New(reason)}
	joined = append(joined, causes...)
	return errors.Join(joined...)
}

func overflowError(reason string) error {
	return errors.Join(core.ErrTemporalOverflow, errors.New(reason))
}

func jsonContractError(reason string, causes ...error) error {
	joined := []error{core.ErrTemporalContract, core.ErrJSONContract, errors.New(reason)}
	joined = append(joined, causes...)
	return errors.Join(joined...)
}
