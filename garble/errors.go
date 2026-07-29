package garble

import (
	"errors"

	"github.com/deliri/primitive/v2026/core"
)

func contractError(errs ...error) error {
	return joinIdentity(core.ErrGarbleContract, errs...)
}

func derivationError(errs ...error) error {
	return joinIdentity(core.ErrGarbleDerivation, errs...)
}

func buildIntentError(errs ...error) error {
	return joinIdentity(core.ErrGarbleBuildIntent, errs...)
}

func jsonError(errs ...error) error {
	return errors.Join(core.ErrJSONContract, contractError(errs...))
}

func joinIdentity(identity error, errs ...error) error {
	values := make([]error, 1, len(errs)+1)
	values[0] = identity
	for _, err := range errs {
		if err != nil {
			values = append(values, err)
		}
	}
	return errors.Join(values...)
}
