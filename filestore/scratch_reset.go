package filestore

import (
	"context"
	"io"
	"os"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
)

// ScratchResetRequest grants disposal of all bytes in a caller-owned scratch
// handle. It does not promise durable publication or recovery.
type ScratchResetRequest struct{ File *os.File }

func (r ScratchResetRequest) Validate() error {
	if r.File == nil {
		return core.ErrFilestoreContract
	}
	return nil
}

// ResetScratch truncates a regular scratch file and restores its write offset
// to zero. Go and the OS own the file; no mirrored extent or lifecycle is kept.
// Failure may leave changed bytes. The caller must discard failed scratch work.
func ResetScratch(ctx context.Context, request ScratchResetRequest) error {
	if err := contextstate.Validate(ctx); err != nil {
		return err
	}
	if err := request.Validate(); err != nil {
		return err
	}
	info, err := request.File.Stat()
	if err != nil {
		return activationError(err)
	}
	if !info.Mode().IsRegular() {
		return core.ErrFilestoreContract
	}
	if err := request.File.Truncate(0); err != nil {
		return activationError(err)
	}
	offset, err := request.File.Seek(0, io.SeekStart)
	if err != nil {
		return activationError(err)
	}
	if offset != 0 {
		return core.ErrFilestoreContract
	}
	return contextstate.Validate(ctx)
}
