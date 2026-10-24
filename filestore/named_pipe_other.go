//go:build !darwin && !linux

package filestore

import (
	"errors"
	"os"
)

func createNamedPipe(NamedPipeRequest) error {
	return activationError(errors.ErrUnsupported)
}

func openNamedPipeReadWrite(Location) (*os.File, error) {
	return nil, sourceError(errors.ErrUnsupported)
}
