package filestore

import (
	"context"
	"errors"
	"io/fs"
	"os"

	"github.com/deliri/primitive/v2026/contextstate"
)

// Touch sets the access and modification times of one rooted regular entry,
// then requires successful Go file and parent-directory synchronization. It
// refuses a symbolic-link leaf and checks the opened file against the entry.
// Chtimes is a name-based Go operation; callers must coordinate concurrent
// namespace changes. Success acknowledges OS synchronization, not a hardware
// guarantee against every power-loss mode.
func Touch(ctx context.Context, request TouchRequest) error {
	if err := contextstate.Validate(ctx); err != nil {
		return err
	}
	if err := request.Validate(); err != nil {
		return err
	}
	stamp, err := request.ModifiedAt.Time()
	if err != nil {
		return contractError(err)
	}
	root, name := request.Location.Root, request.Location.Path
	file, err := openCustodyFile(root, name.String())
	if err != nil {
		return err
	}
	if err := root.Chtimes(name.String(), stamp, stamp); err != nil {
		return errors.Join(activationError(err), file.Close())
	}
	if err := syncCloseCustodyFile(file); err != nil {
		return err
	}
	if err := syncParent(root, name); err != nil {
		return activationError(err)
	}
	return nil
}

// syncCloseCustodyFile settles this operation's Go handle even when Sync fails.
func syncCloseCustodyFile(file *os.File) error {
	syncErr := file.Sync()
	closeErr := file.Close()
	if err := errors.Join(syncErr, closeErr); err != nil {
		return activationError(err)
	}
	return nil
}

// ConfirmDurable requires successful Go synchronization of one rooted regular
// file and its parent directory without changing its bytes or timestamps. It
// refuses symbolic-link leaves and preserves synchronization and close errors.
// Success acknowledges what the OS reports; storage hardware determines the
// resulting power-loss guarantees. Callers coordinate concurrent namespace edits.
func ConfirmDurable(ctx context.Context, request DurabilityRequest) error {
	if err := contextstate.Validate(ctx); err != nil {
		return err
	}
	if err := request.Validate(); err != nil {
		return err
	}
	root, name := request.Location.Root, request.Location.Path
	file, err := openCustodyFile(root, name.String())
	if err != nil {
		return err
	}
	if err := syncCloseCustodyFile(file); err != nil {
		return err
	}
	if err := syncParent(root, name); err != nil {
		return activationError(err)
	}
	return nil
}

// OpenLockFile opens or creates one rooted file whose only purpose is to carry
// an advisory lock, and hands the caller the real OS handle filelock requires.
//
// filelock.Request takes an *os.File and this package produces one nowhere
// else: OpenRead refuses a file that does not exist yet, and OpenAppend hands
// back an append-only handle a lock holder cannot rewrite its own diagnostics
// through. So every product that takes a file lock opens the file with
// os.OpenFile on a bare string path, which leaves the rooted boundary and
// drags a static-analysis suppression along with it. The two capabilities were
// built to meet and had nothing to meet through.
//
// The handle is opened for reading and writing so the holder can record who it
// is, and created if absent because a lock file's existence is not evidence of
// anything. The caller owns the handle: release the lock, then close it.
func OpenLockFile(ctx context.Context, request LockFileRequest) (*os.File, error) {
	if err := contextstate.Validate(ctx); err != nil {
		return nil, err
	}
	if err := request.Validate(); err != nil {
		return nil, err
	}
	file, err := openMutableFile(rootedOpenRequest{
		root: request.Location.Root, path: request.Location.Path.String(),
		flag: os.O_CREATE | os.O_RDWR, mode: request.Mode,
	})
	if err != nil {
		return nil, destinationError(err)
	}
	if err := validateLockFile(file); err != nil {
		return nil, err
	}
	if err := prepareRegularReadFile(file); err != nil {
		return nil, closeCustodyFile(file, destinationError(err))
	}
	if err := file.Chmod(request.Mode); err != nil {
		return nil, closeCustodyFile(file, destinationError(err))
	}
	return file, nil
}

func validateLockFile(file *os.File) error {
	info, err := file.Stat()
	if err != nil {
		return closeCustodyFile(file, destinationError(err))
	}
	if !info.Mode().IsRegular() {
		return closeCustodyFile(file, destinationError(fs.ErrInvalid))
	}
	return nil
}

func closeCustodyFile(file *os.File, primary error) error {
	closeErr := file.Close()
	if closeErr != nil {
		closeErr = destinationError(closeErr)
	}
	return errors.Join(primary, closeErr)
}

// openCustodyFile binds the no-follow namespace observation to the actual Go
// handle without changing ordinary OpenRead's confined-symlink semantics.
func openCustodyFile(root *os.Root, name string) (*os.File, error) {
	entry, err := root.Lstat(name)
	if err != nil {
		return nil, sourceError(err)
	}
	if !entry.Mode().IsRegular() {
		return nil, sourceError(fs.ErrInvalid)
	}
	file, observed, err := openRegularReadFile(root, name)
	if err != nil {
		return nil, err
	}
	if !os.SameFile(entry, observed) {
		return nil, closeReadFile(file, sourceError(fs.ErrInvalid))
	}
	return file, nil
}
