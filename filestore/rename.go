package filestore

import (
	"context"

	"github.com/deliri/primitive/v2026/contextstate"
)

// Rename moves one existing rooted entry to another name under the same
// capability and makes both namespaces durable.
//
// Commit activates a StagedFile, which covers writing new bytes. It cannot
// move something already on disk, so a product that must relocate an existing
// entry — quarantining a directory that breached a limit, promoting a prepared
// tree into service — falls back to os.Rename plus a hand-rolled directory
// sync, outside the rooted boundary and outside this package's durability
// accounting.
//
// An existing target is replaced, which is the rename the operating system
// offers for an arbitrary entry. Create-only activation is Commit's contract
// and reaches it by linking, which cannot express a directory; a caller that
// needs the target to be absent must stage and commit rather than rename.
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
