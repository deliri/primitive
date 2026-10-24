package gotoolchain

import (
	"context"
	"errors"
	"iter"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
	"golang.org/x/tools/go/ssa"
)

// SSAClosureBindingsRequest selects an actual closure construction and a value
// in its lexical owner. It does not assert that the closure will be invoked.
type SSAClosureBindingsRequest struct {
	Closure  *ssa.MakeClosure
	Argument ssa.Value
}

func (r SSAClosureBindingsRequest) Validate() error {
	if r.Closure == nil || !nativeReceiverValuePresent(r.Argument) {
		return core.ErrGoToolchainContract
	}
	return nil
}
func (SSAClosureBindingsRequest) goToolchainInternalFlow() {}

// ClosureBindings lends only exact native free variables bound to the selected
// owner value. Callback parameters are not captures and supply no observation.
// All captures are authenticated before the first match; no closure catalog,
// invocation model, environment copy or assignment history is constructed.
func (v SSAUnitView) ClosureBindings(ctx context.Context, r SSAClosureBindingsRequest) iter.Seq2[SSAArgumentBindingObservation, error] {
	return func(yield func(SSAArgumentBindingObservation, error) bool) {
		refuse := func(err error) { yield(SSAArgumentBindingObservation{}, err) }
		if err := errors.Join(temporal.Wait(temporal.WaitRequest{Context: ctx}), v.Validate(), r.Validate()); err != nil {
			refuse(err)
			return
		}
		if err := v.referrerInstructionMembership(r.Closure); err != nil {
			refuse(err)
			return
		}
		if r.Argument.Parent() != r.Closure.Parent() || !v.ownsStorageValue(r.Argument) {
			refuse(core.ErrGoToolchainContract)
			return
		}
		untaken, err := argumentCallUntakenBlock(r.Closure.Block())
		if err != nil {
			refuse(err)
			return
		}
		if untaken {
			return
		}
		if !nativeReceiverValuePresent(r.Closure.Fn) {
			refuse(core.ErrGoToolchainOutput)
			return
		}
		target, native := r.Closure.Fn.(*ssa.Function)
		if !native {
			refuse(core.ErrGoToolchainOutput)
			return
		}
		if target.Syntax() == nil {
			return
		}
		if target.Parent() != r.Closure.Parent() || target.Pkg != v.Package || !v.argumentBindingFunctionOwned(target) || len(r.Closure.Bindings) != len(target.FreeVars) {
			refuse(core.ErrGoToolchainOutput)
			return
		}
		for index, operand := range r.Closure.Bindings {
			if err := temporal.Wait(temporal.WaitRequest{Context: ctx}); err != nil {
				refuse(err)
				return
			}
			if err := v.argumentBindingOperand(ctx, operand, target.FreeVars[index], target, r.Closure); err != nil {
				refuse(err)
				return
			}
		}
		for index, operand := range r.Closure.Bindings {
			if err := temporal.Wait(temporal.WaitRequest{Context: ctx}); err != nil {
				refuse(err)
				return
			}
			matched, err := v.closureBindingBefore(ctx, operand, r.Argument, r.Closure)
			if err != nil {
				refuse(err)
				return
			}
			if matched && !yield(SSAArgumentBindingObservation{Function: target, Local: target.FreeVars[index]}, nil) {
				return
			}
		}
		if err := temporal.Wait(temporal.WaitRequest{Context: ctx}); err != nil {
			refuse(err)
		}
	}
}

// A native capture cell is an address. Its later initial store cannot prove
// the selected value was bound at construction. Matching already rejects
// replacement and opaque escape; Go's dominance supplies the remaining order.
func (v SSAUnitView) closureBindingBefore(ctx context.Context, operand, argument ssa.Value, before *ssa.MakeClosure) (bool, error) {
	matched, err := argumentCallOperand(operand, argument)
	if !matched || err != nil {
		return matched, err
	}
	cell, localCell := operand.(*ssa.Alloc)
	if !localCell {
		return true, nil
	}
	references := cell.Referrers()
	if references == nil {
		return false, nil
	}
	prior := false
	for _, reference := range *references {
		if err := temporal.Wait(temporal.WaitRequest{Context: ctx}); err != nil {
			return false, err
		}
		store, stored := reference.(*ssa.Store)
		if !stored || store.Addr != cell {
			continue
		}
		order, err := v.InstructionDominates(ctx, SSAInstructionOrderRequest{Before: store, After: before})
		if err != nil {
			return false, errors.Join(core.ErrGoToolchainOutput, err)
		}
		prior = prior || order.Proven
	}
	return prior, nil
}

var _ internalFlow = SSAClosureBindingsRequest{}
