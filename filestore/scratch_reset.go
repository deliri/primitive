package filestore

import (
	"context"
	"io"
	"io/fs"

	"github.com/deliri/primitive/v2026/core"
)

// ScratchResetRequest grants disposal of all bytes in a caller-owned scratch
// handle. It does not promise durable publication or recovery.
type ScratchResetFile interface {
	io.Writer
	io.Seeker
	Stat() (fs.FileInfo, error)
	Truncate(int64) error
}

type ScratchResetRequest struct{ File ScratchResetFile }

func (r ScratchResetRequest) Validate() error {
	if core.WriterIsNil(r.File) {
		return core.ErrFilestoreContract
	}
	return nil
}

// ResetScratch truncates a regular scratch file and restores its write offset
// to zero. Go and the OS own the file; no mirrored extent or lifecycle is kept.
// Failure may leave changed bytes. The caller must discard failed scratch work.
func ResetScratch(ctx context.Context, request ScratchResetRequest) error {
	return RestoreScratchPrefix(ctx, ScratchRestoreRequest{File: request.File})
}
