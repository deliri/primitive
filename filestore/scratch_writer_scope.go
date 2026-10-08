package filestore

import (
	"context"
	"io"
	"os"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
)

// ScratchWriterScopeRequest lends an exclusively created scratch writer.
// Caller policy owns its name and subsequent removal or read. This scope
// proves closure only; it does not claim synchronization or publication.
type ScratchWriterScopeRequest struct {
	Use     func(context.Context, io.Writer) error
	Scratch ScratchRequest
}

func (r ScratchWriterScopeRequest) Validate() error {
	if err := r.Scratch.Validate(); err != nil {
		return err
	}
	if r.Use == nil {
		return core.ErrFilestoreContract
	}
	return nil
}

func WithScratchWriterScope(ctx context.Context, request ScratchWriterScopeRequest) (FileScopeResult, error) {
	if err := contextstate.Validate(ctx); err != nil {
		return FileScopeResult{}, err
	}
	if err := request.Validate(); err != nil {
		return FileScopeResult{}, err
	}
	file, err := OpenScratch(ctx, request.Scratch)
	if err != nil {
		return FileScopeResult{}, err
	}
	result := finishFileScope(ctx, file, func(ctx context.Context, file *os.File) error { return request.Use(ctx, file) })
	return result, result.Validate()
}

var _ core.Validatable = ScratchWriterScopeRequest{}
