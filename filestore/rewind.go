package filestore

import (
	"context"
	"errors"
	"io"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
)

// RewindRequest borrows a Go stream's seek capability. Its zero value is
// invalid. The caller owns the stream lifetime and meaning of replay.
type RewindRequest struct{ Source io.Seeker }

func (r RewindRequest) Validate() error {
	if r.Source == nil {
		return core.ErrFilestoreContract
	}
	return nil
}

// Rewind executes Go's seek-to-beginning operation and verifies its returned
// coordinate. It neither truncates bytes nor keeps a mirrored cursor. A
// canceled result may follow the effect; callers must preserve that uncertainty.
func Rewind(ctx context.Context, request RewindRequest) error {
	if err := contextstate.Validate(ctx); err != nil {
		return err
	}
	if err := request.Validate(); err != nil {
		return err
	}
	position, err := request.Source.Seek(0, io.SeekStart)
	if err != nil {
		return errors.Join(activationError(err), contextstate.Validate(ctx))
	}
	if position != 0 {
		return errors.Join(core.ErrFilestoreContract, contextstate.Validate(ctx))
	}
	return contextstate.Validate(ctx)
}
