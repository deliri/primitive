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
// real file. Caller panics propagate through Go. During unwinding, Filestore
// closes its handle and attempts to remove only its own temporary inode;
// cleanup errors cannot replace the caller's panic or produce a receipt.
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
		return StagedFile{}, abandonCreatedFile(createdFileAbandonment{
			location: request.Temporary, file: file, primary: activationError(err),
		})
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
		return StagedFile{}, abandonCreatedFile(createdFileAbandonment{
			location: request.Temporary, file: file, expected: createdInfo, primary: activationError(err),
		})
	}
	copyReturned := false
	defer func() {
		if !copyReturned {
			// No normal return exists on panic or Goexit. Preserve Go's unwind
			// while releasing custody; never remove a replacement inode.
			_ = abandonCreatedFile(createdFileAbandonment{
				location: request.Temporary, file: file, expected: createdInfo,
			})
		}
	}()
	written, err := copyBounded(boundedCopyRequest{
		ctx: ctx, destination: file, source: request.Source,
		maximum: request.MaximumBytes, kind: streamDestinationFile,
	})
	copyReturned = true
	if err != nil {
		return StagedFile{}, abandonCreatedFile(createdFileAbandonment{
			location: request.Temporary, file: file, expected: createdInfo, primary: err,
		})
	}
	return synchronizeStage(stageSynchronization{
		request: request, file: file, createdInfo: createdInfo, written: written,
	})
}

type stageSynchronization struct {
	createdInfo fs.FileInfo
	file        *os.File
	request     StageRequest
	written     core.ByteLength
}

func synchronizeStage(stage stageSynchronization) (StagedFile, error) {
	request, file, createdInfo, written := stage.request, stage.file, stage.createdInfo, stage.written
	if err := file.Sync(); err != nil {
		return StagedFile{}, abandonCreatedFile(createdFileAbandonment{
			location: request.Temporary, file: file, expected: createdInfo, primary: activationError(err),
		})
	}
	completedInfo, err := file.Stat()
	if err != nil {
		return StagedFile{}, abandonCreatedFile(createdFileAbandonment{
			location: request.Temporary, file: file, expected: createdInfo, primary: activationError(err),
		})
	}
	if err := file.Close(); err != nil {
		return StagedFile{}, cleanupCreatedPath(createdPathCleanup{
			root: request.Temporary.Root, path: request.Temporary.Path,
			expected: createdInfo, primary: activationError(err),
		})
	}
	if err := syncParent(request.Temporary.Root, request.Temporary.Path); err != nil {
		return StagedFile{}, cleanupCreatedPath(createdPathCleanup{
			root: request.Temporary.Root, path: request.Temporary.Path,
			expected: createdInfo, primary: activationError(err),
		})
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
	// Go's rename is a successful no-op when both names already identify one
	// inode. Settle a retained owned stage, preserving any foreign replacement.
	if err := removeExpectedPath(request.Staged.root, request.Staged.path, request.Staged.info); err != nil {
		return err
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
	stageInfo, err := lstatIfExists(request.Staged.root, request.Staged.path)
	if err != nil {
		return activationError(err)
	}
	targetInfo, err := lstatIfExists(request.Staged.root, request.Target)
	if err != nil {
		return activationError(err)
	}
	if stageInfo == nil {
		return recoverMissingStage(request, targetInfo)
	}
	if err := validateStagedObservation(request.Staged, stageInfo); err != nil {
		return err
	}
	return recoverPresentStage(request, targetInfo)
}

func recoverMissingStage(request CommitRequest, targetInfo fs.FileInfo) error {
	if targetInfo == nil {
		return activationError(fs.ErrNotExist)
	}
	if err := validateStagedObservation(request.Staged, targetInfo); err != nil {
		return err
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
	if err := validateStagedObservation(request.Staged, targetInfo); err != nil {
		return err
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
	info, err := lstatIfExists(staged.root, staged.path)
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
	info, err := staged.root.Lstat(staged.path.String())
	if err != nil {
		return activationError(err)
	}
	return validateStagedObservation(staged, info)
}

func validateStagedObservation(staged StagedFile, observed fs.FileInfo) error {
	if observed == nil || !os.SameFile(staged.info, observed) {
		return indeterminateActivationError(errors.New("filestore staged receipt identity changed"))
	}
	if !observed.Mode().IsRegular() || observed.Mode().Perm() != staged.info.Mode().Perm() {
		return activationError(errors.New("filestore staged receipt permissions or type changed"))
	}
	observedBytes, err := core.CheckedUint64FromInt64(observed.Size())
	if err != nil || observedBytes != staged.bytes.Uint64() {
		return sizeError(errors.Join(errors.New("filestore staged receipt extent changed"), err))
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

// Namespace effects act on the entry itself. Following a symbolic link here
// would lend it a different inode's receipt or turn a dangling entry into absence.
func lstatIfExists(root *os.Root, path core.RelativePath) (fs.FileInfo, error) {
	info, err := root.Lstat(path.String())
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

type createdFileAbandonment struct {
	expected fs.FileInfo
	primary  error
	file     *os.File
	location Location
}

func abandonCreatedFile(request createdFileAbandonment) error {
	closeErr := request.file.Close()
	return cleanupCreatedPath(createdPathCleanup{
		root: request.location.Root, path: request.location.Path, expected: request.expected,
		primary: errors.Join(request.primary, classifyOptionalActivationError(closeErr)),
	})
}

type createdPathCleanup struct {
	expected fs.FileInfo
	primary  error
	root     *os.Root
	path     core.RelativePath
}

func cleanupCreatedPath(request createdPathCleanup) error {
	cleanupErr := removeExpectedPath(request.root, request.path, request.expected)
	return errors.Join(request.primary, cleanupErr)
}

func removeExpectedPath(root *os.Root, path core.RelativePath, expected fs.FileInfo) error {
	current, err := lstatIfExists(root, path)
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
	directory, err := openDirectory(root, path.String())
	if err != nil {
		return err
	}
	syncErr := directory.Sync()
	closeErr := directory.Close()
	return errors.Join(syncErr, closeErr)
}

var _ core.Validatable = StagedFile{}
