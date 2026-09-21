package filestore

import (
	"context"

	"github.com/deliri/primitive/v2026/contextstate"
)

// CreateDirectory exclusively creates the final directory under an existing
// parent. An occupied name, including a directory or symbolic link, is refused
// without changing it. This differs from EnsureDirectory's idempotent chain
// preparation. Successful creation synchronizes the directory and its parent.
// A later native failure may leave the newly created directory; it is not
// removed or reported as successful. Callers coordinate namespace changes.
func CreateDirectory(ctx context.Context, request DirectoryRequest) error {
	if err := contextstate.Validate(ctx); err != nil {
		return err
	}
	if err := request.Validate(); err != nil {
		return err
	}
	if err := request.Location.Root.Mkdir(request.Location.Path.String(), request.Mode); err != nil {
		return classifyCreateError(err)
	}
	if err := synchronizeDirectoryMode(request.Location.Root, request.Location.Path, request.Mode); err != nil {
		return activationError(err)
	}
	if err := syncParent(request.Location.Root, request.Location.Path); err != nil {
		return activationError(err)
	}
	return nil
}
