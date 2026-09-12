package filestore

import (
	"context"
	"errors"
	"io"
	"os"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
)

// ContentIndexInspectionRequest binds a held canonical index file to the
// caller's expected digest and encoded extent. Content describes the complete
// index bytes, not the aggregate extents of the records contained within it.
// The caller owns the file and must retain exclusive custody of its contents.
type ContentIndexInspectionRequest struct {
	File    *os.File
	Content ContentIndexEntry
}

func (r ContentIndexInspectionRequest) Validate() error {
	if r.File == nil {
		return core.ErrFilestoreContract
	}
	return r.Content.Validate()
}

// InspectContentIndex validates a complete sorted, unique record stream and
// binds its exact bytes to the caller's expected content identity. It emits no
// records or effects and leaves the borrowed file open at EOF. Callers may
// rewind that same file for subsequent streaming consumption; inspection does
// not make a mutable file immutable or transfer its ownership.
func InspectContentIndex(ctx context.Context, request ContentIndexInspectionRequest) (ContentIndexSummary, error) {
	if err := contextstate.Validate(ctx); err != nil {
		return ContentIndexSummary{}, err
	}
	if err := request.Validate(); err != nil {
		return ContentIndexSummary{}, err
	}
	info, err := request.File.Stat()
	if err != nil {
		return ContentIndexSummary{}, errors.Join(core.ErrFilestoreContract, err)
	}
	if !info.Mode().IsRegular() || info.Size() < 0 || uint64(info.Size()) != request.Content.Extent.Uint64() {
		return ContentIndexSummary{}, core.ErrFilestoreContract
	}
	summary, digest, err := inspectContentIndex(ctx, request.File, nil, true)
	if err != nil {
		return ContentIndexSummary{}, errors.Join(core.ErrFilestoreContract, err)
	}
	// The complete EOF pass leaves the underlying descriptor after every encoded
	// byte, even when its reader prefetched records into its bounded buffer.
	extent, err := request.File.Seek(0, io.SeekCurrent)
	if err != nil {
		return ContentIndexSummary{}, errors.Join(core.ErrFilestoreContract, err)
	}
	if extent < 0 || uint64(extent) != request.Content.Extent.Uint64() || digest != request.Content.Digest {
		return ContentIndexSummary{}, core.ErrFilestoreContract
	}
	if err := contextstate.Validate(ctx); err != nil {
		return ContentIndexSummary{}, err
	}
	return summary, nil
}
