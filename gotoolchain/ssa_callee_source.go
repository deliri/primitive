package gotoolchain

import (
	"context"
	"errors"
	"go/types"
	"reflect"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
	"golang.org/x/tools/go/ssa"
)

// SSACalleeSourceRequest selects an actual native call, defer or go instruction.
// It requests a source loan, without making an execution-timing claim.
type SSACalleeSourceRequest struct{ Call ssa.CallInstruction }

func (r SSACalleeSourceRequest) Validate() error {
	if r.Call == nil || reflect.ValueOf(r.Call).IsNil() || r.Call.Block() == nil || r.Call.Block().Parent() == nil {
		return core.ErrGoToolchainContract
	}
	return nil
}
func (SSACalleeSourceRequest) goToolchainInternalFlow() {}

// CalleeSource authenticates the native call and borrows its local source target.
// Dynamic interface dispatch, opaque values and synthetic/imported bodies yield
// absence. Function-valued arguments are never substituted for the callee.
func (v SSAUnitView) CalleeSource(ctx context.Context, r SSACalleeSourceRequest) (out SSAFunctionValueObservation, err error) {
	defer func() {
		err = errors.Join(err, temporal.Wait(temporal.WaitRequest{Context: ctx}))
		if err != nil {
			out = SSAFunctionValueObservation{}
		}
	}()
	if err = errors.Join(temporal.Wait(temporal.WaitRequest{Context: ctx}), v.Validate(), r.Validate()); err != nil {
		return out, err
	}
	if err = v.referrerInstructionMembership(r.Call); err != nil {
		return out, err
	}
	untaken, queryErr := argumentCallUntakenBlock(r.Call.Block())
	if queryErr != nil || untaken {
		return out, queryErr
	}
	common := r.Call.Common()
	signature, queryErr := nativeArgumentCallSignature(common)
	if queryErr != nil {
		return out, queryErr
	}
	if err = v.sourceCallArguments(ctx, r.Call, signature); err != nil {
		return out, err
	}
	if builtin, native := common.Value.(*ssa.Builtin); native {
		if _, owned := builtin.Object().(*types.Builtin); !owned {
			return out, core.ErrGoToolchainOutput
		}
		return out, nil
	}
	if err = v.functionArgumentBefore(ctx, common.Value, r.Call); err != nil {
		return out, err
	}
	if common.IsInvoke() {
		return out, nil
	}
	out, _, err = v.sourceFunctionValue(ctx, common.Value, r.Call)
	return out, err
}

var _ internalFlow = SSACalleeSourceRequest{}
