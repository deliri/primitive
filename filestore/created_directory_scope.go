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
	destination, err := OpenCreatedDirectoryDestination(ctx, request.Directory)
	if err != nil {
		return err
	}
	defer func() {
		if destination.Validate() == nil {
			resultErr = errors.Join(resultErr, AbandonCreatedDirectoryDestination(destination))
		}
	}()
	root, err := destination.Root()
	if err != nil {
		return err
	}
	if err := request.Use(ctx, root); err != nil {
		return scopeCancellationError(ctx, err)
	}
	return FinishCreatedDirectoryDestination(ctx, destination)
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
