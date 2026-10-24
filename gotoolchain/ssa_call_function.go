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

// SSACallFunctionRequest selects one actual native call, defer or go record.
type SSACallFunctionRequest struct{ Call ssa.CallInstruction }

func (r SSACallFunctionRequest) Validate() error {
	if r.Call == nil || reflect.ValueOf(r.Call).IsNil() || r.Call.Block() == nil || r.Call.Block().Parent() == nil {
		return core.ErrGoToolchainContract
	}
	return nil
}
func (SSACallFunctionRequest) goToolchainInternalFlow() {}

// SSACallFunctionObservation lends Go's named callee object, including imported
// declarations whose bodies are unavailable. A nil function means Go provides
// no exact named target. It does not assert execution timing or a source body.
type SSACallFunctionObservation struct{ Function *types.Func }

func (o SSACallFunctionObservation) Validate() error {
	if o.Function != nil && (o.Function.Pkg() == nil || o.Function.Type() == nil || o.Function.Signature() == nil) {
		return core.ErrGoToolchainOutput
	}
	return nil
}
func (SSACallFunctionObservation) goToolchainInternalFlow() {}

// CallFunction borrows the object supplied by Go's native StaticCallee query.
// Opaque, merged, builtin, interface-dispatched and literal targets remain
// absent. All actual call arguments and callable custody are checked first.
func (v SSAUnitView) CallFunction(ctx context.Context, r SSACallFunctionRequest) (out SSACallFunctionObservation, err error) {
	defer func() {
		err = errors.Join(err, temporal.Wait(temporal.WaitRequest{Context: ctx}))
		if err != nil {
			out = SSACallFunctionObservation{}
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
	if _, builtin := common.Value.(*ssa.Builtin); builtin {
		return out, nil
	}
	if err = v.functionArgumentBefore(ctx, common.Value, r.Call); err != nil {
		return out, err
	}
	target := common.StaticCallee()
	if target == nil {
		return out, nil
	}
	if target.Prog != v.Program || target.Signature == nil {
		return out, core.ErrGoToolchainOutput
	}
	function, named := target.Object().(*types.Func)
	if !named || function.Pkg() == nil {
		return out, nil
	}
	out = SSACallFunctionObservation{Function: function}
	return out, out.Validate()
}

var (
	_ internalFlow = SSACallFunctionRequest{}
	_ internalFlow = SSACallFunctionObservation{}
)
