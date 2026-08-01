package temporal

import (
	"context"
	"time"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
)

// TimeoutRequest supplies one parent and exact relative timeout.
type TimeoutRequest struct {
	Parent   context.Context
	Duration Duration
}

// Validate checks the timeout ingress while admitting standard terminal
// parents, which context.WithTimeout accepts.
func (r TimeoutRequest) Validate() error {
	if err := validateEffectParent(r.Parent); err != nil {
		return contractError("timeout parent is invalid", err)
	}
	if err := r.Duration.Validate(); err != nil {
		return contractError("timeout duration is invalid", err)
	}
	return nil
}

// DeadlineRequest supplies one parent and exact wall deadline.
type DeadlineRequest struct {
	Parent   context.Context
	Deadline Instant
}

// Validate checks the deadline ingress while admitting standard terminal
// parents, which context.WithDeadline accepts.
func (r DeadlineRequest) Validate() error {
	if err := validateEffectParent(r.Parent); err != nil {
		return contractError("deadline parent is invalid", err)
	}
	if err := r.Deadline.Validate(); err != nil {
		return contractError("deadline instant is invalid", err)
	}
	return nil
}

// WaitRequest supplies one context-owned wait.
type WaitRequest struct {
	Context  context.Context
	Duration Duration
}

// Validate requires a currently usable context and valid duration.
func (r WaitRequest) Validate() error {
	if err := contextstate.Validate(r.Context); err != nil {
		return err
	}
	if err := r.Duration.Validate(); err != nil {
		return contractError("wait duration is invalid", err)
	}
	return nil
}

// TickerRequest supplies one positive ticker interval.
type TickerRequest struct {
	Interval Duration
}

// Ticker is one caller-owned standard-library ticker capability. Its channel
// remains the Go time data plane while construction and ownership stay typed.
type Ticker struct {
	ticker *time.Ticker
}

type contextConstruction struct {
	ctx    context.Context
	cancel context.CancelFunc
	err    error
}

// Validate rejects zero because time.NewTicker requires a positive interval.
func (r TickerRequest) Validate() error {
	if err := r.Interval.Validate(); err != nil {
		return contractError("ticker interval is invalid", err)
	}
	if r.Interval.IsZero() {
		return contractError("ticker interval is zero")
	}
	return nil
}

// WithTimeout delegates one validated timeout to context.WithTimeout.
func WithTimeout(request TimeoutRequest) (
	context.Context,
	context.CancelFunc,
	error,
) {
	if err := request.Validate(); err != nil {
		return nil, nil, err
	}
	duration, err := request.Duration.Stdlib()
	if err != nil {
		return nil, nil, err
	}
	result := constructTimeout(request.Parent, duration)
	return result.ctx, result.cancel, result.err
}

// WithDeadline delegates one validated deadline to context.WithDeadline.
func WithDeadline(request DeadlineRequest) (
	context.Context,
	context.CancelFunc,
	error,
) {
	if err := request.Validate(); err != nil {
		return nil, nil, err
	}
	deadline, err := request.Deadline.Time()
	if err != nil {
		return nil, nil, err
	}
	result := constructDeadline(request.Parent, deadline)
	return result.ctx, result.cancel, result.err
}

func constructTimeout(parent context.Context, duration time.Duration) contextConstruction {
	var result contextConstruction
	func() {
		defer containContextConstructorPanic(&result)
		// #nosec G118 -- ownership of the real cancel function is returned.
		result.ctx, result.cancel = context.WithTimeout(parent, duration)
	}()
	return result
}

func constructDeadline(parent context.Context, deadline time.Time) contextConstruction {
	var result contextConstruction
	func() {
		defer containContextConstructorPanic(&result)
		// #nosec G118 -- ownership of the real cancel function is returned.
		result.ctx, result.cancel = context.WithDeadline(parent, deadline)
	}()
	return result
}

func containContextConstructorPanic(result *contextConstruction) {
	if recover() == nil {
		return
	}
	*result = contextConstruction{
		err: contractError("standard context constructor panicked", core.ErrContextObservation),
	}
}

// Wait blocks on one real standard-library timer or the caller's context.
func Wait(request WaitRequest) error {
	if err := request.Validate(); err != nil {
		return err
	}
	duration, err := request.Duration.Stdlib()
	if err != nil {
		return err
	}
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-request.Context.Done():
		return terminalContextError(request.Context)
	case <-timer.C:
		return contextstate.Validate(request.Context)
	}
}

func terminalContextError(ctx context.Context) error {
	state, err := contextstate.ObserveAfterDone(ctx)
	if err != nil {
		return err
	}
	switch state {
	case contextstate.StateCancelled:
		return context.Canceled
	case contextstate.StateDeadlineExceeded:
		return context.DeadlineExceeded
	default:
		return core.ErrContextObservation
	}
}

// OpenTicker returns a caller-owned real standard-library ticker capability.
func OpenTicker(request TickerRequest) (*Ticker, error) {
	if err := request.Validate(); err != nil {
		return nil, err
	}
	interval, err := request.Interval.Stdlib()
	if err != nil {
		return nil, err
	}
	return &Ticker{ticker: time.NewTicker(interval)}, nil
}

// Validate rejects an unset ticker capability.
func (t *Ticker) Validate() error {
	if t == nil || t.ticker == nil {
		return contractError("ticker is unset")
	}
	return nil
}

// Ticks returns the real standard-library tick channel.
func (t *Ticker) Ticks() <-chan time.Time {
	if t == nil || t.ticker == nil {
		return nil
	}
	return t.ticker.C
}

// Stop releases the underlying standard-library ticker resource.
func (t *Ticker) Stop() {
	if t != nil && t.ticker != nil {
		t.ticker.Stop()
	}
}

func validateEffectParent(parent context.Context) error {
	_, err := contextstate.Observe(parent)
	return err
}

var (
	_ core.Validatable = TimeoutRequest{}
	_ core.Validatable = DeadlineRequest{}
	_ core.Validatable = WaitRequest{}
	_ core.Validatable = TickerRequest{}
	_ core.Validatable = (*Ticker)(nil)
)
