package filestore

import (
	"context"
	"errors"
	"io/fs"
	"os"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
)

// AppendCustody is the current native handle/buffer relation, independent of
// the last rotation attempt. A refused rotation can leave custody held.
type AppendCustody uint8

const (
	AppendCustodyUnknown AppendCustody = iota
	AppendCustodyHeld
	AppendCustodyReleased
)

func (c AppendCustody) Validate() error {
	if !c.IsValid() {
		return core.ErrFilestoreContract
	}
	return nil
}

func (c AppendCustody) IsValid() bool { return c == AppendCustodyHeld || c == AppendCustodyReleased }
func (AppendCustody) OffWireEnum()    {}
func (c AppendCustody) String() string {
	switch c {
	case AppendCustodyHeld:
		return "held"
	case AppendCustodyReleased:
		return "released"
	default:
		return core.UnknownEnumDiagnostic
	}
}

// AppendObservation borrows the original owner's current custody and last
// native rotation refusal. It owns no handle and grants no execution capability.
// Explicit Close ends that rotation attempt, including after native settlement.
type AppendObservation struct {
	RotationError error
	Custody       AppendCustody
}

func (o AppendObservation) Validate() error { return o.Custody.Validate() }

func (d *AppendDestination) ObserveAppend() (AppendObservation, error) {
	if d == nil || d.self != d {
		return AppendObservation{}, core.ErrFilestoreContract
	}
	custody := AppendCustodyReleased
	if d.file != nil || d.writer != nil {
		if d.file == nil || d.writer == nil {
			return AppendObservation{}, core.ErrFilestoreContract
		}
		custody = AppendCustodyHeld
	}
	return AppendObservation{Custody: custody, RotationError: d.rotationError}, nil
}

// AppendPathRotationRequest gives Primitive ownership of parent acquisition,
// native rotation and parent cleanup. No caller-owned root lifetime can hide
// an acquisition or cleanup refusal from the append owner's observation.
type AppendPathRotationRequest struct {
	Path   core.AbsolutePath
	Mode   fs.FileMode
	Append AppendMode
}

func (r AppendPathRotationRequest) Validate() error {
	if err := errors.Join(r.Path.Validate(), r.Append.Validate()); err != nil {
		return err
	}
	return validatePermissionMode(r.Mode)
}

func (d *AppendDestination) RotatePath(ctx context.Context, request AppendPathRotationRequest) (resultErr error) {
	if d == nil || d.self != d {
		return core.ErrFilestoreContract
	}
	defer func() { d.rotationError = resultErr }()
	if err := errors.Join(contextstate.Validate(ctx), request.Validate()); err != nil {
		if ctx != nil {
			err = errors.Join(err, context.Cause(ctx))
		}
		return err
	}
	if err := d.Validate(); err != nil {
		return err
	}
	parent, err := request.Path.Parent()
	if err != nil {
		return err
	}
	relative, err := request.Path.RelativeTo(parent)
	if err != nil {
		return err
	}
	scope, err := WithRootScope(ctx, RootScopeRequest{Directory: parent, Use: func(ctx context.Context, root *os.Root) error {
		return d.Rotate(ctx, AppendRequest{Location: Location{Root: root, Path: relative}, Mode: request.Mode, Append: request.Append})
	}})
	if err != nil {
		return err
	}
	return errors.Join(scope.Validate(), scope.OperationError, scope.CleanupError)
}
