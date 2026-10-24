package filestore

import (
	"context"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
)

// ParseSymbolicLinkTarget admits opaque native link text. It preserves every
// byte, including relative, absolute and dangling targets; it neither resolves
// nor normalizes paths. Empty text and NUL are refused. Native creation limits
// remain the operating system's decision.
func ParseSymbolicLinkTarget(value string) (SymbolicLinkTarget, error) {
	target := SymbolicLinkTarget{value: value}
	if err := target.Validate(); err != nil {
		return SymbolicLinkTarget{}, err
	}
	return target, nil
}

// SymbolicLinkRequest names one exclusively created rooted directory entry.
// Target is opaque text, not a capability over the location it describes.
type SymbolicLinkRequest struct {
	Location Location
	Target   SymbolicLinkTarget
}

// Validate closes the destination and target before any namespace effect.
func (r SymbolicLinkRequest) Validate() error {
	if err := r.Location.Validate(); err != nil {
		return err
	}
	if err := validateMutablePath(r.Location.Path); err != nil {
		return err
	}
	return r.Target.Validate()
}

// CreateSymbolicLink exclusively creates one link through Go's rooted Symlink
// and synchronizes its parent. The target is never followed or inspected and
// may name a location outside the root. An occupied name is preserved. A later
// synchronization failure leaves the new link present and returns an activation
// error; it does not roll back the namespace effect or claim durable success.
// Context is checked before the synchronous native operation, which Go cannot
// interrupt after entry. Namespace coordination remains the caller's policy.
func CreateSymbolicLink(ctx context.Context, request SymbolicLinkRequest) error {
	if err := contextstate.Validate(ctx); err != nil {
		return err
	}
	if err := request.Validate(); err != nil {
		return err
	}
	if err := request.Location.Root.Symlink(request.Target.String(), request.Location.Path.String()); err != nil {
		return classifyCreateError(err)
	}
	if err := syncParent(request.Location.Root, request.Location.Path); err != nil {
		return activationError(err)
	}
	return nil
}

var _ core.Validatable = SymbolicLinkRequest{}
