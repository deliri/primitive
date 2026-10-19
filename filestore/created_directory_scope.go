package filestore

import (
	"context"
	"errors"
	"io/fs"
	"os"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
)

// CreatedDirectoryScopeRequest exclusively creates one final directory under
// an existing parent. Use borrows its native root and closes any child handles
// it acquires. The root must not escape Use. Success retains the directory;
// consumer failure, cancellation or panic removes the owned native tree.
type CreatedDirectoryScopeRequest struct {
	Directory DirectoryRequest
	Use       func(context.Context, *os.Root) error
}

func (r CreatedDirectoryScopeRequest) Validate() error {
	if r.Use == nil {
		return errors.Join(core.ErrFilestoreContract, r.Directory.Validate())
	}
	return r.Directory.Validate()
}

// WithCreatedDirectoryScope owns a native namespace lifetime without a file
// inventory. Occupied names are refused. The acquired directory and current
// parent entry must agree before Use and before retention. Callers coordinate
// namespace changes during acquisition, as with CreateDirectory. A native
// refusal before ownership can be observed remains explicitly indeterminate.
func WithCreatedDirectoryScope(ctx context.Context, request CreatedDirectoryScopeRequest) (resultErr error) {
	if err := errors.Join(contextstate.Validate(ctx), request.Validate()); err != nil {
		return err
	}
	parent := request.Directory.Location.Root
	path := request.Directory.Location.Path
	if err := parent.Mkdir(path.String(), request.Directory.Mode); err != nil {
		return classifyCreateError(err)
	}
	owned, err := parent.Lstat(path.String())
	if err != nil {
		return indeterminateActivationError(err)
	}
	retained := false
	defer func() {
		if !retained || resultErr != nil {
			cleanup := context.WithoutCancel(ctx)
			removeErr := removeScratchScope(cleanup, parent, path.String(), owned)
			if removeErr == nil {
				removeErr = syncParent(parent, path)
			}
			if removeErr != nil {
				resultErr = errors.Join(resultErr, cleanupError(removeErr))
			}
		}
	}()
	root, err := parent.OpenRoot(path.String())
	if err != nil {
		return activationError(err)
	}
	defer func() {
		resultErr = scopeCancellationError(ctx, errors.Join(resultErr, root.Close()))
	}()
	resultErr = useCreatedDirectory(ctx, request, root, owned)
	retained = resultErr == nil
	return resultErr
}

func useCreatedDirectory(ctx context.Context, request CreatedDirectoryScopeRequest, root *os.Root, owned fs.FileInfo) error {
	if err := contextstate.Validate(ctx); err != nil {
		return scopeCancellationError(ctx, err)
	}
	parent := request.Directory.Location.Root
	path := request.Directory.Location.Path
	if err := createdDirectoryMatches(parent, path, root, owned); err != nil {
		return err
	}
	dot, err := core.ParseRelativePath(".")
	if err != nil {
		return err
	}
	if err := synchronizeDirectoryMode(root, dot, request.Directory.Mode); err != nil {
		return activationError(err)
	}
	if err := contextstate.Validate(ctx); err != nil {
		return scopeCancellationError(ctx, err)
	}
	if err := request.Use(ctx, root); err != nil {
		return scopeCancellationError(ctx, err)
	}
	if err := contextstate.Validate(ctx); err != nil {
		return scopeCancellationError(ctx, err)
	}
	if err := createdDirectoryMatches(parent, path, root, owned); err != nil {
		return err
	}
	if err := syncDirectory(root, dot); err != nil {
		return activationError(err)
	}
	if err := syncParent(parent, path); err != nil {
		return activationError(err)
	}
	return nil
}

func createdDirectoryMatches(parent *os.Root, path core.RelativePath, root *os.Root, owned fs.FileInfo) error {
	current, err := parent.Lstat(path.String())
	if err != nil {
		return sourceError(err)
	}
	held, err := root.Stat(".")
	if err != nil {
		return sourceError(err)
	}
	if !owned.IsDir() || !os.SameFile(owned, current) || !os.SameFile(owned, held) {
		return core.ErrFilestoreConflict
	}
	return nil
}

var _ core.Validatable = CreatedDirectoryScopeRequest{}
