package gotoolchain

import (
	"context"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
	"go/token"
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
func (v SSAUnitView) sourceFunctionValue(ctx context.Context, original ssa.Value, before ssa.Instruction, invocation *SSAArgumentInvocationFrame) (out SSAFunctionValueObservation, closure *ssa.MakeClosure, err error) {
	value := original
	if err = v.functionArgumentBefore(ctx, value, before); err != nil {
		return out, nil, err
	}
	for {
		if err = temporal.Wait(temporal.WaitRequest{Context: ctx}); err != nil {
			return out, nil, err
		}
		switch operand := value.(type) {
		case *ssa.Parameter:
			if invocation == nil || invocation.kind != SSAInvocationKindArgument || invocation.caller == nil {
				return out, nil, nil
			}
			if invocation.owner != v.Program || operand.Parent() != invocation.binding.Function || before.Parent() != invocation.binding.Function || invocation.call == nil || invocation.call.Parent() != invocation.caller.binding.Function {
				return out, nil, core.ErrGoToolchainContract
			}
			if err = v.referrerInstructionMembership(invocation.call); err != nil {
				return out, nil, err
			}
			parent, _, parentErr := v.sourceFunctionValue(ctx, invocation.call.Common().Value, invocation.call, invocation.caller)
			if parentErr != nil {
				return out, nil, parentErr
			}
			if parent.Function != invocation.binding.Function || len(invocation.call.Common().Args) != len(parent.Function.Params) {
				return out, nil, core.ErrGoToolchainOutput
			}
			signature, signatureErr := nativeArgumentCallSignature(invocation.call.Common())
			if signatureErr != nil {
				return out, nil, signatureErr
			}
			if err = v.sourceCallArguments(ctx, invocation.call, signature); err != nil {
				return out, nil, err
			}
			index := 0
			for index < len(parent.Function.Params) && parent.Function.Params[index] != operand {
				index++
			}
			if index == len(parent.Function.Params) {
				return out, nil, core.ErrGoToolchainOutput
			}
			original = invocation.call.Common().Args[index]
			before, value, invocation = invocation.call, original, invocation.caller
			if err = v.functionArgumentBefore(ctx, value, before); err != nil {
				return out, nil, err
			}
		case *ssa.UnOp:
			if operand.Op != token.MUL {
				return out, nil, nil
			}
			if _, function := operand.Type().Underlying().(*types.Signature); !function {
				return out, nil, nil
			}
			stored, queryErr := v.storedValue(ctx, operand)
			if queryErr != nil {
				return out, nil, queryErr
			}
			if stored == operand {
				return out, nil, nil
			}
			before, value = operand, stored
		case *ssa.ChangeType:
			if err = v.functionArgumentBefore(ctx, operand.X, operand); err != nil {
				return out, nil, err
			}
			if !types.ConvertibleTo(operand.X.Type(), operand.Type()) {
				return out, nil, core.ErrGoToolchainOutput
			}
			before, value = operand, operand.X
		case *ssa.MakeClosure:
			target, native := operand.Fn.(*ssa.Function)
			if !native || target == nil || target.Signature == nil || target.Prog != v.Program {
				return out, nil, core.ErrGoToolchainOutput
			}
			if target.Syntax() == nil {
				return out, nil, nil
			}
			if target.Parent() != operand.Parent() || !v.argumentBindingFunctionOwned(target) || len(operand.Bindings) != len(target.FreeVars) {
				return out, nil, core.ErrGoToolchainOutput
			}
			for index, binding := range operand.Bindings {
				if err = v.argumentBindingOperand(ctx, binding, target.FreeVars[index], target, operand); err != nil {
					return out, nil, err
				}
			}
			closure = operand
			before, value = operand, target
		case *ssa.Function:
			if err = v.functionArgumentBefore(ctx, operand, before); err != nil {
				return out, nil, err
			}
			if operand.Pkg != v.Package || operand.Syntax() == nil || len(operand.Blocks) == 0 {
				return out, nil, nil
			}
			out = SSAFunctionValueObservation{Function: operand, Value: original}
			return out, closure, out.Validate()
		default:
			return out, nil, nil
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
