package filestore

import (
	"context"
	"errors"

	"github.com/deliri/primitive/v2026/core"
)

// Scope failures retain Go's terminal identity and its native custom cause.
// Other refusals remain untouched, including contextstate's Err-only contract.
func scopeCancellationError(ctx context.Context, err error) error {
	if ctx == nil || (!errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded)) {
		return err
	}
	cause := scopeContextCause(ctx)
	if cause == nil || errors.Is(err, cause) {
		return err
	}
	return errors.Join(err, cause)
}

func scopeContextCause(ctx context.Context) (cause error) {
	defer func() {
		if recover() != nil {
			cause = core.ErrContextObservation
		}
	}()
	return context.Cause(ctx)
}
