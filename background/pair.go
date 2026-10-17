package background

import (
	"context"
	"errors"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

// PairRequest lends two concurrent sources and their synchronous consumers.
// Each offer waits until its consumer returns. Consumers run serially on the
// RunPair caller's goroutine, so a caller-owned capability never enters either
// producer goroutine. Sources observe each offer's result and must honor
// cancellation. An offer refusal does not preempt a source's cleanup or its
// policy for emitting subsequent canceled observations.
// There is no queue, input inventory, operation timeout or lifetime work quota.
type PairRequest[A, B any] struct {
	First         func(context.Context, func(A) error) error
	Second        func(context.Context, func(B) error) error
	ConsumeFirst  func(context.Context, A) error
	ConsumeSecond func(context.Context, B) error
}

func (r PairRequest[A, B]) Validate() error {
	if r.First == nil || r.Second == nil || r.ConsumeFirst == nil || r.ConsumeSecond == nil {
		return core.ErrBackgroundContract
	}
	return nil
}

// PairResult reports each source only after its cleanup has returned. A source
// cannot erase the first consumer refusal by ignoring the offer error and
// returning nil. Only that first refusal and the final source error are kept;
// every individual offer result is returned directly to its producer.
// Failure on one side does not cancel its independent sibling.
type PairResult struct {
	FirstError  error
	SecondError error
}

type pairOffer[T any] struct {
	value T
	reply chan error
}

func offerPairValue[T any](ctx context.Context, destination chan<- pairOffer[T], value T) error {
	if err := ctx.Err(); err != nil {
		return errors.Join(err, context.Cause(ctx))
	}
	reply := make(chan error, 1)
	select {
	case destination <- pairOffer[T]{value: value, reply: reply}:
	case <-ctx.Done():
		return errors.Join(ctx.Err(), context.Cause(ctx))
	}
	select {
	case err := <-reply:
		return err
	case <-ctx.Done():
		return errors.Join(ctx.Err(), context.Cause(ctx))
	}
}

func runPairSource[T any](ctx context.Context, cancel context.CancelCauseFunc, source func(context.Context, func(T) error) error, offers chan<- pairOffer[T], done chan<- error) {
	resultErr := error(core.ErrBackgroundStopped)
	defer func() {
		resultErr = errors.Join(resultErr, ctx.Err(), context.Cause(ctx))
		cancel(nil)
		done <- resultErr
	}()
	resultErr = invoke(ctx, func(ctx context.Context) error {
		return source(ctx, func(value T) error { return offerPairValue(ctx, offers, value) })
	})
}

// RunPair owns two native Go goroutines and joins both on every exit, including
// a consumer panic or Goexit. Consumer panics are contained with their native
// error cause; source and consumer errors belong to their respective result.
// Caller cancellation is cooperative: a source which ignores its context and
// never returns cannot be forcibly joined by Go.
func RunPair[A, B any](ctx context.Context, request PairRequest[A, B]) (result PairResult, resultErr error) {
	if err := request.Validate(); err != nil {
		return PairResult{}, err
	}
	firstCtx, cancelFirst, err := temporal.WithCancellation(temporal.CancellationRequest{Parent: ctx})
	if err != nil {
		return PairResult{}, err
	}
	defer cancelFirst(nil)
	if err := contextstate.Validate(firstCtx); err != nil {
		return PairResult{}, errors.Join(err, context.Cause(firstCtx))
	}
	secondCtx, cancelSecond, err := temporal.WithCancellation(temporal.CancellationRequest{Parent: ctx})
	if err != nil {
		return PairResult{}, err
	}
	defer cancelSecond(nil)
	firstOffers, secondOffers := make(chan pairOffer[A]), make(chan pairOffer[B])
	firstDone, secondDone := make(chan error, 1), make(chan error, 1)
	go runPairSource(firstCtx, cancelFirst, request.First, firstOffers, firstDone)
	go runPairSource(secondCtx, cancelSecond, request.Second, secondOffers, secondDone)
	// Unwinding a consumer releases both blocked offers before joining. Normal
	// completion has already read each result, so these nil channels name no join.
	defer func() {
		cancelFirst(context.Canceled)
		cancelSecond(context.Canceled)
		if firstDone != nil {
			result.FirstError = errors.Join(result.FirstError, <-firstDone)
		}
		if secondDone != nil {
			result.SecondError = errors.Join(result.SecondError, <-secondDone)
		}
	}()
	for firstDone != nil || secondDone != nil {
		select {
		case offered := <-firstOffers:
			err := errors.Join(firstCtx.Err(), context.Cause(firstCtx))
			if err == nil {
				err = invoke(firstCtx, func(ctx context.Context) error { return request.ConsumeFirst(ctx, offered.value) })
			}
			if result.FirstError == nil {
				result.FirstError = err
			}
			offered.reply <- err
		case offered := <-secondOffers:
			err := errors.Join(secondCtx.Err(), context.Cause(secondCtx))
			if err == nil {
				err = invoke(secondCtx, func(ctx context.Context) error { return request.ConsumeSecond(ctx, offered.value) })
			}
			if result.SecondError == nil {
				result.SecondError = err
			}
			offered.reply <- err
		case err := <-firstDone:
			result.FirstError = errors.Join(result.FirstError, err)
			firstDone = nil
		case err := <-secondDone:
			result.SecondError = errors.Join(result.SecondError, err)
			secondDone = nil
		}
	}
	return result, nil
}
