package firestoredocuments

import (
	"context"
	"errors"

	"github.com/deliri/primitive/v2026/contextstate"
)

func documentContext(ctx context.Context) error {
	err := contextstate.Validate(ctx)
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return errors.Join(err, context.Cause(ctx))
	}
	return err
}
