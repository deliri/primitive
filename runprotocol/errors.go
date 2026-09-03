package runprotocol

import (
	"errors"

	"github.com/deliri/primitive/v2026/core"
)

func contractError(causes ...error) error {
	return errors.Join(append([]error{core.ErrRunProtocolContract}, causes...)...)
}

func conflictError(causes ...error) error {
	return errors.Join(append([]error{core.ErrRunProtocolConflict}, causes...)...)
}

func jsonError(causes ...error) error {
	return errors.Join(core.ErrJSONContract, contractError(causes...))
}

func aboutJSONLimits() core.StrictJSONLimits {
	return core.DefaultStrictJSONLimits()
}
