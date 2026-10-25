package filestore

import (
	"context"
	"errors"
	"os"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
)

// ReadAtRequest borrows an already-open native file and a destination window.
// Offset and Data retain Go's positional-read semantics, including negative
// offset refusal and an empty window. The caller owns descriptor lifetime.
type ReadAtRequest struct {
	File   *os.File
	Data   []byte
	Offset int64
}

func (r ReadAtRequest) Validate() error {
	if r.File == nil {
		return core.ErrFilestoreContract
	}
	return nil
}

// ReadAt delegates one positional read to the exact borrowed descriptor. It
// never reopens, closes, seeks, retries or allocates a content buffer. Native
// count, EOF and error identities are preserved. Cancellation is checked before
// and after the read; an in-flight native read has no interruption guarantee.
func ReadAt(ctx context.Context, request ReadAtRequest) (int, error) {
	if err := contextstate.Validate(ctx); err != nil {
		if ctx != nil {
			return 0, errors.Join(err, context.Cause(ctx))
		}
		return 0, err
	}
	if err := request.Validate(); err != nil {
		return 0, err
	}
	count, err := request.File.ReadAt(request.Data, request.Offset)
	if contextErr := contextstate.Validate(ctx); contextErr != nil {
		return count, errors.Join(err, contextErr, context.Cause(ctx))
	}
	return count, err
}

var _ core.Validatable = ReadAtRequest{}
