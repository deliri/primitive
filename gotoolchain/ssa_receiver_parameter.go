package gotoolchain

import (
	"context"
	"errors"
	"go/types"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
	"golang.org/x/tools/go/ssa"
)

// SSAReceiverParameterRequest selects one actual call and a parameter in its
// native function. It describes no product method or invocation search.
type SSAReceiverParameterRequest struct {
	Call      *ssa.Call
	Parameter *ssa.Parameter
}

func (r SSAReceiverParameterRequest) Validate() error {
	if r.Call == nil || r.Parameter == nil {
		return core.ErrGoToolchainContract
	}
	return nil
}
func (SSAReceiverParameterRequest) goToolchainInternalFlow() {}

// ReceiverParameter compares the call's actual receiver with a selected native
// parameter, following Go's promoted field path and explicit representations.
// An escaped or replaced capture cell supplies no identity proof. It never
// borrows authority from another call, block or parameter with the same name.
func (v SSAUnitView) ReceiverParameter(ctx context.Context, r SSAReceiverParameterRequest) (out SSARelationObservation, err error) {
	defer func() {
		if err == nil {
			err = temporal.Wait(temporal.WaitRequest{Context: ctx})
		}
		if err != nil {
			out = SSARelationObservation{}
		}
	}()
	if err = errors.Join(temporal.Wait(temporal.WaitRequest{Context: ctx}), v.Validate(), r.Validate()); err != nil {
		return out, err
	}
	block := r.Call.Block()
	if block == nil || block.Parent() == nil {
		return out, core.ErrGoToolchainContract
	}
	fn := block.Parent()
	if fn.Prog != v.Program || fn.Pkg != v.Package || r.Parameter.Parent() != fn || !v.ownsStorageValue(r.Parameter) {
		return out, core.ErrGoToolchainContract
	}
	if err = v.referrerInstructionMembership(r.Call); err != nil {
		return out, err
	}
	if argumentCallUntakenBlock(block) {
		return out, nil
	}
	common := r.Call.Common()
	if !nativeReceiverValuePresent(common.Value) {
		return out, core.ErrGoToolchainOutput
	}
	object := common.Method
	if target := common.StaticCallee(); target != nil {
		if target.Signature == nil {
			return out, core.ErrGoToolchainOutput
		}
		object, _ = target.Object().(*types.Func)
	}
	if object == nil || object.Pkg() == nil || object.Signature().Recv() == nil {
		return out, nil
	}
	selection, present := types.LookupSelection(r.Parameter.Type(), false, object.Pkg(), object.Name())
	if !present {
		return out, nil
	}
	selected, method := selection.Obj().(*types.Func)
	if !method || selected.Origin() != object.Origin() {
		return out, nil
	}
	var receiver ssa.Value
	if common.IsInvoke() {
		receiver = common.Value
	} else if closure, bound := common.Value.(*ssa.MakeClosure); bound {
		if len(closure.Bindings) != 1 {
			return out, core.ErrGoToolchainOutput
		}
		receiver = closure.Bindings[0]
	} else if len(common.Args) > 0 {
		receiver = common.Args[0]
	} else {
		return out, core.ErrGoToolchainOutput
	}
	if !nativeReceiverValuePresent(receiver) {
		return out, core.ErrGoToolchainOutput
	}
	if receiver.Parent() != nil && receiver.Parent() != fn {
		return out, core.ErrGoToolchainOutput
	}
	if instruction, native := receiver.(ssa.Instruction); native {
		if err = v.referrerInstructionMembership(instruction); err != nil {
			return out, errors.Join(core.ErrGoToolchainOutput, err)
		}
	} else if !v.ownsStorageValue(receiver) {
		return out, core.ErrGoToolchainOutput
	}
	indices := selection.Index()
	out.Proven = argumentCallReceiver(receiver, r.Parameter, indices[:len(indices)-1])
	return out, nil
}

var _ internalFlow = SSAReceiverParameterRequest{}
