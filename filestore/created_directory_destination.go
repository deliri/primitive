package filestore

import (
	"context"
	"errors"
	"io/fs"
	"os"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
)

// CreatedDirectoryDestination owns one exclusively created native directory.
// It keeps only the real root, its original identity and acquisition request.
// Callers synchronize child file contents and close child handles before
// finish; this owner synchronizes the directory namespace. The handle is linear and
// not safe for concurrent settlement; copying it cannot duplicate custody.
type CreatedDirectoryDestination struct {
	self    *CreatedDirectoryDestination
	root    *os.Root
	created fs.FileInfo
	request DirectoryRequest
}

// OpenCreatedDirectoryDestination exclusively acquires the final directory
// name under an existing parent. As with CreateDirectory, callers coordinate
// namespace changes during acquisition. An unobservable acquisition remains
// explicitly indeterminate. No child inventory or directory model is retained.
func OpenCreatedDirectoryDestination(ctx context.Context, request DirectoryRequest) (*CreatedDirectoryDestination, error) {
	if err := errors.Join(contextstate.Validate(ctx), request.Validate()); err != nil {
		return nil, err
	}
	parent, path := request.Location.Root, request.Location.Path
	if err := parent.Mkdir(path.String(), request.Mode); err != nil {
		return nil, classifyCreateError(err)
	}
	owned, err := parent.Lstat(path.String())
	if err != nil {
		return nil, indeterminateActivationError(err)
	}
	root, err := parent.OpenRoot(path.String())
	if err != nil {
		return nil, cleanupCreatedDirectory(request, owned, activationError(err))
	}
	destination := &CreatedDirectoryDestination{root: root, created: owned, request: request}
	destination.self = destination
	if err := createdDirectoryMatches(parent, path, root, owned); err != nil {
		return nil, destination.fail(err)
	}
	dot, err := core.ParseRelativePath(".")
	if err != nil {
		return nil, destination.fail(err)
	}
	if err := synchronizeDirectoryMode(root, dot, request.Mode); err != nil {
		return nil, destination.fail(activationError(err))
	}
	if err := contextstate.Validate(ctx); err != nil {
		return nil, destination.fail(scopeCancellationError(ctx, err))
	}
	return destination, nil
}

func (d *CreatedDirectoryDestination) Validate() error {
	if d == nil || d.self != d || d.root == nil || d.created == nil {
		return core.ErrFilestoreContract
	}
	return d.request.Validate()
}

// Root lends the exact acquired native directory. The destination owns its
// close; the loan expires when finish or abandon settles the destination.
func (d *CreatedDirectoryDestination) Root() (*os.Root, error) {
	if err := d.Validate(); err != nil {
		return nil, err
	}
	return d.root, nil
}

// Location returns the original parent and name, separately from the held
// child root. It is an acquisition coordinate, not proof of current naming.
func (d *CreatedDirectoryDestination) Location() (Location, error) {
	if err := d.Validate(); err != nil {
		return Location{}, err
	}
	return d.request.Location, nil
}

// FinishCreatedDirectoryDestination verifies the held identity against the
// original name, synchronizes the directory and parent, and closes custody.
// Success retains the tree. Any native refusal removes only the original
// observed tree; a foreign replacement is preserved and reported as conflict.
func FinishCreatedDirectoryDestination(ctx context.Context, destination *CreatedDirectoryDestination) error {
	if err := destination.Validate(); err != nil {
		return err
	}
	if err := prepareCreatedDirectoryRetention(ctx, destination); err != nil {
		return destination.fail(err)
	}
	closeErr := scopeCancellationError(ctx, destination.root.Close())
	request, created := destination.request, destination.created
	destination.release()
	if closeErr != nil {
		return cleanupCreatedDirectory(request, created, closeErr)
	}
	return nil
}

func prepareCreatedDirectoryRetention(ctx context.Context, destination *CreatedDirectoryDestination) error {
	if err := contextstate.Validate(ctx); err != nil {
		return scopeCancellationError(ctx, err)
	}
	request := destination.request
	if err := createdDirectoryMatches(request.Location.Root, request.Location.Path, destination.root, destination.created); err != nil {
		return err
	}
	dot, err := core.ParseRelativePath(".")
	if err != nil {
		return err
	}
	if err := syncDirectory(destination.root, dot); err != nil {
		return activationError(err)
	}
	if err := syncParent(request.Location.Root, request.Location.Path); err != nil {
		return activationError(err)
	}
	return scopeCancellationError(ctx, nil)
}

// AbandonCreatedDirectoryDestination closes custody and removes its original
// native tree. Cleanup is available after operation cancellation; it never
// follows a foreign replacement at the acquired directory's original name.
func AbandonCreatedDirectoryDestination(destination *CreatedDirectoryDestination) error {
	if err := destination.Validate(); err != nil {
		return err
	}
	return destination.fail(nil)
}

func (d *CreatedDirectoryDestination) fail(primary error) error {
	request, created := d.request, d.created
	closeErr := d.root.Close()
	d.release()
	return cleanupCreatedDirectory(request, created, errors.Join(primary, closeErr))
}

func cleanupCreatedDirectory(request DirectoryRequest, owned fs.FileInfo, primary error) error {
	err := removeScratchScope(context.Background(), request.Location.Root, request.Location.Path.String(), owned)
	if err == nil {
		err = syncParent(request.Location.Root, request.Location.Path)
	}
	if err != nil {
		return errors.Join(primary, cleanupError(err))
	}
	return primary
}

func (d *CreatedDirectoryDestination) release() {
	d.self = nil
	d.root = nil
	d.created = nil
}

var _ core.Validatable = (*CreatedDirectoryDestination)(nil)
