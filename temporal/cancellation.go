package temporal

import (
	"context"

	"github.com/deliri/primitive/v2026/core"
)

// CancellationRequest supplies the parent of a caller-owned cancellation
// lifetime. Go owns propagation, terminal state and the first winning cause.
type CancellationRequest struct {
	// Parent carries the caller's existing context lifetime.
	Parent context.Context
}

// Validate refuses a missing or broken parent while admitting Go's standard
// canceled and deadline-exceeded parents without changing their identities.
func (r CancellationRequest) Validate() error {
	if err := validateEffectParent(r.Parent); err != nil {
		return contractError("cancellation parent is invalid", err)
	}
	return nil
}

// WithCancellation creates a real Go cancellation lifetime. The caller owns
// the returned cancel function and must release it on every return path.
// No timer, goroutine, cause registry or cancellation model is introduced.
// Named results allow a broken parent contract to be contained at construction.
func WithCancellation(request CancellationRequest) (ctx context.Context, cancel context.CancelCauseFunc, err error) {
	if err := request.Validate(); err != nil {
		return nil, nil, err
	}
	defer func() {
		if recover() != nil {
			ctx = nil
			cancel = nil
			err = contractError("standard cancellation constructor panicked", core.ErrContextObservation)
		}
	}()
	ctx, cancel = context.WithCancelCause(request.Parent)
	return ctx, cancel, nil
}

var _ core.Validatable = CancellationRequest{}
