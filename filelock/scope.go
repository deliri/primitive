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
// after return. Immediate contention does not invoke Use. The result separates
// operation errors from release and close errors; cancellation cannot skip cleanup.
func WithScope(ctx context.Context, request ScopeRequest) (ScopeResult, error) {
	if err := errors.Join(contextstate.Validate(ctx), request.Validate()); err != nil {
		return ScopeResult{}, err
	}
	var result ScopeResult
	scope, err := filestore.WithLockFileScope(ctx, filestore.LockFileScopeRequest{Carrier: request.Carrier, Use: func(ctx context.Context, file *os.File) error {
		var err error
		result.acquisition, err = Acquire(ctx, Request{File: file, Exclusivity: request.Exclusivity, Patience: request.Patience})
		if err != nil {
			return err
		}
		held, err := result.acquisition.Held()
		if err != nil || !held {
			return err
		}
		defer func() { result.releaseError = Release(context.WithoutCancel(ctx), file) }()
		return request.Use(ctx)
	}})
	if err != nil {
		return ScopeResult{}, err
	}
	result.carrierScope = scope
	return result, result.Validate()
}

// ScopeResult records the actual acquisition and native lifetime observations.
// Its zero value is invalid. A valid result proves cleanup was attempted; the
// caller must inspect operation and cleanup errors before claiming completion.
// No cleanup error is reclassified as an error from the caller's operation.
type ScopeResult struct {
	acquisition  Acquisition
	carrierScope filestore.FileScopeResult
	releaseError error
}

func (r ScopeResult) Validate() error       { return r.carrierScope.Validate() }
func (r ScopeResult) Held() (bool, error)   { return r.acquisition.Held() }
func (r ScopeResult) OperationError() error { return r.carrierScope.OperationError() }
func (r ScopeResult) CleanupError() error {
	return errors.Join(r.releaseError, r.carrierScope.CleanupError())
}

var _ core.Validatable = ScopeRequest{}
var _ core.Validatable = ScopeResult{}
