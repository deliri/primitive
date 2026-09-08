package filestore

import (
	"context"
	"errors"
	"io/fs"
	"os"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
)

type rootedOpenRequest struct {
	root *os.Root
	path string
	flag int
	mode os.FileMode
}

// OpenRead opens one existing regular file for reading below a rooted
// boundary and hands the caller the real OS handle.
//
// Read already streams a file into an io.Writer, which serves a caller that
// owns the destination. It does not serve a caller that must hand a reader to
// something else — a hasher, a bounded upload, a content-addressed ingest —
// because there is no way to turn a writer destination back into a source
// without buffering the whole file or standing up a pipe and a goroutine.
// Products facing that reach for os.Open, and reaching past the rooted
// boundary is exactly what this package exists to prevent. OpenAppend already
// returns a real handle for the write side; this is its read counterpart.
//
// The caller owns the returned handle and must close it. A path that resolves
// to a non-regular file is refused before any bytes are read. The rooted OS
// capability may follow a symbolic link confined beneath the root, while a
// dangling link or one that escapes the root is refused.
func OpenRead(ctx context.Context, request ReadHandleRequest) (*os.File, error) {
	if err := contextstate.Validate(ctx); err != nil {
		return nil, err
	}
	if err := request.Validate(); err != nil {
		return nil, err
	}
	file, _, err := openRegularReadFile(
		request.Location.Root,
		request.Location.Path.String(),
	)
	return file, err
}

// OpenUpdate opens one existing regular file below a rooted boundary for
// caller-owned reading and in-place updates. The caller owns and must close the
// returned handle. Filestore verifies the opened object is regular before the
// handle can be used, so consumers do not reopen the namespace with os.OpenFile.
func OpenUpdate(ctx context.Context, request UpdateHandleRequest) (*os.File, error) {
	if err := contextstate.Validate(ctx); err != nil {
		return nil, err
	}
	if err := request.Validate(); err != nil {
		return nil, err
	}
	file, err := openMutableFile(rootedOpenRequest{
		root: request.Location.Root,
		path: request.Location.Path.String(),
		flag: os.O_RDWR,
	})
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

// OpenStagedRead reopens the exact inode named by one StagedFile receipt. It
// returns the real Go file only after rechecking the receipt before and after
// open, closing the namespace race without introducing a reader wrapper.
func OpenStagedRead(ctx context.Context, staged StagedFile) (*os.File, error) {
	if err := contextstate.Validate(ctx); err != nil {
		return nil, err
	}
	if err := staged.Validate(); err != nil {
		return nil, err
	}
	if err := validateCurrentStage(staged); err != nil {
		return nil, err
	}
	file, observed, err := openRegularReadFile(staged.root, staged.path.String())
	if err != nil {
		return nil, err
	}
	if err := validateStagedObservation(staged, observed); err != nil {
		return nil, closeReadFile(file, err)
	}
	return file, nil
}

// OpenParent opens the parent of one absolute path as a rooted capability and
// names the entry inside it.
//
// Every product that holds an absolute path and wants a filestore operation
// performs this same split: take the parent, open it as a root, re-parse the
// base as a relative path. Written by hand it is a dozen lines of string
// surgery per call site, and each copy decides for itself whether to clean the
// path first and what to do when the base is the filesystem root.
//
// The caller owns the returned Location's Root and must close it. That is the
// same ownership OpenAppend and OpenRead already hand out, so the rule does not
// change: whoever received the handle closes it.
func OpenParent(ctx context.Context, path core.AbsolutePath) (Location, error) {
	if err := contextstate.Validate(ctx); err != nil {
		return Location{}, err
	}
	if err := path.Validate(); err != nil {
		return Location{}, contractError(err)
	}
	parent, err := path.Parent()
	if err != nil {
		return Location{}, contractError(err)
	}
	base, err := path.Base()
	if err != nil {
		return Location{}, contractError(err)
	}
	target, err := core.ParseRelativePath(base.String())
	if err != nil {
		return Location{}, contractError(err)
	}
	root, err := openRootDirectory(parent.String())
	if err != nil {
		return Location{}, sourceError(err)
	}
	return Location{Root: root, Path: target}, nil
}

// OpenRoot opens one absolute directory as a caller-owned Go rooted capability.
// Acquisition enforces a directory at the OS boundary and follows a final
// directory symlink. Subsequent operations keep os.Root confinement semantics.
// The caller closes the returned root.
func OpenRoot(ctx context.Context, path core.AbsolutePath) (*os.Root, error) {
	if err := contextstate.Validate(ctx); err != nil {
		return nil, err
	}
	if err := path.Validate(); err != nil {
		return nil, contractError(err)
	}
	root, err := openRootDirectory(path.String())
	if err != nil {
		return nil, sourceError(err)
	}
	return root, nil
}

// ValidateRootIdentity proves root and directory still name the same opened
// filesystem object. Root.Name is diagnostic only: descriptor-backed roots
// intentionally carry /proc/self/fd or /dev/fd names.
func ValidateRootIdentity(root *os.Root, directory core.AbsolutePath) error {
	if root == nil {
		return contractError(errors.New("filestore root identity is absent"))
	}
	if err := directory.Validate(); err != nil {
		return contractError(err)
	}
	rootInfo, err := root.Stat(".")
	if err != nil {
		return sourceError(err)
	}
	directoryInfo, err := os.Stat(directory.String())
	if err != nil {
		return sourceError(err)
	}
	if !os.SameFile(rootInfo, directoryInfo) {
		return contractError(errors.New("filestore root identity differs from directory"))
	}
	return nil
}
