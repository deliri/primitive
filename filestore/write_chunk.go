package filestore

import (
	"context"
	"errors"
	"io"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
)

// ChunkWriteRequest borrows one destination for one standard-library Write.
// It adds no buffering, retry, synchronization, close or publication claim.
type ChunkWriteRequest struct {
	Destination io.Writer
	Bytes       []byte
}

func (r ChunkWriteRequest) Validate() error {
	if core.WriterIsNil(r.Destination) {
		return core.ErrFilestoreContract
	}
	return nil
}

// WriteChunk preserves the count actually accepted by Go, including a partial
// count beside failure. The returned extent is acceptance, never content proof
// or durability. Invalid native counts withdraw the count and preserve cause.
func WriteChunk(ctx context.Context, request ChunkWriteRequest) (accepted core.ByteLength, resultErr error) {
	defer func() {
		if ctx != nil && (errors.Is(resultErr, context.Canceled) || errors.Is(resultErr, context.DeadlineExceeded)) {
			resultErr = errors.Join(resultErr, context.Cause(ctx))
		}
	}()
	if err := errors.Join(contextstate.Validate(ctx), request.Validate()); err != nil {
		return core.ByteLength{}, err
	}
	count, writeErr := (streamWriter{destination: request.Destination, kind: streamDestinationCaller}).Write(request.Bytes)
	extent, countErr := core.NewByteLength(uint64(count))
	return extent, errors.Join(writeErr, countErr, contextstate.Validate(ctx))
}

var _ core.Validatable = ChunkWriteRequest{}
