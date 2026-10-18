package filestore

import (
	"context"
	"errors"
	"io"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
)

// ScratchRestoreRequest discards the tail beyond Prefix and restores the write
// position to that prefix. File stays inside its enclosing native scope. Its
// owner must exclude concurrent writes for the synchronous operation.
type ScratchRestoreRequest struct {
	File   ScratchResetFile
	Prefix core.ByteLength
}

func (r ScratchRestoreRequest) Validate() error {
	if core.WriterIsNil(r.File) {
		return core.ErrFilestoreContract
	}
	if err := r.Prefix.Validate(); err != nil {
		return err
	}
	_, err := r.Prefix.Int64()
	return err
}

// RestoreScratchPrefix observes the held regular file, refuses an extension,
// then uses Go's truncate and seek operations. Success observes the resulting
// native extent; it does not promise durability. A failure can leave changed
// bytes, so the caller must abandon uncertain scratch work.
func RestoreScratchPrefix(ctx context.Context, request ScratchRestoreRequest) (resultErr error) {
	defer func() {
		if ctx != nil && (errors.Is(resultErr, context.Canceled) || errors.Is(resultErr, context.DeadlineExceeded)) {
			resultErr = errors.Join(resultErr, context.Cause(ctx))
		}
	}()
	if err := errors.Join(contextstate.Validate(ctx), request.Validate()); err != nil {
		return err
	}
	info, err := request.File.Stat()
	if err != nil {
		return activationError(err)
	}
	if info == nil {
		return core.ErrFilestoreContract
	}
	prefix, err := request.Prefix.Int64()
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Size() < 0 || prefix > info.Size() {
		return core.ErrFilestoreContract
	}
	if err := request.File.Truncate(prefix); err != nil {
		return activationError(err)
	}
	offset, err := request.File.Seek(prefix, io.SeekStart)
	if err != nil {
		return activationError(err)
	}
	if offset != prefix {
		return core.ErrFilestoreContract
	}
	completed, err := request.File.Stat()
	if err != nil {
		return activationError(err)
	}
	if completed == nil {
		return core.ErrFilestoreContract
	}
	if !completed.Mode().IsRegular() || completed.Size() != prefix {
		return core.ErrFilestoreContract
	}
	return contextstate.Validate(ctx)
}

var _ core.Validatable = ScratchRestoreRequest{}
