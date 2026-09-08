package filestore

import (
	"context"
	"errors"
	"io/fs"
	"os"

	"github.com/deliri/primitive/v2026/contextstate"
)

// SetPermissions applies the requested Go permission mode to one rooted entry,
// then synchronizes and closes that same native handle. Confined symlinks follow
// Go's rooted-open semantics. Native acquisition refusals leave the mode
// untouched. Go decides which opened entries can synchronize; a sync/close
// failure after Chmod is an indeterminate activation with its native cause.
func SetPermissions(ctx context.Context, request PermissionRequest) error {
	if err := contextstate.Validate(ctx); err != nil {
		return err
	}
	if err := request.Validate(); err != nil {
		return err
	}
	file, err := openPermissionFile(request.Location)
	if err != nil {
		return activationError(err)
	}
	if err := prepareRegularReadFile(file); err != nil {
		return closeActivationFile(file, activationError(err))
	}
	if err := file.Chmod(request.Mode); err != nil {
		return closeActivationFile(file, activationError(err))
	}
	if err := syncCloseCustodyFile(file); err != nil {
		return indeterminateActivationError(err)
	}
	return nil
}

// Either Go access mode can hold an entry for Fsync. A permission-denied
// read open is the only reason to try write access; preserve both native errors
// when neither acquisition is permitted. No write or truncation is performed.
func openPermissionFile(location Location) (*os.File, error) {
	file, readErr := openReadFile(location.Root, location.Path.String())
	if !errors.Is(readErr, fs.ErrPermission) {
		return file, readErr
	}
	file, writeErr := openMutableFile(rootedOpenRequest{
		root: location.Root, path: location.Path.String(), flag: os.O_WRONLY,
	})
	if writeErr != nil {
		return nil, errors.Join(readErr, writeErr)
	}
	return file, nil
}

var _ interface{ Validate() error } = PermissionRequest{}
