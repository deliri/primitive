package filestore

import (
	"context"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
)

// StageWriteRequest borrows one chunk for a synchronous native write.
// The caller retains the chunk; execution never buffers or retains it.
type StageWriteRequest struct {
	Destination *StageDestination
	Data        []byte
}

func (r StageWriteRequest) Validate() error { return r.Destination.Validate() }

// StageWriteObservation reports native acceptance, including partial writes.
// It does not prove synchronization or publication; finish owns that receipt.
// Its zero value is invalid because no native write was observed.
type StageWriteObservation struct {
	bytesWritten core.ByteLength
	observed     bool
}

// BytesWritten returns the exact native progress, including a failed partial write.
func (o StageWriteObservation) BytesWritten() core.ByteLength { return o.bytesWritten }

func (o StageWriteObservation) Validate() error {
	if !o.observed {
		return core.ErrFilestoreContract
	}
	return o.bytesWritten.Validate()
}

// WriteStage validates context and linear custody before calling Go's file
// writer directly. Even failed native writes return their observed extent.
func WriteStage(ctx context.Context, request StageWriteRequest) (StageWriteObservation, error) {
	if err := contextstate.Validate(ctx); err != nil {
		return StageWriteObservation{}, err
	}
	if err := request.Validate(); err != nil {
		return StageWriteObservation{}, err
	}
	written, writeErr := request.Destination.file.Write(request.Data)
	length, err := core.NewByteLength(uint64(written))
	if err != nil {
		return StageWriteObservation{}, sizeError(err)
	}
	observation := StageWriteObservation{bytesWritten: length, observed: true}
	if writeErr != nil {
		return observation, destinationError(writeErr)
	}
	return observation, nil
}

var (
	_ core.Validatable = StageWriteRequest{}
	_ core.Validatable = StageWriteObservation{}
)
