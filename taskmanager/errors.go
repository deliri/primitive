package taskmanager

import (
	"errors"

	"github.com/deliri/primitive/v2026/core"
)

func contractError(causes ...error) error {
	joined := errors.Join(causes...)
	if joined == nil {
		return core.ErrTaskManagerContract
	}
	return errors.Join(core.ErrTaskManagerContract, joined)
}

func jsonContractError(causes ...error) error {
	return errors.Join(core.ErrJSONContract, contractError(causes...))
}
