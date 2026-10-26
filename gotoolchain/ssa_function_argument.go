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

// SSAFunctionArgumentRequest selects one actual native call. Go owns argument
// layout, including receivers; the final non-receiver argument is selected.
type SSAFunctionArgumentRequest struct{ Call ssa.CallInstruction }

func (r SSAFunctionArgumentRequest) Validate() error {
	if r.Call == nil || reflect.ValueOf(r.Call).IsNil() || r.Call.Block() == nil || r.Call.Block().Parent() == nil {
		return core.ErrGoToolchainContract
	}
	return nil
}
func (SSAFunctionArgumentRequest) goToolchainInternalFlow() {}

// LastFunctionArgument authenticates one call's actual final argument and
// follows only Go's explicit function representation changes and closure loan.
// No invocation simulation, variable table or function population is retained.
func (v SSAUnitView) LastFunctionArgument(ctx context.Context, r SSAFunctionArgumentRequest) (out SSAFunctionValueObservation, err error) {
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
	if signature.Params().Len() == 0 {
		return out, nil
	}
	value := common.Args[len(common.Args)-1]
	if err = v.functionArgumentBefore(ctx, value, r.Call); err != nil {
		return out, err
	}
	if !types.AssignableTo(value.Type(), signature.Params().At(signature.Params().Len()-1).Type()) {
		return out, core.ErrGoToolchainOutput
	}
	if _, function := types.Unalias(value.Type()).Underlying().(*types.Signature); !function {
		return out, nil
	}
	out, _, err = v.sourceFunctionValue(ctx, value, r.Call, nil)
	return out, err
}

// Signature is Go's authoritative native call-layout query, including calls
// through a type parameter. Contain malformed-provider panics at this boundary.
func nativeArgumentCallSignature(call *ssa.CallCommon) (signature *types.Signature, err error) {
	defer func() {
		if failure := recover(); failure != nil {
			signature, err = nil, core.ErrGoToolchainOutput
			if cause, native := failure.(error); native {
				err = errors.Join(err, cause)
			}
		}
	}()
	if call == nil || !nativeReceiverValuePresent(call.Value) {
		return nil, core.ErrGoToolchainOutput
	}
	signature = call.Signature()
	if signature == nil {
		return nil, core.ErrGoToolchainOutput
	}
	return signature, nil
}

var (
	_ internalFlow = SSAFunctionArgumentRequest{}
)
