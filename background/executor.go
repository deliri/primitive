package background

import (
	"context"
	"errors"
	"sync"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

// Executor owns a fixed worker set and one cancellation/join observer. It must
// be opened and must not be copied. Only a first completion-callback panic is
// retained; action errors are delivered directly and never accumulated.
type Executor struct {
	mu      sync.Mutex
	ctx     context.Context
	cancel  context.CancelCauseFunc
	policy  Policy
	queue   chan Work
	done    chan struct{}
	workers sync.WaitGroup
	stopped bool
	failure error
}

func Open(request OpenRequest) (*Executor, error) {
	if err := request.Validate(); err != nil {
		return nil, err
	}
	ctx, cancel, err := temporal.WithCancellation(temporal.CancellationRequest{Parent: request.Parent})
	if err != nil {
		return nil, errors.Join(core.ErrBackgroundContract, err)
	}
	executor := &Executor{ctx: ctx, cancel: cancel, policy: request.Policy,
		queue: make(chan Work, int(request.Policy.Waiting)), done: make(chan struct{}, 0)}
	executor.workers.Add(int(request.Policy.Concurrent))
	for range request.Policy.Concurrent {
		go executor.worker()
	}
	go executor.observe()
	return executor, nil
}

func (e *Executor) Validate() error {
	if e == nil || e.ctx == nil || e.cancel == nil || e.queue == nil || e.done == nil {
		return core.ErrBackgroundContract
	}
	return e.policy.Validate()
}

// Submit transfers callback ownership only on success. The queue never blocks
// the caller, and is sealed under the same mutex that admits submissions.
func (e *Executor) Submit(work Work) error {
	if err := e.Validate(); err != nil {
		return err
	}
	if err := work.Validate(); err != nil {
		return err
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.stopped || e.ctx.Err() != nil {
		return errors.Join(core.ErrBackgroundStopped, e.ctx.Err(), context.Cause(e.ctx))
	}
	select {
	case e.queue <- work:
		return nil
	default:
		return core.ErrBackgroundCapacity
	}
}

func (e *Executor) seal() {
	e.mu.Lock()
	defer e.mu.Unlock()
	if !e.stopped {
		e.stopped = true
		close(e.queue)
	}
}

func (e *Executor) observe() {
	<-e.ctx.Done()
	e.seal()
	e.workers.Wait()
	close(e.done)
}

func (e *Executor) worker() {
	defer e.workers.Done()
	for work := range e.queue {
		e.execute(work)
	}
}

func (e *Executor) execute(work Work) {
	result := errors.Join(e.ctx.Err(), context.Cause(e.ctx))
	if result == nil {
		ctx, cancel, err := temporal.WithTimeout(temporal.TimeoutRequest{Parent: e.ctx, Duration: e.policy.Timeout})
		if err != nil {
			result = err
		} else {
			result = errors.Join(invoke(ctx, work.Run), ctx.Err(), context.Cause(ctx))
			cancel()
		}
	}
	if err := complete(work.Complete, result); err != nil {
		e.mu.Lock()
		if e.failure == nil {
			e.failure = err
		}
		e.mu.Unlock()
	}
}

func invoke(ctx context.Context, action func(context.Context) error) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			native, _ := recovered.(error)
			err = errors.Join(core.ErrBackgroundCallbackPanic, native)
		}
	}()
	return action(ctx)
}

func complete(observe func(error), result error) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			native, _ := recovered.(error)
			err = errors.Join(core.ErrBackgroundCallbackPanic, native)
		}
	}()
	observe(result)
	return nil
}

// Stop seals admission, cancels execution and joins accepted work. A terminal
// caller may fail the join; a later Stop can still observe completed cleanup.
func (e *Executor) Stop(ctx context.Context) error {
	if err := e.Validate(); err != nil {
		return err
	}
	if _, err := contextstate.Observe(ctx); err != nil {
		return errors.Join(core.ErrBackgroundContract, err)
	}
	e.cancel(context.Canceled)
	e.seal()
	select {
	case <-e.done:
		e.mu.Lock()
		defer e.mu.Unlock()
		return e.failure
	case <-ctx.Done():
		return errors.Join(core.ErrBackgroundStopped, ctx.Err(), context.Cause(ctx))
	}
}

var _ core.Validatable = (*Executor)(nil)
