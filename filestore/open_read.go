package filestore

import (
	"context"
	"errors"
	"io/fs"
	"os"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
)

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
// The caller owns the returned handle and must close it. A path that is not a
// regular file is refused before any bytes are read, so a symbolic link,
// directory, or device planted at the name cannot answer for the file the
// caller asked for.
func OpenRead(ctx context.Context, request ReadHandleRequest) (*os.File, error) {
	if err := contextstate.Validate(ctx); err != nil {
		return nil, err
	}
	if err := request.Validate(); err != nil {
		return nil, err
	}
	return openRegularReadFile(
		request.Location.Root,
		request.Location.Path.String(),
	)
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
	file, err := openRegularReadFile(staged.root, staged.path.String())
	if err != nil {
		return nil, err
	}
	observed, err := file.Stat()
	if err != nil {
		return nil, closeReadFile(file, sourceError(err))
	}
	if err := validateOpenedStage(staged, observed); err != nil {
		return nil, closeReadFile(file, err)
	}
	return file, nil
}

func validateOpenedStage(staged StagedFile, observed fs.FileInfo) error {
	if observed == nil || !os.SameFile(staged.info, observed) {
		return indeterminateActivationError(errors.New("filestore opened stage identity changed"))
	}
	if !observed.Mode().IsRegular() || observed.Mode().Perm() != staged.info.Mode().Perm() {
		return activationError(errors.New("filestore opened stage permissions or type changed"))
	}
	observedBytes, err := core.CheckedUint64FromInt64(observed.Size())
	if err != nil || observedBytes != staged.bytes.Uint64() {
		return sizeError(errors.Join(errors.New("filestore opened stage extent changed"), err))
	}
	return nil
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
	root, err := os.OpenRoot(parent.String())
	if err != nil {
		return Location{}, sourceError(err)
	}
	return Location{Root: root, Path: target}, nil
}

// OpenRoot opens one absolute directory as a rooted capability.
//
// Location.Root is an *os.Root, so every filestore operation needs one, but
// until now filestore would only ever hand a root back attached to a parent
// split (OpenParent) or an opened file. A product that keeps a long-lived store
// directory and performs many operations under it had no contract to ask for
// that directory as a root, so it reached past filestore into os.OpenRoot and
// paid for it twice: the typed AbsolutePath went back out as a string, and the
// failure arrived as a bare OS error at each call site instead of one filestore
// identity.
//
// The directory check is left to the OS. Inspecting first and opening second
// would decide against a path that another process can replace in between, and
// os.OpenRoot already refuses a non-directory.
//
// The caller owns the returned root and must close it, the same ownership rule
// OpenParent, OpenAppend, and OpenRead already hand out.
func OpenRoot(ctx context.Context, path core.AbsolutePath) (*os.Root, error) {
	if err := contextstate.Validate(ctx); err != nil {
		return nil, err
	}
	if err := path.Validate(); err != nil {
		return nil, contractError(err)
	}
	root, err := os.OpenRoot(path.String())
	if err != nil {
		return nil, sourceError(err)
	}
	return root, nil
}
