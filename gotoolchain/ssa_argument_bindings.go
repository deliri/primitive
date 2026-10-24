package gotoolchain

import (
	"context"
	"errors"
	"go/types"
	"iter"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
	"golang.org/x/tools/go/ssa"
)

// SSAArgumentBindingsRequest selects one actual synchronous invocation and one
// value in its caller. It does not request a call graph or helper traversal.
type SSAArgumentBindingsRequest struct {
	Call     *ssa.Call
	Argument ssa.Value
}

func (r SSAArgumentBindingsRequest) Validate() error {
	if r.Call == nil || !nativeReceiverValuePresent(r.Argument) {
		return core.ErrGoToolchainContract
	}
	return nil
}
func (SSAArgumentBindingsRequest) goToolchainInternalFlow() {}

// SSAArgumentBindingObservation lends a native callee parameter or free variable
// bound to the selected caller value. Multiple observations remain distinct;
// no binding, alias, or function population is retained.
type SSAArgumentBindingObservation struct {
	Function *ssa.Function
	Local    ssa.Value
}

func (o SSAArgumentBindingObservation) Validate() error {
	if o.Function == nil || !nativeReceiverValuePresent(o.Local) || o.Local.Parent() != o.Function {
		return core.ErrGoToolchainOutput
	}
	switch o.Local.(type) {
	case *ssa.Parameter, *ssa.FreeVar:
		return nil
	default:
		return core.ErrGoToolchainOutput
	}
}
func (SSAArgumentBindingObservation) goToolchainInternalFlow() {}

// ArgumentBindings follows only this call's native argument and closure layout.
// Opaque calls, imported bodies, compiler wrappers and known untaken calls lend
// no source binding. All native operand custody is checked before the first
// observation, so a malformed later argument cannot lend a valid prefix.
func (v SSAUnitView) ArgumentBindings(ctx context.Context, r SSAArgumentBindingsRequest) iter.Seq2[SSAArgumentBindingObservation, error] {
	return func(yield func(SSAArgumentBindingObservation, error) bool) {
		refuse := func(err error) { yield(SSAArgumentBindingObservation{}, err) }
		if err := errors.Join(temporal.Wait(temporal.WaitRequest{Context: ctx}), v.Validate(), r.Validate()); err != nil {
			refuse(err)
			return
		}
		if err := v.referrerInstructionMembership(r.Call); err != nil {
			refuse(err)
			return
		}
		if r.Argument.Parent() != r.Call.Parent() || !v.ownsStorageValue(r.Argument) {
			refuse(core.ErrGoToolchainContract)
			return
		}
		untaken, err := argumentCallUntakenBlock(r.Call.Block())
		if err != nil {
			refuse(err)
			return
		}
		if untaken {
			return
		}
		common := r.Call.Common()
		if !nativeReceiverValuePresent(common.Value) {
			refuse(core.ErrGoToolchainOutput)
			return
		}
		if closure, ok := common.Value.(*ssa.MakeClosure); ok {
			if !nativeReceiverValuePresent(closure.Fn) || !v.ownsStorageValue(closure) || closure.Parent() != r.Call.Parent() {
				refuse(core.ErrGoToolchainOutput)
				return
			}
			target, native := closure.Fn.(*ssa.Function)
			if !native || target.Syntax() != nil && target.Parent() != closure.Parent() {
				refuse(core.ErrGoToolchainOutput)
				return
			}
			if err := v.fieldReadOperandBefore(ctx, closure, r.Call); err != nil {
				refuse(err)
				return
			}
		}
		target := common.StaticCallee()
		if target == nil {
			return
		}
		if target.Syntax() == nil {
			return
		}
		if !v.argumentBindingFunctionOwned(target) {
			refuse(core.ErrGoToolchainOutput)
			return
		}
		if target.Pkg != v.Package {
			return
		}
		if target.Signature == nil || len(common.Args) != len(target.Params) {
			refuse(core.ErrGoToolchainOutput)
			return
		}
		var bindings []ssa.Value
		if closure, ok := common.Value.(*ssa.MakeClosure); ok {
			bindings = closure.Bindings
		}
		if len(bindings) != len(target.FreeVars) {
			refuse(core.ErrGoToolchainOutput)
			return
		}
		// Borrow two fixed native lanes. Validate the complete layout before
		// projecting matches; neither pass allocates a binding table.
		for lane := 0; lane < 2; lane++ {
			width := len(bindings)
			if lane == 1 {
				width = len(common.Args)
			}
			for index := 0; index < width; index++ {
				if err := temporal.Wait(temporal.WaitRequest{Context: ctx}); err != nil {
					refuse(err)
					return
				}
				var operand, local ssa.Value
				if lane == 0 {
					operand, local = bindings[index], target.FreeVars[index]
				} else {
					operand, local = common.Args[index], target.Params[index]
				}
				if err := v.argumentBindingOperand(ctx, operand, local, target, r.Call); err != nil {
					refuse(err)
					return
				}
			}
		}
		for lane := 0; lane < 2; lane++ {
			width := len(bindings)
			if lane == 1 {
				width = len(common.Args)
			}
			for index := 0; index < width; index++ {
				if err := temporal.Wait(temporal.WaitRequest{Context: ctx}); err != nil {
					refuse(err)
					return
				}
				var operand, local ssa.Value
				if lane == 0 {
					operand, local = bindings[index], target.FreeVars[index]
				} else {
					operand, local = common.Args[index], target.Params[index]
				}
				matched, err := argumentCallOperand(operand, r.Argument)
				if err != nil {
					refuse(err)
					return
				}
				if matched && !yield(SSAArgumentBindingObservation{Function: target, Local: local}, nil) {
					return
				}
			}
		}
		if err := temporal.Wait(temporal.WaitRequest{Context: ctx}); err != nil {
			refuse(err)
		}
	}
}

// Native declaration lookup and the compiler's lexical parent loan authenticate
// function identity without copying a function registry.
func (v SSAUnitView) argumentBindingFunctionOwned(fn *ssa.Function) bool {
	for fn != nil && fn.Prog == v.Program {
		if object, ok := fn.Object().(*types.Func); ok {
			owned := v.Program.FuncValue(object)
			return owned == fn || owned != nil && fn.Origin() != nil && owned == fn.Origin()
		}
		parent := fn.Parent()
		if parent == nil {
			return false
		}
		present := false
		for _, child := range parent.AnonFuncs {
			if child == fn {
				present = true
				break
			}
		}
		if !present {
			return false
		}
		fn = parent
	}
	return false
}

// Authenticate one native binding without creating an operand index. Entry and
// capture values have no instruction position; instructions must precede the
// actual loan boundary.
func (v SSAUnitView) argumentBindingOperand(ctx context.Context, operand, local ssa.Value, target *ssa.Function, before ssa.Instruction) error {
	if !nativeReceiverValuePresent(operand) || !nativeReceiverValuePresent(local) || local.Parent() != target || !types.AssignableTo(operand.Type(), local.Type()) {
		return core.ErrGoToolchainOutput
	}
	owned := false
	switch native := operand.(type) {
	case *ssa.Function:
		owned = v.argumentBindingFunctionOwned(native)
	case *ssa.Parameter, *ssa.FreeVar, *ssa.Const, *ssa.Global:
		owned = v.ownsStorageValue(operand)
	default:
		if err := v.fieldReadOperandBefore(ctx, operand, before); err != nil {
			return err
		}
		owned = true
	}
	if !owned || operand.Parent() != nil && operand.Parent() != before.Parent() {
		return core.ErrGoToolchainOutput
	}
	return nil
}

var _ internalFlow = SSAArgumentBindingsRequest{}
var _ internalFlow = SSAArgumentBindingObservation{}
