package keygen

import (
	"errors"

	"github.com/deliri/primitive/v2026/core"
)

func contractError(errs ...error) error {
	return joinIdentity(core.ErrKeygenContract, errs...)
}

func entropyError(errs ...error) error {
	return joinIdentity(core.ErrKeygenEntropy, errs...)
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
