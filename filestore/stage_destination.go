package filestore

import (
	"context"
	"errors"
	"io/fs"
	"os"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
)

// StageDestination is a linear O(1) ownership handle over one real Go file.
// It owns no buffering or writer model: File returns the standard-library
// handle, and FinishStageDestination or AbandonStageDestination settles it.
type StageDestination struct {
	self    *StageDestination
	file    *os.File
	created fs.FileInfo
	request StageDestinationRequest
}

// OpenStageDestination exclusively creates one caller-named temporary and
// hands its linear ownership handle to the external producer.
func OpenStageDestination(ctx context.Context, request StageDestinationRequest) (*StageDestination, error) {
	if err := contextstate.Validate(ctx); err != nil {
		return nil, err
	}
	if err := request.Validate(); err != nil {
		return nil, err
	}
	file, err := request.Temporary.Root.OpenFile(
		request.Temporary.Path.String(),
		os.O_WRONLY|os.O_CREATE|os.O_EXCL,
		request.Mode,
	)
	if err != nil {
		return nil, classifyCreateError(err)
	}
	created, err := file.Stat()
	if err != nil {
		return nil, abandonCreatedFile(request.Temporary, file, nil, activationError(err))
	}
	if err := file.Chmod(request.Mode); err != nil {
		return nil, abandonCreatedFile(request.Temporary, file, created, activationError(err))
	}
	destination := &StageDestination{file: file, created: created, request: request}
	destination.self = destination
	return destination, nil
}

// Validate rejects an unset, copied, or already-settled ownership handle.
func (d *StageDestination) Validate() error {
	if d == nil || d.self != d || d.file == nil || d.created == nil {
		return contractError(errors.New("filestore stage destination is unset, copied, or settled"))
	}
	return d.request.Validate()
}

// File returns the real caller-owned Go file. The caller may hand it directly
// to a streaming standard-library-compatible producer but must not close it;
// finish or abandon owns closure.
func (d *StageDestination) File() (*os.File, error) {
	if err := d.Validate(); err != nil {
		return nil, err
	}
	return d.file, nil
}

// FinishStageDestination synchronizes and closes the exact created inode,
// proves its declared byte length and custody name, then returns a StagedFile
// for atomic Commit.
func FinishStageDestination(ctx context.Context, destination *StageDestination) (StagedFile, error) {
	if err := contextstate.Validate(ctx); err != nil {
		return StagedFile{}, destination.fail(err)
	}
	if err := destination.Validate(); err != nil {
		return StagedFile{}, err
	}
	return finishStageDestination(destination)
}

func finishStageDestination(destination *StageDestination) (StagedFile, error) {
	if err := destination.file.Chmod(destination.request.Mode); err != nil {
		return StagedFile{}, destination.fail(activationError(err))
	}
	if err := destination.file.Sync(); err != nil {
		return StagedFile{}, destination.fail(activationError(err))
	}
	completed, err := destination.file.Stat()
	if err != nil {
		return StagedFile{}, destination.fail(activationError(err))
	}
	if err := destination.validateCompleted(completed); err != nil {
		return StagedFile{}, destination.fail(err)
	}
	if err := destination.file.Close(); err != nil {
		return StagedFile{}, destination.cleanupClosed(activationError(err))
	}
	if err := syncParent(destination.request.Temporary.Root, destination.request.Temporary.Path); err != nil {
		return StagedFile{}, destination.cleanupClosed(activationError(err))
	}
	staged := StagedFile{
		root: destination.request.Temporary.Root, path: destination.request.Temporary.Path,
		bytes: destination.request.ExpectedBytes, info: completed,
	}
	if err := validateCurrentStage(staged); err != nil {
		return StagedFile{}, destination.cleanupClosed(err)
	}
	destination.release()
	return staged, nil
}

func (d *StageDestination) validateCompleted(completed fs.FileInfo) error {
	if completed == nil || !completed.Mode().IsRegular() || !os.SameFile(d.created, completed) {
		return indeterminateActivationError(errors.New("filestore stage destination identity changed"))
	}
	if completed.Mode().Perm() != d.request.Mode {
		return activationError(errors.New("filestore stage destination permissions differ"))
	}
	completedBytes, err := core.CheckedUint64FromInt64(completed.Size())
	if err != nil || completedBytes != d.request.ExpectedBytes.Uint64() {
		return sizeError(errors.Join(errors.New("filestore stage destination extent differs"), err))
	}
	return nil
}

// AbandonStageDestination closes and removes only the exact inode created by
// OpenStageDestination. Cleanup remains available after operation cancellation.
func AbandonStageDestination(destination *StageDestination) error {
	if err := destination.Validate(); err != nil {
		return err
	}
	return destination.fail(nil)
}

func (d *StageDestination) fail(primary error) error {
	if err := d.Validate(); err != nil {
		return errors.Join(primary, err)
	}
	cleanupErr := abandonCreatedFile(d.request.Temporary, d.file, d.created, primary)
	d.release()
	return cleanupErr
}

func (d *StageDestination) cleanupClosed(primary error) error {
	cleanupErr := cleanupCreatedPath(
		d.request.Temporary.Root,
		d.request.Temporary.Path,
		d.created,
		primary,
	)
	d.release()
	return cleanupErr
}

func (d *StageDestination) release() {
	d.self = nil
	d.file = nil
	d.created = nil
}

var (
	_ core.Validatable = StageDestinationRequest{}
	_ core.Validatable = (*StageDestination)(nil)
)
