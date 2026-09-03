package sourceclaim

import (
	"errors"

	"github.com/deliri/primitive/v2026/core"
)

func contractError(cause error) error {
	return errors.Join(core.ErrSourceClaimContract, cause)
}

func conflictError(cause error) error {
	return errors.Join(core.ErrSourceClaimConflict, cause)
}
