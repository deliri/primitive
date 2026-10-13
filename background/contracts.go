package background

import (
	"context"
	"errors"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

// Policy declares resident resources and one cooperative operation lifetime.
// Waiting bounds callbacks not yet assigned to a worker; no lifetime work
// count or caller identity is retained.
type Policy struct {
	Concurrent uint16
	Waiting    uint16
	Timeout    temporal.Duration
}

func (p Policy) Validate() error {
	if p.Concurrent == 0 || p.Waiting == 0 || p.Timeout.IsZero() {
		return core.ErrBackgroundContract
	}
	if err := p.Timeout.Validate(); err != nil {
		return errors.Join(core.ErrBackgroundContract, err)
	}
	return nil
}

// OpenRequest lends the process lifetime and configured resident resources.
type OpenRequest struct {
	Parent context.Context
	Policy Policy
}

func (r OpenRequest) Validate() error {
	if err := r.Policy.Validate(); err != nil {
		return err
	}
	if err := contextstate.Validate(r.Parent); err != nil {
		return errors.Join(core.ErrBackgroundContract, err)
	}
	return nil
}

// Work lends one action and its completion observation. Complete is called
// exactly once for every accepted submission, including cancellation before
// Run starts. Both callbacks must return cooperatively; their retained closure
// data belongs to the caller. A refused submission invokes neither callback.
type Work struct {
	Run      func(context.Context) error
	Complete func(error)
}

func (w Work) Validate() error {
	if w.Run == nil || w.Complete == nil {
		return core.ErrBackgroundContract
	}
	return nil
}

var (
	_ core.Validatable = Policy{}
	_ core.Validatable = OpenRequest{}
	_ core.Validatable = Work{}
)
