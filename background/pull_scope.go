package background

import (
	"context"
	"errors"
	"iter"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

// PullScopeRequest lends one forward sequence and its synchronous consumer.
// Primitive owns the native pull coroutine until Use returns or unwinds. No
// producer value is requested before Next, and no inventory or queue is built.
type PullScopeRequest[T any] struct {
	Source iter.Seq2[T, error]
	Use    func(context.Context, *PullSource[T]) error
}

func (r PullScopeRequest[T]) Validate() error {
	if r.Source == nil || r.Use == nil {
		return core.ErrBackgroundContract
	}
	return nil
}

// PullSource is a synchronous loan of iter.Pull2's next operation. It must
// not be copied, retained after Use, or used concurrently. Next resumes only
// its source, on the native Go coroutine; producer defers run before the scope
// returns, including an early consumer return, panic, or Goexit.
type PullSource[T any] struct {
	self *PullSource[T]
	ctx  context.Context
	next func() (T, error, bool)
}

func (s *PullSource[T]) Validate() error {
	if s == nil || s.self != s || s.ctx == nil {
		return core.ErrBackgroundContract
	}
	if s.next == nil {
		return core.ErrBackgroundStopped
	}
	return nil
}

// Next returns either one exact source value, clean exhaustion, or a refusal
// with a zero value. A source error never lends a partial value beside it.
// Caller cancellation does not resume the producer; scope cleanup still stops
// and joins the suspended native coroutine.
func (s *PullSource[T]) Next() (value T, present bool, err error) {
	if err := s.Validate(); err != nil {
		return value, false, err
	}
	if err := contextstate.Validate(s.ctx); err != nil {
		return value, false, errors.Join(err, context.Cause(s.ctx))
	}
	item, sourceErr, ok := s.next()
	if sourceErr != nil {
		return value, false, sourceErr
	}
	if !ok {
		return value, false, nil
	}
	return item, true, nil
}

// WithPullScope delegates suspension, resumption and stop to iter.Pull2. Use
// runs on the calling goroutine and finishes before the loan is invalidated.
// Native source/consumer panics retain background's callback identity and
// native error cause. There is no operation timeout or total-item limit.
func WithPullScope[T any](ctx context.Context, request PullScopeRequest[T]) (resultErr error) {
	if err := request.Validate(); err != nil {
		return err
	}
	ctx, cancel, err := temporal.WithCancellation(temporal.CancellationRequest{Parent: ctx})
	if err != nil {
		return err
	}
	defer cancel(nil)
	if err := contextstate.Validate(ctx); err != nil {
		return errors.Join(err, context.Cause(ctx))
	}
	next, stop := iter.Pull2(request.Source)
	source := &PullSource[T]{ctx: ctx, next: next}
	source.self = source
	defer func() {
		source.next = nil
		cleanupErr := invoke(context.WithoutCancel(ctx), func(context.Context) error { stop(); return nil })
		resultErr = errors.Join(resultErr, cleanupErr, ctx.Err(), context.Cause(ctx))
	}()
	return invoke(ctx, func(ctx context.Context) error { return request.Use(ctx, source) })
}
