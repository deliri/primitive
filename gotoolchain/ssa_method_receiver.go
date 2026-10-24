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

// SSAMethodReceiverRequest selects one actual native call, defer or go instruction.
type SSAMethodReceiverRequest struct{ Call ssa.CallInstruction }

func (r SSAMethodReceiverRequest) Validate() error {
	if r.Call == nil || reflect.ValueOf(r.Call).IsNil() {
		return core.ErrGoToolchainContract
	}
	return nil
}
func (SSAMethodReceiverRequest) goToolchainInternalFlow() {}

// SSAMethodReceiverObservation lends the called method and its actual receiver
// operand. An ordinary function or opaque callable has neither. This describes
// the native call layout, without resolving storage or promoted field paths.
type SSAMethodReceiverObservation struct {
	Method   *types.Func
	Receiver ssa.Value
}

func (o SSAMethodReceiverObservation) Validate() error {
	if o.Method == nil && o.Receiver == nil {
		return nil
	}
	if o.Method == nil || o.Method.Signature() == nil || o.Method.Signature().Recv() == nil || !nativeReceiverValuePresent(o.Receiver) {
		return core.ErrGoToolchainOutput
	}
	return nil
}
func (SSAMethodReceiverObservation) goToolchainInternalFlow() {}

// MethodReceiver borrows Go's direct, interface or bound method receiver. It
// validates actual call and operand custody and retains no operand population.
// Native compiler construction remains owned by WithSSAUnit.
func (v SSAUnitView) MethodReceiver(ctx context.Context, r SSAMethodReceiverRequest) (out SSAMethodReceiverObservation, err error) {
	defer func() {
		if err == nil {
			err = temporal.Wait(temporal.WaitRequest{Context: ctx})
		}
		if err != nil {
			out = SSAMethodReceiverObservation{}
		}
	}()
	if err = errors.Join(temporal.Wait(temporal.WaitRequest{Context: ctx}), v.Validate(), r.Validate()); err != nil {
		return out, err
	}
	if block := r.Call.Block(); block == nil || block.Parent() == nil {
		return out, core.ErrGoToolchainContract
	}
	if err = v.referrerInstructionMembership(r.Call); err != nil {
		return out, err
	}
	untaken, queryErr := argumentCallUntakenBlock(r.Call.Block())
	if queryErr != nil {
		return out, queryErr
	}
	if untaken {
		return out, nil
	}
	common := r.Call.Common()
	if !nativeReceiverValuePresent(common.Value) {
		return out, core.ErrGoToolchainOutput
	}
	signature, signatureErr := nativeArgumentCallSignature(common)
	if signatureErr != nil {
		return out, signatureErr
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
	if closure, bound := common.Value.(*ssa.MakeClosure); bound {
		if target, native := closure.Fn.(*ssa.Function); !native || target == nil {
			return out, core.ErrGoToolchainOutput
		} else if len(closure.Bindings) != len(target.FreeVars) {
			return out, core.ErrGoToolchainOutput
		}
	}
	method := common.Method
	if target := common.StaticCallee(); target != nil {
		if target.Signature == nil {
			return out, core.ErrGoToolchainOutput
		}
		method, _ = target.Object().(*types.Func)
	}
	if method == nil {
		return out, nil
	}
	if method.Signature() == nil {
		return out, core.ErrGoToolchainOutput
	}
	if method.Signature().Recv() == nil {
		if common.IsInvoke() {
			return out, core.ErrGoToolchainOutput
		}
		return out, nil
	}
	var receiver ssa.Value
	var receiverType types.Type
	if common.IsInvoke() {
		receiver = common.Value
		receiverType = method.Signature().Recv().Type()
	} else if closure, bound := common.Value.(*ssa.MakeClosure); bound {
		target := closure.Fn.(*ssa.Function)
		if len(closure.Bindings) != 1 || target.FreeVars[0] == nil || target.FreeVars[0].Type() == nil {
			return out, core.ErrGoToolchainOutput
		}
		receiver = closure.Bindings[0]
		receiverType = target.FreeVars[0].Type()
	} else if len(common.Args) != 0 {
		receiver = common.Args[0]
		signature := common.StaticCallee().Signature
		if signature.Recv() != nil {
			receiverType = signature.Recv().Type()
		} else if signature.Params().Len() != 0 {
			receiverType = signature.Params().At(0).Type()
		}
	}
	if !nativeReceiverValuePresent(receiver) || receiverType == nil || !types.AssignableTo(receiver.Type(), receiverType) || receiver.Parent() != nil && receiver.Parent() != r.Call.Parent() {
		return out, core.ErrGoToolchainOutput
	}
	if instruction, native := receiver.(ssa.Instruction); native {
		if err = v.referrerInstructionMembership(instruction); err != nil {
			return out, errors.Join(core.ErrGoToolchainOutput, err)
		}
	} else if !v.ownsStorageValue(receiver) {
		return out, core.ErrGoToolchainOutput
	}
	if global, native := receiver.(*ssa.Global); native && v.Program.Package(global.Pkg.Pkg) != global.Pkg {
		return out, core.ErrGoToolchainOutput
	}
	return SSAMethodReceiverObservation{Method: method, Receiver: receiver}, nil
}

var _ internalFlow = SSAMethodReceiverRequest{}
var _ internalFlow = SSAMethodReceiverObservation{}
