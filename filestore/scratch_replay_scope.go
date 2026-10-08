package filestore

import (
	"context"
	"io"
	"os"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
)

// ScratchReplayFile borrows the native operations needed to replace and
// replay provisional bytes. It has no Close method: Primitive owns closure.
type ScratchReplayFile interface {
	ScratchResetFile
	io.Reader
}

// ScratchReplayScopeRequest owns one exclusively created read/write scratch
// lifetime. Caller policy chooses reset and replay points. No durability,
// buffering, inventory, or mirrored cursor is introduced.
type ScratchReplayScopeRequest struct {
	Use     func(context.Context, ScratchReplayFile) error
	Scratch ScratchRequest
}

func (r ScratchReplayScopeRequest) Validate() error {
	if err := r.Scratch.Validate(); err != nil {
		return err
	}
	if r.Use == nil {
		return core.ErrFilestoreContract
	}
	return nil
}

func WithScratchReplayScope(ctx context.Context, request ScratchReplayScopeRequest) (FileScopeResult, error) {
	if err := contextstate.Validate(ctx); err != nil {
		return FileScopeResult{}, err
	}
	if err := request.Validate(); err != nil {
		return FileScopeResult{}, err
	}
	file, err := openScratch(ctx, request.Scratch, os.O_RDWR)
	if err != nil {
		return FileScopeResult{}, err
	}
	result := finishFileScope(ctx, file, func(ctx context.Context, file *os.File) error { return request.Use(ctx, file) })
	return result, result.Validate()
}

var _ ScratchReplayFile = (*os.File)(nil)
var _ core.Validatable = ScratchReplayScopeRequest{}
