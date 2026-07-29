package filestore

import (
	"context"
	"errors"
	"io/fs"
	"os"

	"github.com/deliri/primitive/v2026/contextstate"
)

// OpenAppend returns one caller-owned real OS append handle.
func OpenAppend(ctx context.Context, request AppendRequest) (*os.File, error) {
	if err := contextstate.Validate(ctx); err != nil {
		return nil, err
	}
	if err := request.Validate(); err != nil {
		return nil, err
	}
	file, err := createAppend(request)
	if err == nil {
		return file, nil
	}
	if !errors.Is(err, fs.ErrExist) {
		return nil, err
	}
	file, err = request.Location.Root.OpenFile(
		request.Location.Path.String(),
		os.O_WRONLY|os.O_APPEND,
		0,
	)
	if err != nil {
		return nil, activationError(err)
	}
	if err := validateAppendFile(file); err != nil {
		return nil, err
	}
	return file, nil
}

// RotateAppend synchronizes and closes the transferred outgoing handle, then
// exclusively creates and returns one caller-owned incoming handle.
func RotateAppend(ctx context.Context, request RotationRequest) (*os.File, error) {
	if err := contextstate.Validate(ctx); err != nil {
		return nil, err
	}
	if err := request.Validate(); err != nil {
		return nil, err
	}
	if err := synchronizeAndCloseOutgoing(request.Outgoing); err != nil {
		return nil, err
	}
	return createAppend(request.Incoming)
}

func createAppend(request AppendRequest) (*os.File, error) {
	file, err := request.Location.Root.OpenFile(
		request.Location.Path.String(),
		os.O_WRONLY|os.O_APPEND|os.O_CREATE|os.O_EXCL,
		request.Mode,
	)
	if err != nil {
		return nil, classifyCreateError(err)
	}
	createdInfo, err := file.Stat()
	if err != nil {
		return nil, abandonCreatedFile(
			request.Location,
			file,
			nil,
			activationError(err),
		)
	}
	if err := prepareCreatedAppend(request, file); err != nil {
		return nil, abandonCreatedFile(
			request.Location,
			file,
			createdInfo,
			err,
		)
	}
	return file, nil
}

func prepareCreatedAppend(request AppendRequest, file *os.File) error {
	if err := file.Chmod(request.Mode); err != nil {
		return activationError(err)
	}
	if err := file.Sync(); err != nil {
		return activationError(err)
	}
	if err := syncParent(request.Location.Root, request.Location.Path); err != nil {
		return activationError(err)
	}
	return nil
}

func validateAppendFile(file *os.File) error {
	info, err := file.Stat()
	if err != nil {
		return closeAppendFile(file, activationError(err))
	}
	if !info.Mode().IsRegular() {
		return closeAppendFile(file, activationError(fs.ErrInvalid))
	}
	return nil
}

func synchronizeAndCloseOutgoing(file *os.File) error {
	syncErr := file.Sync()
	closeErr := file.Close()
	if syncErr == nil && closeErr == nil {
		return nil
	}
	return activationError(errors.Join(syncErr, closeErr))
}

func closeAppendFile(file *os.File, primary error) error {
	closeErr := file.Close()
	if closeErr != nil {
		closeErr = activationError(closeErr)
	}
	return errors.Join(primary, closeErr)
}

// Remove durably removes one named file or empty directory. It never recurses.
func Remove(ctx context.Context, request RemovalRequest) error {
	if err := contextstate.Validate(ctx); err != nil {
		return err
	}
	if err := request.Validate(); err != nil {
		return err
	}
	err := request.Location.Root.Remove(request.Location.Path.String())
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return cleanupError(err)
	}
	if err := syncParent(request.Location.Root, request.Location.Path); err != nil {
		return cleanupError(err)
	}
	return nil
}
