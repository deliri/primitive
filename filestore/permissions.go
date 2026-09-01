package filestore

import (
	"context"

	"github.com/deliri/primitive/v2026/contextstate"
)

// SetPermissions applies one exact mode through the rooted capability. It
// follows neither an untrusted absolute path nor an ambient working directory.
func SetPermissions(ctx context.Context, request PermissionRequest) error {
	if err := contextstate.Validate(ctx); err != nil {
		return err
	}
	if err := request.Validate(); err != nil {
		return err
	}
	if err := request.Location.Root.Chmod(request.Location.Path.String(), request.Mode); err != nil {
		return activationError(err)
	}
	if err := syncParent(request.Location.Root, request.Location.Path); err != nil {
		return activationError(err)
	}
	return nil
}

var _ interface{ Validate() error } = PermissionRequest{}
