package filestore

import (
	"errors"

	"github.com/deliri/primitive/v2026/core"
)

func contractError(err error) error {
	return errors.Join(core.ErrFilestoreContract, err)
}

func sizeError(err error) error {
	return errors.Join(core.ErrFilestoreSize, err)
}

func sourceError(err error) error {
	return errors.Join(core.ErrFilestoreSource, err)
}

func destinationError(err error) error {
	return errors.Join(core.ErrFilestoreDestination, err)
}

func conflictError(err error) error {
	return errors.Join(core.ErrFilestoreConflict, err)
}

func activationError(err error) error {
	return errors.Join(core.ErrFilestoreActivation, err)
}

func indeterminateActivationError(err error) error {
	return errors.Join(core.ErrFilestoreActivationIndeterminate, err)
}

func cleanupError(err error) error {
	return errors.Join(core.ErrFilestoreCleanup, err)
}
