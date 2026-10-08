package filestore

import (
	"context"
	"errors"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
)

// StageSyncRequest synchronizes the current native body without finishing it.
type StageSyncRequest struct{ Destination *StageDestination }

func (r StageSyncRequest) Validate() error { return r.Destination.Validate() }

// StageSyncObservation identifies the native extent after successful body sync.
// The zero value is invalid. Directory synchronization and publication belong
// to finish and commit, and a declared final extent is checked only at finish.
type StageSyncObservation struct {
	bytesWritten core.ByteLength
	synchronized bool
}

func (o StageSyncObservation) BytesWritten() core.ByteLength { return o.bytesWritten }

func (o StageSyncObservation) Validate() error {
	if !o.synchronized {
		return contractError(errors.New("filestore stage synchronization was not observed"))
	}
	return o.bytesWritten.Validate()
}

// SyncStage delegates to Go's native synchronization and extent observation.
// It leaves linear custody available for subsequent writes or settlement.
func SyncStage(ctx context.Context, request StageSyncRequest) (StageSyncObservation, error) {
	if err := contextstate.Validate(ctx); err != nil {
		return StageSyncObservation{}, err
	}
	if err := request.Validate(); err != nil {
		return StageSyncObservation{}, err
	}
	return syncStage(request.Destination)
}

func syncStage(destination *StageDestination) (StageSyncObservation, error) {
	if err := destination.file.Sync(); err != nil {
		return StageSyncObservation{}, activationError(err)
	}
	info, err := destination.file.Stat()
	if err != nil {
		return StageSyncObservation{}, activationError(err)
	}
	extent, err := core.CheckedUint64FromInt64(info.Size())
	if err != nil {
		return StageSyncObservation{}, sizeError(err)
	}
	length, err := core.NewByteLength(extent)
	if err != nil {
		return StageSyncObservation{}, sizeError(err)
	}
	return StageSyncObservation{bytesWritten: length, synchronized: true}, nil
}

var (
	_ core.Validatable = StageSyncRequest{}
	_ core.Validatable = StageSyncObservation{}
)
