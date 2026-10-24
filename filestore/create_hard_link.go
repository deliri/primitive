package filestore

import (
	"context"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
)

// HardLinkRequest creates Target as another name for Location's inode under
// the same root capability. It does not copy file bytes or follow a symbolic
// link at Location. Both parents must already exist.
type HardLinkRequest struct {
	Location Location
	Target   core.RelativePath
}

func (r HardLinkRequest) Validate() error {
	if err := r.Location.Validate(); err != nil {
		return err
	}
	if err := validateMutablePath(r.Location.Path); err != nil {
		return err
	}
	return validateMutablePath(r.Target)
}

// CreateHardLink calls Go Root.Link and synchronizes the changed destination
// parent. An occupied destination is refused without replacement. Native
// source, confinement, cross-device and permission causes are preserved. A
// synchronization error after creation is reported with the created entry
// retained; it is not rolled back. This operation acknowledges the namespace,
// not previously unsynchronized source contents. Callers own namespace access.
func CreateHardLink(ctx context.Context, request HardLinkRequest) error {
	if err := contextstate.Validate(ctx); err != nil {
		return err
	}
	if err := request.Validate(); err != nil {
		return err
	}
	if err := request.Location.Root.Link(request.Location.Path.String(), request.Target.String()); err != nil {
		return classifyCreateError(err)
	}
	if err := syncParent(request.Location.Root, request.Target); err != nil {
		return activationError(err)
	}
	return nil
}

var _ core.Validatable = HardLinkRequest{}
