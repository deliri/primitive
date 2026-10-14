package process

import (
	"context"
	"errors"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

// WaitGroupAbsentRequest observes one borrowed execution. The caller chooses its
// timeout and polling cadence; neither choice supplies absence authority.
type WaitGroupAbsentRequest struct {
	Execution *Execution
	Timeout   temporal.Duration
	Interval  temporal.Duration
}

func (r WaitGroupAbsentRequest) Validate() error {
	if err := errors.Join(r.Execution.usable(), r.Timeout.Validate(), r.Interval.Validate()); err != nil {
		return err
	}
	if r.Interval.IsZero() {
		return core.ErrProcessContract
	}
	if r.Execution.containment.Isolation != IsolationGroup {
		return core.ErrProcessContract
	}
	return nil
}

// WaitGroupAbsent returns only a complete native Gone observation. It neither sends
// a signal nor waits/reaps the child. Caller-owned streams and Wait custody stay
// with the execution's owner. No identity or observation population is retained.
func WaitGroupAbsent(ctx context.Context, request WaitGroupAbsentRequest) (Liveness, error) {
	if err := errors.Join(contextstate.Validate(ctx), request.Validate()); err != nil {
		return LivenessUnknown, err
	}
	owner, cancel, err := temporal.WithTimeout(temporal.TimeoutRequest{Parent: ctx, Duration: request.Timeout})
	if err != nil {
		return LivenessUnknown, err
	}
	defer cancel()
	for {
		if err := contextstate.Validate(owner); err != nil {
			return LivenessUnknown, err
		}
		observed, err := request.Execution.GroupLiveness()
		if err := errors.Join(err, observed.Validate(), contextstate.Validate(owner)); err != nil {
			return LivenessUnknown, err
		}
		if observed == LivenessGone {
			return observed, nil
		}
		if err := temporal.Wait(temporal.WaitRequest{Context: owner, Duration: request.Interval}); err != nil {
			return LivenessUnknown, err
		}
	}
}
