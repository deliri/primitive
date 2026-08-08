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

// Sweep hard-stops every process still running in the group the child leads
// or led. It exists because a tree can outlive its reaped leader: a run
// cancelled mid-tree, or a wait released by WaitDelay while a descendant held
// an inherited pipe, leaves survivors that Deliver and Terminate can no
// longer address once Wait has returned.
//
// The group address is not the stored-number hazard the direct pid is. The
// kernel keeps the leader's number bound to the group while any member still
// runs, so a sweep reaches only the created group or learns that it is gone.
// A group already gone, or one this process may no longer address, is a
// successful sweep rather than a failure, because neither can be repaired by
// retrying. The residual window is named rather than hidden: once the last
// member exits, the number may be recycled into an unrelated new group, so a
// supervisor sweeps on evidence of survivors, never as routine hygiene after
// every reaped run.
//
// A directly contained child is refused: it has no group address, and its
// stored number after reap is exactly the recycled-identity delivery the
// ProcessIdentity contract forbids. Sweep is legal while the wait is still
// in flight, where it is the absence-tolerant force stop a drain path wants.
func (e *Execution) Sweep() error {
	if err := e.usable(); err != nil {
		return err
	}
	if e.containment.Isolation != IsolationGroup {
		return contractError("sweep addresses a whole group; this containment is direct")
	}
	return sweepGroup(e.identity)
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
