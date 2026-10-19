package filestore

import (
	"context"
	"os"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/lineio"
)

// ReadAtScopeRequest lends sequential and positional reads over the same
// native file. Use is synchronous and must not close or retain the source.
type ReadAtScopeRequest struct {
	Use      func(context.Context, lineio.RecordSource) error
	Location Location
}

func (r ReadAtScopeRequest) Validate() error {
	if err := r.Location.Validate(); err != nil {
		return err
	}
	if r.Use == nil {
		return core.ErrFilestoreContract
	}
	return nil
}

// WithReadAtScope owns the native read handle through callback completion and
// closure. Positional reads refer to the borrowed inode and do not reopen its
// namespace name or disturb Go's sequential cursor.
func WithReadAtScope(ctx context.Context, request ReadAtScopeRequest) (FileScopeResult, error) {
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
	result := finishFileScope(ctx, file, fileScopeCallback{Use: func(ctx context.Context, file *os.File) error { return request.Use(ctx, file) }})
	return result, result.Validate()
}

var _ core.Validatable = ReadAtScopeRequest{}
