package filestore

import (
	"context"
	"errors"
	"os"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
)

// ScratchDirectoryScopeRequest lends a private root and its verified native
// pathname for child processes. Both expire when Use returns. Callers must
// coordinate namespace changes while native processes use the pathname.
// Root.Name is diagnostic text and is never used as a process directory.
type ScratchDirectoryScopeRequest struct {
	Parent core.AbsolutePath
	Use    func(context.Context, *os.Root, core.AbsolutePath) error
}

func (r ScratchDirectoryScopeRequest) Validate() error {
	if err := r.Parent.Validate(); err != nil {
		return contractError(err)
	}
	if r.Use == nil {
		return core.ErrFilestoreContract
	}
	return nil
}

// WithScratchDirectoryScope reuses the rooted scratch lifetime. Creation,
// identity checking, cleanup after every outcome and native handle closure
// remain in filestore; the consumer does not reconstruct paths from handles.
func WithScratchDirectoryScope(ctx context.Context, request ScratchDirectoryScopeRequest) (resultErr error) {
	if err := errors.Join(contextstate.Validate(ctx), request.Validate()); err != nil {
		return err
	}
	parent, err := OpenRoot(ctx, request.Parent)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, CloseRoot(RootHandleRequest{Root: parent})) }()
	return withScratchParent(ctx, parent, func(ctx context.Context, root *os.Root, name string) error {
		directory, err := request.Parent.Resolve(name)
		if err != nil {
			return err
		}
		if err = ValidateRootIdentity(root, directory); err != nil {
			return err
		}
		return request.Use(ctx, root, directory)
	})
}
