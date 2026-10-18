package temporal

import (
	"context"

	"github.com/deliri/primitive/v2026/core"
)

// DetachedContextRequest retains parent values while removing cancellation and
// deadlines. It grants no timer or cancellation lifetime to the caller.
type DetachedContextRequest struct {
	Parent context.Context
}

// Validate follows Go's WithoutCancel admission: only an absent parent is
// refused. Detachment must not poll Err, Done, Deadline or Value on the parent.
func (r DetachedContextRequest) Validate() error {
	if r.Parent == nil {
		return contractError("detached context parent is absent", core.ErrNilContext)
	}
	return nil
}

// WithoutCancellation delegates the detached lifetime and value lookup to Go.
func WithoutCancellation(request DetachedContextRequest) (context.Context, error) {
	if err := request.Validate(); err != nil {
		return nil, err
	}
	return context.WithoutCancel(request.Parent), nil
}

var _ core.Validatable = DetachedContextRequest{}
