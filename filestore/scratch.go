package filestore

import (
	"context"
	"errors"
	"io/fs"
	"os"

	"github.com/deliri/primitive/v2026/contextstate"
)

// ScratchRequest exclusively creates a disposable regular file. The caller
// owns the returned standard-library handle and closes it. No durability is
// promised: a caller needing durable publication uses Stage and Commit.
type ScratchRequest struct {
	Location Location
	Mode     fs.FileMode
}

func (r ScratchRequest) Validate() error {
	if err := r.Location.Validate(); err != nil {
		return err
	}
	if err := validateMutablePath(r.Location.Path); err != nil {
		return err
	}
	return validatePermissionMode(r.Mode)
}

// OpenScratch creates one new file without synchronizing it or its parent.
// O_EXCL preserves any existing file, directory, symlink, or special entry.
func OpenScratch(ctx context.Context, request ScratchRequest) (*os.File, error) {
	if err := contextstate.Validate(ctx); err != nil {
		return nil, err
	}
	if err := request.Validate(); err != nil {
		return nil, err
	}
	file, err := request.Location.Root.OpenFile(request.Location.Path.String(), os.O_WRONLY|os.O_CREATE|os.O_EXCL, request.Mode)
	if err != nil {
		return nil, classifyCreateError(err)
	}
	created, err := file.Stat()
	if err != nil {
		return nil, abandonCreatedFile(createdFileAbandonment{location: request.Location, file: file, primary: activationError(err)})
	}
	if err := file.Chmod(request.Mode); err != nil {
		return nil, abandonCreatedFile(createdFileAbandonment{location: request.Location, file: file, expected: created, primary: activationError(err)})
	}
	return file, nil
}

// EnsureScratchDirectory prepares disposable directories without synchronizing
// their namespace. os.Root owns containment and traversal; the final directory
// receives the requested permissions just like EnsureDirectory.
func EnsureScratchDirectory(ctx context.Context, request DirectoryRequest) error {
	if err := contextstate.Validate(ctx); err != nil {
		return err
	}
	if err := request.Validate(); err != nil {
		return err
	}
	if err := request.Location.Root.MkdirAll(request.Location.Path.String(), request.Mode); err != nil {
		return activationError(err)
	}
	directory, err := openDirectory(request.Location.Root, request.Location.Path.String())
	if err != nil {
		return activationError(err)
	}
	if err := errors.Join(directory.Chmod(request.Mode), directory.Close()); err != nil {
		return activationError(err)
	}
	return nil
}
