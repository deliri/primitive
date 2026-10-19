package filestore

import (
	"context"
	"errors"
	"os"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
)

// WithInspectedReadScope lends a read descriptor only after Go proves that it
// names the append owner's held file. A different file, even with identical
// bytes, cannot supply the owner's observation. Final symlinks are refused;
// hard links to the original object are admitted. No read handle escapes.
//
// Identity admission precedes Flush, so a refused path cannot cause pending
// writes. Successful admission flushes the original Go working window before
// metadata and bytes are lent. This is visibility, not a synchronization claim.
// The caller serializes owner use; replacing/closing the owner during Use is a
// refused scope. All native close, callback and cancellation causes survive.
func (d *AppendDestination) WithInspectedReadScope(ctx context.Context, request InspectedReadScopeRequest) (result FileScopeResult, resultErr error) {
	defer func() {
		resultErr = scopeCancellationError(ctx, resultErr)
		result.operationError = scopeCancellationError(ctx, result.operationError)
	}()
	if err := errors.Join(contextstate.Validate(ctx), request.Validate(), d.Validate()); err != nil {
		return FileScopeResult{}, err
	}
	held := d.file
	source, err := OpenCustodyRead(ctx, ReadHandleRequest{Location: request.Location})
	if err != nil {
		return FileScopeResult{}, err
	}
	result = finishFileScope(ctx, source, func(ctx context.Context, source *os.File) error {
		acquired, err := source.Stat()
		if err != nil {
			return sourceError(err)
		}
		original, err := held.Stat()
		if err != nil {
			return sourceError(err)
		}
		if !os.SameFile(original, acquired) {
			return core.ErrFilestoreContract
		}
		if err := d.Flush(); err != nil {
			return destinationError(err)
		}
		metadata, err := InspectOpenFile(ctx, HandleInspectionRequest{File: source})
		if err != nil {
			return err
		}
		useErr := request.Use(ctx, InspectedRead{Source: source, Metadata: metadata})
		if d.file != held {
			useErr = errors.Join(useErr, core.ErrFilestoreContract)
		}
		return errors.Join(useErr, contextstate.Validate(ctx))
	})
	return result, result.Validate()
}
