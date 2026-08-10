package retrieval

import (
	"errors"

	"github.com/deliri/primitive/v2026/core"
)

func contractError(causes ...error) error {
	return errors.Join(append([]error{core.ErrRetrievalContract}, causes...)...)
}

func jsonError(causes ...error) error {
	return errors.Join(core.ErrJSONContract, contractError(causes...))
}

func bindingError(causes ...error) error {
	return errors.Join(append([]error{core.ErrRetrievalBinding}, causes...)...)
}
