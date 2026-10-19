package filestore

import (
	"context"
	"errors"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
)

// AppendSyncRequest borrows original append custody for a synchronization.
type AppendSyncRequest struct{ Destination *AppendDestination }

func (r AppendSyncRequest) Validate() error { return r.Destination.Validate() }

// AppendSyncObservation records successful Go buffer flush and file Sync.
// AcceptedBytes is the owner's historical Write acceptance at that boundary,
// including writes since acquisition across rotations. It is not a file extent,
// a namespace observation, or a hardware power-loss promise. Later cleanup or
// writes cannot change this immutable receipt. The zero value is invalid.
type AppendSyncObservation struct {
	accepted     core.ByteLength
	synchronized bool
}

func (o AppendSyncObservation) AcceptedBytes() core.ByteLength { return o.accepted }

func (o AppendSyncObservation) Validate() error {
	if !o.synchronized {
		return core.ErrFilestoreContract
	}
	return o.accepted.Validate()
}

// SyncAppend returns no receipt on admission, flush or synchronization refusal.
// The caller retains the owner and separately settles its lifetime with Close.
func SyncAppend(ctx context.Context, request AppendSyncRequest) (AppendSyncObservation, error) {
	if err := contextstate.Validate(ctx); err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			err = errors.Join(err, context.Cause(ctx))
		}
		return AppendSyncObservation{}, err
	}
	if err := request.Validate(); err != nil {
		return AppendSyncObservation{}, err
	}
	accepted, err := request.Destination.AcceptedBytes()
	if err != nil {
		return AppendSyncObservation{}, err
	}
	if err := request.Destination.Sync(); err != nil {
		return AppendSyncObservation{}, err
	}
	return AppendSyncObservation{accepted: accepted, synchronized: true}, nil
}
