package filestore

import (
	"context"
	"errors"
	"os"
	"path/filepath"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
)

const scratchScopePrefix = "primitive-scratch-"

// ScratchScopeRequest lends a private native root synchronously. The caller
// owns filenames, schemas, and closure of child handles it acquires. Primitive
// creates and removes the actual namespace, joins cleanup failures, and closes
// the borrowed root when Use returns. Use must not retain the root.
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
	directory, err := os.MkdirTemp(request.Parent.String(), scratchScopePrefix)
	if err != nil {
		return activationError(err)
	}
	defer func() {
		resultErr = errors.Join(resultErr, removeScratchScope(context.WithoutCancel(ctx), parent, directory))
	}()
	root, err := os.OpenRoot(directory)
	if err != nil {
		return activationError(err)
	}
	defer func() { resultErr = errors.Join(resultErr, root.Close()) }()
	return request.Use(ctx, root)
}

func removeScratchScope(ctx context.Context, parent *os.Root, directory string) error {
	path, err := core.ParseRelativePath(filepath.Base(directory))
	if err != nil {
		return err
	}
	return RemoveTree(ctx, TreeRemovalRequest{Location: Location{Root: parent, Path: path}})
}
