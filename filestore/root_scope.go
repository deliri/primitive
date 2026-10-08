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
	Directory core.AbsolutePath
	Use       func(context.Context, *os.Root) error
}

func (r RootScopeRequest) Validate() error {
	if err := r.Directory.Validate(); err != nil {
		return contractError(err)
	}
	if r.Use == nil {
		return core.ErrFilestoreContract
	}
	return nil
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
// Acquisition failures return an invalid zero result. Callback panic becomes
// a typed refusal; its payload is not returned or logged. There is no directory
// inventory, goroutine, filesystem model or product completion decision.
func WithRootScope(ctx context.Context, request RootScopeRequest) (result RootScopeResult, resultErr error) {
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
	result.OperationError = useRootScope(ctx, request.Use, root)
	return result, nil
}

func useRootScope(ctx context.Context, use func(context.Context, *os.Root) error, root *os.Root) (resultErr error) {
	defer func() {
		if recover() != nil {
			resultErr = fmt.Errorf("root scope callback panicked: %w", core.ErrFilestoreContract)
		}
	}()
	return use(ctx, root)
}

var _ core.Validatable = RootScopeRequest{}
var _ core.Validatable = RootScopeResult{}
