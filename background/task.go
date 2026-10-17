package background

import (
	"context"
	"errors"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

// TaskRequest lends one cooperative action for its parent's lifetime. It has
// no independent clock budget, queue, schedule, or retry. Work's completion
// observer runs exactly once, including cancellation before Run starts.
type TaskRequest struct {
	Parent context.Context
	Work   Work
}

func (r TaskRequest) Validate() error {
	if err := r.Work.Validate(); err != nil {
		return err
	}
	if err := (temporal.CancellationRequest{Parent: r.Parent}).Validate(); err != nil {
		return errors.Join(core.ErrBackgroundContract, err)
	}
	return nil
}

// TaskResult observes the action, independently of whether a caller completed
// its wait. A failed Wait returns no action observation.
type TaskResult struct {
	ActionError     error
	CompletionError error
}

// Task owns one native Go goroutine and cancellation lifetime. It must not be
// copied. Every accepted action is joined by Close; an action that ignores its
// context cannot be forcibly stopped. Wait can bound the caller's wait without
// claiming an incomplete join or canceling the action behind another owner.
type Task struct {
	self   *Task
	ctx    context.Context
	cancel context.CancelCauseFunc
	done   chan struct{}
	result TaskResult
}

func StartTask(request TaskRequest) (*Task, error) {
	if err := request.Validate(); err != nil {
		return nil, err
	}
	ctx, cancel, err := temporal.WithCancellation(temporal.CancellationRequest{Parent: request.Parent})
	if err != nil {
		return nil, errors.Join(core.ErrBackgroundContract, err)
	}
	if err := ctx.Err(); err != nil {
		cause := context.Cause(ctx)
		cancel(nil)
		return nil, errors.Join(core.ErrBackgroundStopped, err, cause)
	}
	task := &Task{ctx: ctx, cancel: cancel, done: make(chan struct{})}
	task.self = task
	go task.run(request.Work)
	return task, nil
}

func (t *Task) Validate() error {
	if t == nil || t.self != t || t.ctx == nil || t.cancel == nil || t.done == nil {
		return core.ErrBackgroundContract
	}
	return nil
}

func (t *Task) run(work Work) {
	// Goexit runs defers without returning through invoke. Keep an explicit
	// stopped observation so that path joins without inventing action success.
	t.result = TaskResult{ActionError: core.ErrBackgroundStopped, CompletionError: core.ErrBackgroundStopped}
	defer func() {
		t.cancel(nil)
		close(t.done)
	}()
	defer func() {
		t.result.ActionError = errors.Join(t.result.ActionError, t.ctx.Err(), context.Cause(t.ctx))
		t.result.CompletionError = complete(work.Complete, t.result.ActionError)
	}()
	err := errors.Join(t.ctx.Err(), context.Cause(t.ctx))
	if err == nil {
		err = invoke(t.ctx, work.Run)
	}
	t.result.ActionError = err
}

// Cancel requests cooperative termination using Go's first-cause semantics.
// Cancellation does not by itself prove the action has returned.
func (t *Task) Cancel(cause error) error {
	if err := t.Validate(); err != nil {
		return err
	}
	t.cancel(cause)
	return nil
}

func (t *Task) Wait(parent context.Context) (TaskResult, error) {
	if err := t.Validate(); err != nil {
		return TaskResult{}, err
	}
	ctx, cancel, err := temporal.WithCancellation(temporal.CancellationRequest{Parent: parent})
	if err != nil {
		return TaskResult{}, errors.Join(core.ErrBackgroundContract, err)
	}
	defer cancel(nil)
	select {
	case <-t.done:
		return t.result, nil
	default:
	}
	select {
	case <-t.done:
		return t.result, nil
	case <-ctx.Done():
		return TaskResult{}, errors.Join(core.ErrBackgroundStopped, ctx.Err(), context.Cause(ctx))
	}
}

// Close cancels and joins cooperatively. A callback must not join itself.
func (t *Task) Close() (TaskResult, error) {
	if err := t.Cancel(context.Canceled); err != nil {
		return TaskResult{}, err
	}
	return t.Wait(context.Background())
}

var (
	_ core.Validatable = TaskRequest{}
	_ core.Validatable = (*Task)(nil)
)
