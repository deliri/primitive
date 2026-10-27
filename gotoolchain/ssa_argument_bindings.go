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
	return v.argumentBindings(ctx, r, nil)
}

func (v SSAUnitView) argumentBindings(ctx context.Context, r SSAArgumentBindingsRequest, invocation *SSAArgumentInvocationFrame) iter.Seq2[SSAArgumentBindingObservation, error] {
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
		if _, builtin := common.Value.(*ssa.Builtin); builtin {
			return
		}
		source, closure, sourceErr := v.sourceFunctionValue(ctx, common.Value, r.Call, invocation)
		if sourceErr != nil {
			refuse(sourceErr)
			return
		}
		target := source.Function
		if target == nil {
			return
		}
		if len(common.Args) != len(target.Params) {
			refuse(core.ErrGoToolchainOutput)
			return
		}
		var bindings []ssa.Value
		if closure != nil {
			bindings = closure.Bindings
		}
		selectedCapture, captureErr := argumentCaptureSource(ctx, r, invocation, closure)
		if captureErr != nil {
			refuse(captureErr)
			return
		}

		if len(bindings) != len(target.FreeVars) {
			refuse(core.ErrGoToolchainOutput)
			return
		}
		// Validate the complete native layout before lending any match.
		if validationErr := v.validateArgumentBindingOperands(ctx, r.Call, target, closure); validationErr != nil {
			refuse(validationErr)
			return
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
				selected := r.Argument
				if lane == 0 {
					selected = selectedCapture
				}
				matched, matchErr := argumentCallOperand(operand, selected)
				if matchErr != nil {
					refuse(matchErr)
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

// Borrow the selected capture from the existing native caller chain. Captures
// and ordinary arguments may originate in different native functions; neither
// a copied binding environment nor a second ancestry population is required.
func argumentCaptureSource(ctx context.Context, request SSAArgumentBindingsRequest, invocation *SSAArgumentInvocationFrame, closure *ssa.MakeClosure) (ssa.Value, error) {
	if closure == nil || closure.Parent() == request.Call.Parent() {
		return request.Argument, nil
	}
	for invocation != nil && invocation.binding.Function != closure.Parent() {
		if err := temporal.Wait(temporal.WaitRequest{Context: ctx}); err != nil {
			return nil, err
		}
		invocation = invocation.caller
	}
	if invocation == nil {
		return nil, core.ErrGoToolchainOutput
	}
	return invocation.binding.Local, nil
}

// Native declaration lookup and the compiler's lexical parent loan authenticate
// function identity without copying a function registry.
func (v SSAUnitView) argumentBindingFunctionOwned(fn *ssa.Function) bool {
	for fn != nil && fn.Prog == v.Program {
		// Go owns the synthetic initializer even though it has no types.Func.
		// Exact package-member identity authenticates its lexical children.
		if fn == v.Package.Func("init") {
			return true
		}
		if object, ok := fn.Object().(*types.Func); ok {
			owned := v.Program.FuncValue(object.Origin())
			if owned == fn {
				return true
			}
			// Go gives generic method instances their own object. Its origin
			// binds the declaration; the private block parent binds this exact
			// native instance and refuses a copied function or block.
			return owned != nil && fn.Origin() == owned && fn.Syntax() == owned.Syntax() &&
				len(fn.Blocks) != 0 && fn.Blocks[0] != nil && fn.Blocks[0].Parent() == fn && fn.Blocks[0].Index == 0
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

// Validate both borrowed native lanes before the first observation. A closure's
// captures are authenticated at their actual creation, including when the
// invocation later occurs in a helper's distinct native function.
func (v SSAUnitView) validateArgumentBindingOperands(ctx context.Context, call *ssa.Call, target *ssa.Function, closure *ssa.MakeClosure) error {
	var captures []ssa.Value
	if closure != nil {
		captures = closure.Bindings
	}
	for lane := 0; lane < 2; lane++ {
		width := len(captures)
		if lane == 1 {
			width = len(call.Common().Args)
		}
		for index := 0; index < width; index++ {
			if err := temporal.Wait(temporal.WaitRequest{Context: ctx}); err != nil {
				return err
			}
			var operand, local ssa.Value
			var before ssa.Instruction = call
			if lane == 0 {
				operand, local, before = captures[index], target.FreeVars[index], closure
			} else {
				operand, local = call.Common().Args[index], target.Params[index]
			}
			if err := v.argumentBindingOperand(ctx, operand, local, target, before); err != nil {
				return err
			}
		}
	}
	return nil
}
