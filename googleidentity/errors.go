package googleidentity

import (
	"errors"

	"github.com/deliri/primitive/v2026/core"
)

func contractError(cause error) error {
	if cause == nil {
		return core.ErrGoogleIdentityContract
	}
	return errors.Join(core.ErrGoogleIdentityContract, cause)
}
