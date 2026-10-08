package filestore

import (
	"context"
	"os"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
)

// LockFileScopeRequest lends a native carrier to a cooperating Primitive
// capability. Use must not close or retain the handle. No file or lock model
// is retained; acquisition and closure share the regular native scope owner.
type LockFileScopeRequest struct {
	Use     func(context.Context, *os.File) error
	Carrier LockFileRequest
}

func (r LockFileScopeRequest) Validate() error {
	if r.Use == nil {
		return core.ErrFilestoreContract
	}
	return r.Carrier.Validate()
}

func WithLockFileScope(ctx context.Context, request LockFileScopeRequest) (FileScopeResult, error) {
	if err := contextstate.Validate(ctx); err != nil {
		return FileScopeResult{}, err
	}
	if err := request.Validate(); err != nil {
		return FileScopeResult{}, err
	}
	file, err := OpenLockFile(ctx, request.Carrier)
	if err != nil {
		return FileScopeResult{}, err
	}
	result := finishFileScope(ctx, file, request.Use)
	return result, result.Validate()
}

var _ core.Validatable = LockFileScopeRequest{}
