//go:build !linux || (!amd64 && !arm64)

package filestore

import (
	"context"
	"errors"
	"os"
)

func filesystemDurabilityPlatform() error { return activationError(errors.ErrUnsupported) }

func synchronizeFilesystem(context.Context, *os.File) error {
	return filesystemDurabilityPlatform()
}
