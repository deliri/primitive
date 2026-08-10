package submission

import (
	"errors"

	"github.com/deliri/primitive/v2026/core"
)

func contractError(causes ...error) error {
	joined := make([]error, 0, len(causes)+1)
	joined = append(joined, core.ErrControlPlaneContract)
	for _, cause := range causes {
		if cause != nil {
			joined = append(joined, cause)
		}
	}
	return errors.Join(joined...)
}

func bindingError(causes ...error) error {
	return contractError(append([]error{core.ErrControlPlaneResponseBinding}, causes...)...)
}

func jsonError(causes ...error) error {
	return errors.Join(core.ErrJSONContract, contractError(causes...))
}
