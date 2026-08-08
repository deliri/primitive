package process

import (
	"context"
	"sync/atomic"

	"github.com/deliri/primitive/v2026/core"
)

// Execution is one started direct child between Begin and Wait.
//
// It exists so a supervisor can hold the running child as a typed value:
// register it, signal it from another goroutine, and hard-stop its whole
// group on a force path, all while the reaping wait is in flight. Deliver
// and Terminate are safe to call concurrently with Wait; after Wait returns,
// the identity may already name a reaped process, and under group isolation
// the group address outlives the leader only while some member still runs.
type Execution struct {
	streams     commandStreams
	parent      context.Context
	prepared    preparedCommand
	failures    *streamFailures
	cancel      context.CancelCauseFunc
	commandPath core.AbsolutePath
	identity    ProcessIdentity
	waited      atomic.Bool
	reaped      atomic.Bool
	containment Containment
}

// Identity returns the platform identifier of the started child, for durable
// execution records and diagnostics.
func (e *Execution) Identity() (ProcessIdentity, error) {
	if err := e.usable(); err != nil {
		return 0, err
	}
	if err := e.identity.Validate(); err != nil {
		return 0, err
	}
	return e.identity, nil
}

// usable reports the refusal every supervision door shares: an Execution that
// is nil or was not started by Begin holds no child, and both are caller
// defects to report loudly rather than a nil dereference into os/exec.
func (e *Execution) usable() error {
	if e == nil || e.prepared.command == nil || e.cancel == nil {
		return contractError("execution was not started by Begin")
	}
	return nil
}

// Deliver addresses one admitted signal to the child, or to the whole group
// the child leads under group isolation.
func (e *Execution) Deliver(signal CancelSignal) error {
	if err := e.usable(); err != nil {
		return err
	}
	// Supervision is scoped to the wait being in flight. Once Wait has
	// returned, the identity is a stored number the kernel may have handed to
	// an unrelated process or group, and signaling it would be exactly the
	// stored-number-later delivery the ProcessIdentity contract forbids.
	if e.reaped.Load() {
		return contractError("execution was already reaped")
	}
	if err := signal.Validate(); err != nil {
		return err
	}
	return deliverSignal(signalDelivery{
		process:     e.prepared.command.Process,
		identity:    e.identity,
		containment: e.containment,
		signal:      signal,
	})
}

// Terminate hard-stops the child, or the whole group the child leads under
// group isolation. It is the force path a supervisor takes when the polite
// cancellation has already been spent.
func (e *Execution) Terminate() error {
	return e.Deliver(CancelSignalKill)
}

// Wait streams to completion, reaps the direct child, and seals the
// observation. Exactly one wait exists per execution: a second call is a
// contract violation, not a cached answer, because the first caller may
// still be acting on the result.
func (e *Execution) Wait() (Result, error) {
	if err := e.usable(); err != nil {
		return Result{}, err
	}
	if !e.waited.CompareAndSwap(false, true) {
		return Result{}, contractError("execution was already reaped")
	}
	defer e.cancel(nil)
	defer e.reaped.Store(true)
	return waitCommand(waitRequest{
		parent:      e.parent,
		commandPath: e.commandPath,
		prepared:    e.prepared,
		streams:     e.streams,
		failures:    e.failures,
	})
}
