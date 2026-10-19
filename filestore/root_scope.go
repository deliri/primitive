package filestore

import (
	"context"
	"fmt"
	"os"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
)

// RootScopeRequest lends a real native root synchronously. The callback owns
// closure of any child handles it acquires and must not retain the root.
// Primitive owns acquisition and root closure, including callback refusal.
type RootScopeRequest struct {
	Directory   core.AbsolutePath
	Use         func(context.Context, *os.Root) error
	PanicPolicy ScopePanicPolicy
}

func (r RootScopeRequest) Validate() error {
	if err := r.Directory.Validate(); err != nil {
		return contractError(err)
	}
	if r.Use == nil {
		return core.ErrFilestoreContract
	}
	return r.PanicPolicy.Validate()
}

// RootScopeResult separates callback meaning from native cleanup. Its zero
// value is invalid: no acquired root lifetime has completed. A valid result
// proves closure was attempted, including when that attempt failed; callers
// must inspect CleanupError before claiming closure succeeded.
type RootScopeResult struct {
	OperationError error
	CleanupError   error
	completed      bool
}

func (r RootScopeResult) Validate() error {
	if !r.completed {
		return core.ErrFilestoreContract
	}
	return nil
}

// WithRootScope owns one OS root and returns only after its close attempt.
// Acquisition failures return an invalid zero result. The selected panic
// policy either sanitizes the callback panic or propagates it through native
// cleanup. Panic payloads are never returned or logged in scope results.
func WithRootScope(ctx context.Context, request RootScopeRequest) (result RootScopeResult, resultErr error) {
	defer func() {
		resultErr = scopeCancellationError(ctx, resultErr)
		result.OperationError = scopeCancellationError(ctx, result.OperationError)
	}()

	if err := contextstate.Validate(ctx); err != nil {
		return RootScopeResult{}, err
	}
	if err := request.Validate(); err != nil {
		return RootScopeResult{}, err
	}
	root, err := OpenRoot(ctx, request.Directory)
	if err != nil {
		return RootScopeResult{}, err
	}
	defer func() {
		result.CleanupError = root.Close()
		result.completed = true
		resultErr = result.Validate()
	}()
	result.OperationError = useRootScope(ctx, request, root)
	return result, nil
}

func useRootScope(ctx context.Context, request RootScopeRequest, root *os.Root) (resultErr error) {
	if request.PanicPolicy == ScopePanicPropagate {
		return request.Use(ctx, root)
	}
	defer func() {
		if recover() != nil {
			resultErr = fmt.Errorf("root scope callback: %w", core.ErrFilestoreCallbackPanic)
		}
	}()
	return request.Use(ctx, root)
}

var _ core.Validatable = RootScopeRequest{}
var _ core.Validatable = RootScopeResult{}
