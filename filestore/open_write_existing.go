package filestore

import (
	"context"
	"io/fs"
	"os"

	"github.com/deliri/primitive/v2026/contextstate"
)

// OpenWriteExisting acquires an existing regular file for write-only native
// updates. It does not create, truncate, append, replace, synchronize or close it.
// Unlike OpenUpdate it needs no read permission, including for write-only
// virtual control files. Acquisition is nonblocking until native kind admission.
// The caller owns the returned Go handle and the format/meaning of its writes.
func OpenWriteExisting(ctx context.Context, request UpdateHandleRequest) (*os.File, error) {
	if err := contextstate.Validate(ctx); err != nil {
		return nil, err
	}
	if err := request.Validate(); err != nil {
		return nil, err
	}
	file, err := openMutableFile(rootedOpenRequest{root: request.Location.Root, path: request.Location.Path.String(), flag: os.O_WRONLY})
	if err != nil {
		return nil, activationError(err)
	}
	info, err := file.Stat()
	if err != nil {
		return nil, closeReadFile(file, activationError(err))
	}
	if !info.Mode().IsRegular() {
		return nil, closeReadFile(file, activationError(fs.ErrInvalid))
	}
	if err := prepareRegularReadFile(file); err != nil {
		return nil, closeReadFile(file, activationError(err))
	}
	return file, nil
}
