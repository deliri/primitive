package about

import (
	"errors"

	"github.com/deliri/primitive/v2026/core"
)

func contractError(causes ...error) error {
	return errors.Join(append([]error{core.ErrAboutContract}, causes...)...)
}

func conflictError(causes ...error) error {
	return errors.Join(append([]error{core.ErrAboutConflict}, causes...)...)
}

func transportError(causes ...error) error {
	return errors.Join(append([]error{core.ErrAboutTransport}, causes...)...)
}

func jsonError(causes ...error) error {
	return errors.Join(core.ErrJSONContract, contractError(causes...))
}
