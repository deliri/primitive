package gotoolchain

import (
	"context"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
	"go/types"
	"golang.org/x/tools/go/ssa"
)

// SSAFunctionValueObservation borrows a source-backed target in this unit.
// Zero means that no such target is available: no argument, opaque/merged
// value, nil function, imported body or synthetic wrapper. It does not assert
// when or whether a caller executes the target.
type SSAFunctionValueObservation struct {
	Function *ssa.Function
	Value    ssa.Value
}

func (o SSAFunctionValueObservation) Validate() error {
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
func (SSAFunctionValueObservation) goToolchainInternalFlow() {}

// sourceFunctionValue follows native representation edges without retaining a graph.
func (v SSAUnitView) sourceFunctionValue(ctx context.Context, original ssa.Value, before ssa.Instruction) (out SSAFunctionValueObservation, err error) {
	value := original
	if err = v.functionArgumentBefore(ctx, value, before); err != nil {
		return out, err
	}
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
			out = SSAFunctionValueObservation{Function: operand, Value: original}
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

func (v SSAUnitView) sourceCallArguments(ctx context.Context, call ssa.CallInstruction, signature *types.Signature) (err error) {
	common := call.Common()
	count := signature.Params().Len()
	if signature.Recv() != nil && !common.IsInvoke() {
		count++
	}
	if len(common.Args) != count {
		return core.ErrGoToolchainOutput
	}
	for index, argument := range common.Args {
		if err = v.functionArgumentBefore(ctx, argument, call); err != nil {
			return err
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
			return core.ErrGoToolchainOutput
		}
	}
	return nil
}

var _ internalFlow = SSAFunctionValueObservation{}
