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

// SSAFunctionArgumentObservation borrows a source-backed target in this unit.
// Zero means that no such target is available: no argument, opaque/merged
// value, nil function, imported body or synthetic wrapper. It does not assert
// that the receiving function invokes the argument.
type SSAFunctionArgumentObservation struct {
	Function *ssa.Function
	Value    ssa.Value
}

func (o SSAFunctionArgumentObservation) Validate() error {
	if o.Function == nil && o.Value == nil {
		return nil
	}
	if o.Function == nil || o.Function.Prog == nil || o.Function.Pkg == nil || o.Function.Signature == nil || o.Function.Syntax() == nil || len(o.Function.Blocks) == 0 || !nativeReceiverValuePresent(o.Value) {
		return core.ErrGoToolchainOutput
	}
	if _, function := types.Unalias(o.Value.Type()).Underlying().(*types.Signature); !function || !types.ConvertibleTo(o.Function.Type(), o.Value.Type()) {
		return core.ErrGoToolchainOutput
	}
	return nil
}
func (SSAFunctionArgumentObservation) goToolchainInternalFlow() {}

// LastFunctionArgument authenticates one call's actual final argument and
// follows only Go's explicit function representation changes and closure loan.
// No invocation simulation, variable table or function population is retained.
func (v SSAUnitView) LastFunctionArgument(ctx context.Context, r SSAFunctionArgumentRequest) (out SSAFunctionArgumentObservation, err error) {
	defer func() {
		err = errors.Join(err, temporal.Wait(temporal.WaitRequest{Context: ctx}))
		if err != nil {
			out = SSAFunctionArgumentObservation{}
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
	count := signature.Params().Len()
	if signature.Recv() != nil && !common.IsInvoke() {
		count++
	}
	if len(common.Args) != count {
		return out, core.ErrGoToolchainOutput
	}
	for index, argument := range common.Args {
		if err = v.functionArgumentBefore(ctx, argument, r.Call); err != nil {
			return out, err
		}
		var parameter *types.Var
		if signature.Recv() != nil && !common.IsInvoke() {
			if index == 0 {
				parameter = signature.Recv()
			} else {
				parameter = signature.Params().At(index - 1)
			}
		} else {
			parameter = signature.Params().At(index)
		}
		if !types.AssignableTo(argument.Type(), parameter.Type()) {
			return out, core.ErrGoToolchainOutput
		}
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
	before := ssa.Instruction(r.Call)
	for {
		if err = temporal.Wait(temporal.WaitRequest{Context: ctx}); err != nil {
			return out, err
		}
		switch operand := value.(type) {
		case *ssa.ChangeType:
			if err = v.functionArgumentBefore(ctx, operand.X, operand); err != nil {
				return out, err
			}
			if !types.ConvertibleTo(operand.X.Type(), operand.Type()) {
				return out, core.ErrGoToolchainOutput
			}
			before, value = operand, operand.X
		case *ssa.MakeClosure:
			target, native := operand.Fn.(*ssa.Function)
			if !native || target == nil || target.Signature == nil || target.Prog != v.Program {
				return out, core.ErrGoToolchainOutput
			}
			if target.Syntax() == nil {
				return out, nil
			}
			if target.Parent() != operand.Parent() || !v.argumentBindingFunctionOwned(target) || len(operand.Bindings) != len(target.FreeVars) {
				return out, core.ErrGoToolchainOutput
			}
			for index, binding := range operand.Bindings {
				if err = v.argumentBindingOperand(ctx, binding, target.FreeVars[index], target, operand); err != nil {
					return out, err
				}
			}
			before, value = operand, target
		case *ssa.Function:
			if err = v.functionArgumentBefore(ctx, operand, before); err != nil {
				return out, err
			}
			if operand.Pkg != v.Package || operand.Syntax() == nil || len(operand.Blocks) == 0 {
				return out, nil
			}
			out = SSAFunctionArgumentObservation{Function: operand, Value: common.Args[len(common.Args)-1]}
			return out, out.Validate()
		default:
			return out, nil
		}
	}
}

func (v SSAUnitView) functionArgumentBefore(ctx context.Context, value ssa.Value, before ssa.Instruction) error {
	if function, native := value.(*ssa.Function); native {
		if function == nil || function.Signature == nil || function.Prog != v.Program || function.Syntax() != nil && !v.argumentBindingFunctionOwned(function) {
			return core.ErrGoToolchainOutput
		}
		return temporal.Wait(temporal.WaitRequest{Context: ctx})
	}
	return v.fieldReadOperandBefore(ctx, value, before)
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
	_ internalFlow = SSAFunctionArgumentObservation{}
)
