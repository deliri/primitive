package projectstandards

import (
	"errors"

	"github.com/deliri/primitive/v2026/core"
)

func contractError(causes ...error) error {
	return errors.Join(append([]error{core.ErrProjectStandardsContract}, causes...)...)
}

func conflictError(causes ...error) error {
	return errors.Join(append([]error{core.ErrProjectStandardsConflict}, causes...)...)
}

func transportError(causes ...error) error {
	return errors.Join(append([]error{core.ErrProjectStandardsTransport}, causes...)...)
}

func jsonError(causes ...error) error {
	return errors.Join(core.ErrJSONContract, contractError(causes...))
}
