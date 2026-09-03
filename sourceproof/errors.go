package sourceproof

import (
	"errors"

	"github.com/deliri/primitive/v2026/core"
)

func contractError(cause error) error {
	return errors.Join(core.ErrSourceProofContract, cause)
}

func conflictError(cause error) error {
	return errors.Join(core.ErrSourceProofConflict, cause)
}

func contractJoin(values ...error) error {
	for _, value := range values {
		if value != nil {
			return contractError(errors.Join(values...))
		}
	}
	return nil
}
