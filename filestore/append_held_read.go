package filestore

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"os"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
)

// HeldAppendReadRequest borrows the actual append descriptor synchronously.
// No pathname or caller-owned close capability participates in this read.
type HeldAppendReadRequest struct {
	Use func(context.Context, InspectedRead) error
}

func (r HeldAppendReadRequest) Validate() error {
	if r.Use == nil {
		return core.ErrFilestoreContract
	}
	return nil
}

// WithHeldReadScope flushes and lends the original held object even after its
// name is removed or replaced. Go's descriptor access controls actual reads;
// write-only owners preserve their native read refusal. Reads have an isolated
// section cursor and a Go cancellation lifetime that expires on every return.
// This borrows append custody; it neither closes the owner nor claims a close.
func (d *AppendDestination) WithHeldReadScope(ctx context.Context, request HeldAppendReadRequest) (resultErr error) {
	defer func() { resultErr = scopeCancellationError(ctx, resultErr) }()
	if err := errors.Join(contextstate.Validate(ctx), request.Validate(), d.Validate()); err != nil {
		return err
	}
	held := d.file
	if err := d.Flush(); err != nil {
		return destinationError(err)
	}
	metadata, err := InspectOpenFile(ctx, HandleInspectionRequest{File: held})
	if err != nil {
		return err
	}
	extent, err := metadata.SizeBytes()
	if err != nil {
		return err
	}
	length, err := extent.Int64()
	if err != nil {
		return err
	}
	lifetime, cancel := context.WithCancelCause(ctx)
	defer cancel(fs.ErrClosed)
	reader := heldAppendReadSource{lifetime: lifetime, source: io.NewSectionReader(held, 0, length)}
	err = useFileScope(ctx, held, fileScopeCallback{Use: func(ctx context.Context, _ *os.File) error {
		return request.Use(ctx, InspectedRead{Source: reader, Metadata: metadata})
	}})
	if d.file != held {
		err = errors.Join(err, core.ErrFilestoreContract)
	}
	return errors.Join(err, contextstate.Validate(ctx))
}

type heldAppendReadSource struct {
	lifetime context.Context
	source   *io.SectionReader
}

func (r heldAppendReadSource) Read(data []byte) (int, error) {
	if err := contextstate.Validate(r.lifetime); err != nil {
		return 0, errors.Join(err, context.Cause(r.lifetime))
	}
	return r.source.Read(data)
}
func (r heldAppendReadSource) ReadAt(data []byte, offset int64) (int, error) {
	if err := contextstate.Validate(r.lifetime); err != nil {
		return 0, errors.Join(err, context.Cause(r.lifetime))
	}
	return r.source.ReadAt(data, offset)
}
func (r heldAppendReadSource) Seek(offset int64, whence int) (int64, error) {
	if err := contextstate.Validate(r.lifetime); err != nil {
		return 0, errors.Join(err, context.Cause(r.lifetime))
	}
	return r.source.Seek(offset, whence)
}
