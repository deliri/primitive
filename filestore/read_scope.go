package filestore

import (
	"context"
	"io"
	"os"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
)

// ReadScopeRequest borrows one regular native source synchronously. Use must
// not retain or close the reader. Primitive owns acquisition and closure.
type ReadScopeRequest struct {
	Use      func(context.Context, io.ReadSeeker) error
	Location Location
}

func (r ReadScopeRequest) Validate() error {
	if err := r.Location.Validate(); err != nil {
		return err
	}
	if r.Use == nil {
		return core.ErrFilestoreContract
	}
	return nil
}

// WithReadScope lends only read/seek capability and returns after closure.
// Refused acquisition has no completed result. Callback failure, cancellation
// or panic does not detach the real native file from its close owner.
func WithReadScope(ctx context.Context, request ReadScopeRequest) (FileScopeResult, error) {
	if err := contextstate.Validate(ctx); err != nil {
		return FileScopeResult{}, err
	}
	if err := request.Validate(); err != nil {
		return FileScopeResult{}, err
	}
	file, err := OpenRead(ctx, ReadHandleRequest{Location: request.Location})
	if err != nil {
		return FileScopeResult{}, err
	}
	result := finishFileScope(ctx, file, func(ctx context.Context, file *os.File) error { return request.Use(ctx, file) })
	return result, result.Validate()
}

var _ core.Validatable = ReadScopeRequest{}
