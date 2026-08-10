package filestore

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
)

const (
	temporaryIdentityDiagnostic = "filestore temporary name identifies a different file"
	stagedReceiptDiagnostic     = "filestore staged file receipt is inconsistent"
)

// StagedFile is an opaque receipt for one synchronized caller-named temporary
// file. It is not a reader, writer, or filesystem substitute.
type StagedFile struct {
	info  fs.FileInfo
	root  *os.Root
	path  core.RelativePath
	bytes core.ByteLength
}

// Path returns the owned temporary name.
func (s StagedFile) Path() core.RelativePath {
	return s.path
}

// BytesWritten returns the exact staged byte length.
func (s StagedFile) BytesWritten() core.ByteLength {
	return s.bytes
}

// Validate rejects an unset or internally inconsistent receipt.
func (s StagedFile) Validate() error {
	if s.root == nil || s.info == nil {
		return contractError(errors.New("filestore staged file is unset"))
	}
	if err := validateMutablePath(s.path); err != nil {
		return err
	}
	size := s.info.Size()
	if !s.info.Mode().IsRegular() || size < 0 {
		return contractError(errors.New(stagedReceiptDiagnostic))
	}
	if uint64(size) != s.bytes.Uint64() {
		return contractError(errors.New(stagedReceiptDiagnostic))
	}
	return nil
}

// Stage streams one bounded source into an exclusively created, synchronized
// real file.
func Stage(ctx context.Context, request StageRequest) (StagedFile, error) {
	if err := contextstate.Validate(ctx); err != nil {
		return StagedFile{}, err
	}
	if err := request.Validate(); err != nil {
		return StagedFile{}, err
	}
	file, err := request.Temporary.Root.OpenFile(
		request.Temporary.Path.String(),
		os.O_WRONLY|os.O_CREATE|os.O_EXCL,
		request.Mode,
	)
	if err != nil {
		return StagedFile{}, classifyCreateError(err)
	}
	createdInfo, err := file.Stat()
	if err != nil {
		return StagedFile{}, abandonCreatedFile(
			request.Temporary,
			file,
			nil,
			activationError(err),
		)
	}
	staged, err := finishStage(ctx, request, file, createdInfo)
	if err != nil {
		return StagedFile{}, err
	}
	return staged, nil
}

func finishStage(
	ctx context.Context,
	request StageRequest,
	file *os.File,
	createdInfo fs.FileInfo,
) (StagedFile, error) {
	if err := file.Chmod(request.Mode); err != nil {
		return StagedFile{}, abandonCreatedFile(
			request.Temporary, file, createdInfo,
			activationError(err),
		)
	}
	written, err := copyBounded(
		ctx,
		file,
		request.Source,
		request.MaximumBytes,
		streamDestinationFile,
	)
	if err != nil {
		return StagedFile{}, abandonCreatedFile(
			request.Temporary, file, createdInfo, err,
		)
	}
	return synchronizeStage(request, file, createdInfo, written)
}

func synchronizeStage(
	request StageRequest,
	file *os.File,
	createdInfo fs.FileInfo,
	written core.ByteLength,
) (StagedFile, error) {
	if err := file.Sync(); err != nil {
		return StagedFile{}, abandonCreatedFile(
			request.Temporary, file, createdInfo,
			activationError(err),
		)
	}
	completedInfo, err := file.Stat()
	if err != nil {
		return StagedFile{}, abandonCreatedFile(
			request.Temporary, file, createdInfo,
			activationError(err),
		)
	}
	if err := file.Close(); err != nil {
		return StagedFile{}, cleanupCreatedPath(
			request.Temporary.Root,
			request.Temporary.Path,
			createdInfo,
			activationError(err),
		)
	}
	if err := syncParent(request.Temporary.Root, request.Temporary.Path); err != nil {
		return StagedFile{}, cleanupCreatedPath(
			request.Temporary.Root,
			request.Temporary.Path,
			createdInfo,
			activationError(err),
		)
	}
	return StagedFile{
		root:  request.Temporary.Root,
		path:  request.Temporary.Path,
		bytes: written,
		info:  completedInfo,
	}, nil
}

// Write performs Stage and Commit for a target known before streaming. A
// nonzero returned CommitRequest owns recovery after an unresolved error.
func Write(
	ctx context.Context,
	request WriteRequest,
) (CommitRequest, error) {
	if err := contextstate.Validate(ctx); err != nil {
		return CommitRequest{}, err
	}
	if err := request.Validate(); err != nil {
		return CommitRequest{}, err
	}
	staged, err := Stage(ctx, StageRequest{
		Source: request.Source,
		Temporary: Location{
			Root: request.Location.Root,
			Path: request.Temporary,
		},
		Mode:         request.Mode,
		MaximumBytes: request.MaximumBytes,
	})
	if err != nil {
		return CommitRequest{}, err
	}
	commit := CommitRequest{
		Staged:  staged,
		Target:  request.Location.Path,
		Install: request.Install,
	}
	err = Commit(ctx, commit)
	if err == nil {
		return CommitRequest{}, nil
	}
	return resolveFailedWrite(commit, err)
}

func resolveFailedWrite(
	request CommitRequest,
	commitErr error,
) (CommitRequest, error) {
	if errors.Is(commitErr, core.ErrFilestoreActivationIndeterminate) ||
		errors.Is(commitErr, core.ErrFilestoreCleanup) {
		return request, commitErr
	}
	cleanupErr := removeExpectedPath(
		request.Staged.root,
		request.Staged.path,
		request.Staged.info,
	)
	if cleanupErr != nil {
		return request, errors.Join(commitErr, cleanupErr)
	}
	return CommitRequest{}, commitErr
}

// Commit atomically activates one synchronized stage.
func Commit(ctx context.Context, request CommitRequest) error {
	if err := contextstate.Validate(ctx); err != nil {
		return err
	}
	if err := request.Validate(); err != nil {
		return err
	}
	if err := validateCurrentStage(request.Staged); err != nil {
		return err
	}
	if request.Install == InstallCreate {
		return commitCreate(request)
	}
	return commitReplace(request)
}

func commitCreate(request CommitRequest) error {
	err := request.Staged.root.Link(
		request.Staged.path.String(),
		request.Target.String(),
	)
	if err != nil {
		if errors.Is(err, fs.ErrExist) {
			return conflictError(err)
		}
		return activationError(err)
	}
	if err := syncParent(request.Staged.root, request.Target); err != nil {
		return indeterminateActivationError(err)
	}
	if err := removeStageName(request.Staged); err != nil {
		return err
	}
	if err := syncParent(request.Staged.root, request.Staged.path); err != nil {
		return cleanupError(err)
	}
	return nil
}

func commitReplace(request CommitRequest) error {
	err := request.Staged.root.Rename(
		request.Staged.path.String(),
		request.Target.String(),
	)
	if err != nil {
		return activationError(err)
	}
	if err := syncParent(request.Staged.root, request.Target); err != nil {
		return indeterminateActivationError(err)
	}
	if differentParentDirectories(request.Staged.path, request.Target) {
		if err := syncParent(request.Staged.root, request.Staged.path); err != nil {
			return indeterminateActivationError(err)
		}
	}
	return nil
}

// Recover re-observes the real stage and target names and completes the exact
// Commit request without a public state machine.
func Recover(ctx context.Context, request CommitRequest) error {
	if err := contextstate.Validate(ctx); err != nil {
		return err
	}
	if err := request.Validate(); err != nil {
		return err
	}
	stageInfo, err := statIfExists(request.Staged.root, request.Staged.path)
	if err != nil {
		return activationError(err)
	}
	targetInfo, err := statIfExists(request.Staged.root, request.Target)
	if err != nil {
		return activationError(err)
	}
	if stageInfo == nil {
		return recoverMissingStage(request, targetInfo)
	}
	if !os.SameFile(request.Staged.info, stageInfo) {
		return indeterminateActivationError(errors.New(temporaryIdentityDiagnostic))
	}
	return recoverPresentStage(request, targetInfo)
}

func recoverMissingStage(request CommitRequest, targetInfo fs.FileInfo) error {
	if targetInfo == nil {
		return activationError(fs.ErrNotExist)
	}
	if !os.SameFile(request.Staged.info, targetInfo) {
		return indeterminateActivationError(errors.New("filestore target identifies a different file"))
	}
	if err := syncParent(request.Staged.root, request.Target); err != nil {
		return indeterminateActivationError(err)
	}
	if differentParentDirectories(request.Staged.path, request.Target) {
		if err := syncParent(request.Staged.root, request.Staged.path); err != nil {
			return indeterminateActivationError(err)
		}
	}
	return nil
}

func recoverPresentStage(request CommitRequest, targetInfo fs.FileInfo) error {
	if targetInfo == nil {
		if request.Install == InstallCreate {
			return commitCreate(request)
		}
		return commitReplace(request)
	}
	if request.Install == InstallReplace {
		return commitReplace(request)
	}
	if !os.SameFile(request.Staged.info, targetInfo) {
		return conflictError(os.ErrExist)
	}
	if err := syncParent(request.Staged.root, request.Target); err != nil {
		return indeterminateActivationError(err)
	}
	if err := removeStageName(request.Staged); err != nil {
		return err
	}
	if err := syncParent(request.Staged.root, request.Staged.path); err != nil {
		return cleanupError(err)
	}
	return nil
}

func differentParentDirectories(left, right core.RelativePath) bool {
	return filepath.Dir(left.String()) != filepath.Dir(right.String())
}

// Discard durably removes only the name owned by one staged receipt.
func Discard(ctx context.Context, staged StagedFile) error {
	if err := contextstate.Validate(ctx); err != nil {
		return err
	}
	if err := staged.Validate(); err != nil {
		return err
	}
	info, err := statIfExists(staged.root, staged.path)
	if err != nil {
		return cleanupError(err)
	}
	if info != nil && !os.SameFile(staged.info, info) {
		return cleanupError(conflictError(errors.New(temporaryIdentityDiagnostic)))
	}
	if info != nil {
		if err := staged.root.Remove(staged.path.String()); err != nil &&
			!errors.Is(err, fs.ErrNotExist) {
			return cleanupError(err)
		}
	}
	if err := syncParent(staged.root, staged.path); err != nil {
		return cleanupError(err)
	}
	return nil
}

func validateCurrentStage(staged StagedFile) error {
	info, err := staged.root.Stat(staged.path.String())
	if err != nil {
		return activationError(err)
	}
	if !os.SameFile(staged.info, info) {
		return indeterminateActivationError(errors.New(temporaryIdentityDiagnostic))
	}
	if !info.Mode().IsRegular() || info.Mode().Perm() != staged.info.Mode().Perm() {
		return activationError(errors.New("filestore staged file permissions or type changed"))
	}
	observedBytes, err := core.CheckedUint64FromInt64(info.Size())
	if err != nil || observedBytes != staged.bytes.Uint64() {
		return sizeError(errors.Join(errors.New("filestore staged file extent changed"), err))
	}
	return nil
}

func removeStageName(staged StagedFile) error {
	err := staged.root.Remove(staged.path.String())
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return cleanupError(err)
	}
	return nil
}

func statIfExists(root *os.Root, path core.RelativePath) (fs.FileInfo, error) {
	info, err := root.Stat(path.String())
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	return info, err
}

func classifyCreateError(err error) error {
	if errors.Is(err, fs.ErrExist) {
		return conflictError(err)
	}
	return activationError(err)
}

func abandonCreatedFile(
	location Location,
	file *os.File,
	expected fs.FileInfo,
	primary error,
) error {
	closeErr := file.Close()
	return cleanupCreatedPath(
		location.Root,
		location.Path,
		expected,
		errors.Join(primary, classifyOptionalActivationError(closeErr)),
	)
}

func cleanupCreatedPath(
	root *os.Root,
	path core.RelativePath,
	expected fs.FileInfo,
	primary error,
) error {
	cleanupErr := removeExpectedPath(root, path, expected)
	return errors.Join(primary, cleanupErr)
}

func removeExpectedPath(root *os.Root, path core.RelativePath, expected fs.FileInfo) error {
	current, err := statIfExists(root, path)
	if err != nil {
		return cleanupError(err)
	}
	if current == nil {
		return nil
	}
	if expected != nil && !os.SameFile(expected, current) {
		return cleanupError(conflictError(errors.New("filestore cleanup name identifies a different file")))
	}
	if err := root.Remove(path.String()); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return cleanupError(err)
	}
	if err := syncParent(root, path); err != nil {
		return cleanupError(err)
	}
	return nil
}

func classifyOptionalActivationError(err error) error {
	if err == nil {
		return nil
	}
	return activationError(err)
}

func syncParent(root *os.Root, path core.RelativePath) error {
	parent, err := core.ParseRelativePath(filepath.Dir(path.String()))
	if err != nil {
		return contractError(err)
	}
	return syncDirectory(root, parent)
}

func syncDirectory(root *os.Root, path core.RelativePath) error {
	directory, err := root.Open(path.String())
	if err != nil {
		return err
	}
	syncErr := directory.Sync()
	closeErr := directory.Close()
	return errors.Join(syncErr, closeErr)
}

var _ core.Validatable = StagedFile{}
