package gotoolchain

import (
	"context"
	"errors"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

// InvocationCalleeSource observes a callee in one actual borrowed invocation.
// It preserves CalleeSource's absence for a root's unknown callable parameter.
// A supplied caller frame may establish the exact argument of a local helper;
// the observation grants no universal runtime execution claim.
func (v SSAUnitView) InvocationCalleeSource(ctx context.Context, request SSAArgumentInvocationRequest) (out SSAFunctionValueObservation, err error) {
	defer func() {
		err = errors.Join(err, temporal.Wait(temporal.WaitRequest{Context: ctx}))
		if err != nil {
			out = SSAFunctionValueObservation{}
		}
	}()
	if err = errors.Join(temporal.Wait(temporal.WaitRequest{Context: ctx}), v.Validate(), request.Validate()); err != nil {
		return out, err
	}
	if request.Caller.owner != v.Program || request.Call.Parent() != request.Caller.binding.Function {
		return out, core.ErrGoToolchainContract
	}
	out, err = v.CalleeSource(ctx, SSACalleeSourceRequest{Call: request.Call})
	if err != nil || out.Function != nil {
		return out, err
	}
	untaken, queryErr := argumentCallUntakenBlock(request.Call.Block())
	if queryErr != nil || untaken {
		return out, queryErr
	}
	out, _, err = v.sourceFunctionValue(ctx, request.Call.Common().Value, request.Call, request.Caller)
	return out, err
}
