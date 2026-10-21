package filestore

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"strconv"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/keygen"
)

const scratchScopePrefix = "primitive-scratch-"

// ScratchScopeRequest lends a private native root synchronously. The caller
// owns filenames, schemas, and closure of child handles it acquires. Primitive
// creates and removes the actual namespace, joins cleanup failures, and closes
// the borrowed root when Use returns. Use must not retain the root. Root.Name
// is diagnostic text; child effects must use the borrowed root capability.
type ScratchScopeRequest struct {
	Parent core.AbsolutePath
	Use    func(context.Context, *os.Root) error
}

func (r ScratchScopeRequest) Validate() error {
	if err := r.Parent.Validate(); err != nil {
		return contractError(err)
	}
	if r.Use == nil {
		return core.ErrFilestoreContract
	}
	return nil
}

// WithScratchScope owns a real Go/OS lifetime, without a directory inventory
// or simulated file state. Cleanup remains owned after caller cancellation.
func WithScratchScope(ctx context.Context, request ScratchScopeRequest) (resultErr error) {
	if err := errors.Join(contextstate.Validate(ctx), request.Validate()); err != nil {
		return err
	}
	parent, err := OpenRoot(ctx, request.Parent)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, parent.Close()) }()
	return withScratchParent(ctx, parent, func(ctx context.Context, root *os.Root, _ string) error { return request.Use(ctx, root) })
}

// Once acquired, the parent descriptor owns every child effect. The pathname
// used to acquire it is never resolved again, including during cleanup.
// https://pkg.go.dev/os#Root; keygen owns the native entropy draws.
func withScratchParent(ctx context.Context, parent *os.Root, use func(context.Context, *os.Root, string) error) (resultErr error) {
	directory, err := createScratchDirectory(ctx, parent)
	if err != nil {
		return err
	}
	info, err := parent.Lstat(directory)
	if err != nil {
		return activationError(err)
	}
	defer func() {
		resultErr = errors.Join(resultErr, removeScratchScope(context.WithoutCancel(ctx), parent, directory, info))
	}()
	root, err := parent.OpenRoot(directory)
	if err != nil {
		return activationError(err)
	}
	defer func() { resultErr = errors.Join(resultErr, root.Close()) }()
	return use(ctx, root, directory)
}

func createScratchDirectory(ctx context.Context, parent *os.Root) (string, error) {
	for {
		if err := contextstate.Validate(ctx); err != nil {
			return "", err
		}
		first, err := keygen.RandomUint64()
		if err != nil {
			return "", err
		}
		second, err := keygen.RandomUint64()
		if err != nil {
			return "", err
		}
		name := scratchScopePrefix + strconv.FormatUint(first, 16) + "-" + strconv.FormatUint(second, 16)
		if err := parent.Mkdir(name, 0o700); err != nil {
			if errors.Is(err, fs.ErrExist) {
				continue
			}
			return "", activationError(err)
		}
		return name, nil
	}
}

func removeScratchScope(ctx context.Context, parent *os.Root, directory string, owned fs.FileInfo) error {
	current, err := parent.Lstat(directory)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return cleanupError(err)
	}
	if !os.SameFile(owned, current) {
		return cleanupError(core.ErrFilestoreConflict)
	}
	path, err := core.ParseRelativePath(directory)
	if err != nil {
		return err
	}
	return RemoveTree(ctx, TreeRemovalRequest{Location: Location{Root: parent, Path: path}})
}
