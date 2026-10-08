package filelock

import (
	"context"
	"errors"
	"os"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

// ScopeRequest selects one native lock carrier and one synchronous operation.
// Primitive owns the carrier, acquisition, release and closure. Use receives
// no native handle and runs only while this scope actually holds the lock.
type ScopeRequest struct {
	Use         func(context.Context) error
	Carrier     filestore.LockFileRequest
	Exclusivity Exclusivity
	Patience    Patience
}

func (r ScopeRequest) Validate() error {
	if r.Use == nil {
		return core.ErrPrimitiveContract
	}
	return errors.Join(r.Carrier.Validate(), r.Exclusivity.Validate(), r.Patience.Validate())
}

// WithScope returns the historical acquisition observation after native
// cleanup. Held reports whether the operation was admitted, never a live hold
// after return. Immediate contention does not invoke Use. Errors preserve the
// operation, release and close identities; cancellation cannot skip cleanup.
func WithScope(ctx context.Context, request ScopeRequest) (acquisition Acquisition, resultErr error) {
	if err := errors.Join(contextstate.Validate(ctx), request.Validate()); err != nil {
		return Acquisition{}, err
	}
	scope, err := filestore.WithLockFileScope(ctx, filestore.LockFileScopeRequest{Carrier: request.Carrier, Use: func(ctx context.Context, file *os.File) (operationErr error) {
		var err error
		acquisition, err = Acquire(ctx, Request{File: file, Exclusivity: request.Exclusivity, Patience: request.Patience})
		if err != nil {
			return err
		}
		held, err := acquisition.Held()
		if err != nil || !held {
			return err
		}
		defer func() { operationErr = errors.Join(operationErr, Release(context.WithoutCancel(ctx), file)) }()
		return request.Use(ctx)
	}})
	if err != nil {
		return Acquisition{}, err
	}
	return acquisition, errors.Join(scope.Validate(), scope.OperationError(), scope.CleanupError())
}

var _ core.Validatable = ScopeRequest{}
