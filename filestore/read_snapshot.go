package filestore

import (
	"context"
	"errors"
	"io"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
)

// ReadSnapshotRequest borrows the exact open object. No path is reopened and
// its sequential cursor is neither inspected nor changed.
type ReadSnapshotRequest struct{ Source ScratchReplayFile }

func (r ReadSnapshotRequest) Validate() error {
	if core.ReaderIsNil(r.Source) {
		return core.ErrFilestoreContract
	}
	return nil
}

// ReadSnapshot lends an independent Go SectionReader bounded by the native
// extent observed at creation. Later writes cannot extend that view. The
// caller retains the enclosing native file lifetime; this owns no Close.
// An empty observation has no reader and never fabricates content.
type ReadSnapshot struct {
	Reader io.Reader
	Extent core.ByteLength
}

func (r ReadSnapshot) Validate() error {
	if err := r.Extent.Validate(); err != nil {
		return err
	}
	if (r.Extent.Uint64() == 0) != core.ReaderIsNil(r.Reader) {
		return core.ErrFilestoreContract
	}
	return nil
}

func ObserveReadSnapshot(ctx context.Context, request ReadSnapshotRequest) (result ReadSnapshot, resultErr error) {
	defer func() {
		if ctx != nil && (errors.Is(resultErr, context.Canceled) || errors.Is(resultErr, context.DeadlineExceeded)) {
			resultErr = errors.Join(resultErr, context.Cause(ctx))
		}
	}()
	if err := errors.Join(contextstate.Validate(ctx), request.Validate()); err != nil {
		return ReadSnapshot{}, err
	}
	observed, err := InspectOpenFile(ctx, HandleInspectionRequest{File: request.Source})
	if err != nil {
		return ReadSnapshot{}, err
	}
	extent, err := observed.SizeBytes()
	if err != nil {
		return ReadSnapshot{}, err
	}
	size, err := extent.Int64()
	if err != nil {
		return ReadSnapshot{}, err
	}
	if size == 0 {
		return ReadSnapshot{}, nil
	}
	result = ReadSnapshot{Reader: io.NewSectionReader(request.Source, 0, size), Extent: extent}
	if err := errors.Join(result.Validate(), contextstate.Validate(ctx)); err != nil {
		return ReadSnapshot{}, err
	}
	return result, nil
}

var _ core.Validatable = ReadSnapshotRequest{}
var _ core.Validatable = ReadSnapshot{}
