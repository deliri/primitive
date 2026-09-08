package filestore

import (
	"context"

	"github.com/deliri/primitive/v2026/contextstate"
)

// Rename moves one rooted entry through Go Root.Rename and synchronizes both
// changed parents. Go's replacement rules apply: an existing non-directory
// target may be replaced, but an existing directory target is refused. A
// same-inode hard-link rename may leave both names unchanged, as Go specifies.
// A synchronization failure after rename retains the indeterminate identity;
// the completed namespace effect is not rolled back.
func Rename(ctx context.Context, request RenameRequest) error {
	if err := contextstate.Validate(ctx); err != nil {
		return err
	}
	if err := request.Validate(); err != nil {
		return err
	}
	root := request.Location.Root
	if err := root.Rename(request.Location.Path.String(), request.Target.String()); err != nil {
		return activationError(err)
	}
	if err := syncParent(root, request.Target); err != nil {
		return indeterminateActivationError(err)
	}
	if !differentParentDirectories(request.Location.Path, request.Target) {
		return nil
	}
	if err := syncParent(root, request.Location.Path); err != nil {
		return indeterminateActivationError(err)
	}
	return nil
}
