//go:build !darwin && !linux

package filestore

import (
	"errors"
	"os"
)

func duplicateBlockingFile(*os.File) (*os.File, error) {
	return nil, sourceError(errors.ErrUnsupported)
}
