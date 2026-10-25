package gotoolchain

import (
	"context"
	"errors"
	"iter"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
	"golang.org/x/tools/go/ssa"
)

// SSACaptureFramesRequest selects a native parameter or free variable in its
// owning source function. It requests lexical capture custody, not invocation.
type SSACaptureFramesRequest struct {
	Function *ssa.Function
	Argument ssa.Value
}

func (r SSACaptureFramesRequest) Validate() error {
	if r.Function == nil || !nativeReceiverValuePresent(r.Argument) {
		return core.ErrGoToolchainContract
	}
	switch r.Argument.(type) {
	case *ssa.Parameter, *ssa.FreeVar:
		return nil
	default:
		return core.ErrGoToolchainContract
	}
}
func (SSACaptureFramesRequest) goToolchainInternalFlow() {}

// CaptureFrames first lends the selected frame, then its exact native captures
// in construction order, depth first. Each construction delegates operand and
// ordering authority to ClosureBindings. Uninvoked captures remain captures;
// callback parameters and unrelated values never acquire this selected value.
// The stream retains active native ancestry, with no environment, function or
// instruction population. This is not a constant-memory claim across depth.
func (v SSAUnitView) CaptureFrames(ctx context.Context, r SSACaptureFramesRequest) iter.Seq2[SSAArgumentBindingObservation, error] {
	return func(yield func(SSAArgumentBindingObservation, error) bool) {
		if err := errors.Join(temporal.Wait(temporal.WaitRequest{Context: ctx}), v.Validate(), r.Validate()); err != nil {
			yield(SSAArgumentBindingObservation{}, err)
			return
		}
		if r.Function.Prog != v.Program || r.Function.Pkg != v.Package || !v.argumentBindingFunctionOwned(r.Function) || r.Argument.Parent() != r.Function || !v.ownsStorageValue(r.Argument) {
			yield(SSAArgumentBindingObservation{}, core.ErrGoToolchainContract)
			return
		}
		if !v.captureFramesInFunction(ctx, SSAArgumentBindingObservation{Function: r.Function, Local: r.Argument}, yield) {
			return
		}
		if err := temporal.Wait(temporal.WaitRequest{Context: ctx}); err != nil {
			yield(SSAArgumentBindingObservation{}, err)
		}
	}
}

func (v SSAUnitView) captureFramesInFunction(ctx context.Context, frame SSAArgumentBindingObservation, yield func(SSAArgumentBindingObservation, error) bool) bool {
	if !yield(frame, nil) {
		return false
	}
	for instruction, err := range v.Instructions(ctx, SSAInstructionRequest{Function: frame.Function, Extent: SSAInstructionExtentFunction}) {
		if err != nil {
			yield(SSAArgumentBindingObservation{}, err)
			return false
		}
		closure, ok := instruction.(*ssa.MakeClosure)
		if !ok {
			continue
		}
		for binding, err := range v.ClosureBindings(ctx, SSAClosureBindingsRequest{Closure: closure, Argument: frame.Local}) {
			if err != nil {
				yield(SSAArgumentBindingObservation{}, err)
				return false
			}
			if !v.captureFramesInFunction(ctx, binding, yield) {
				return false
			}
		}
	}
	return true
}

var _ internalFlow = SSACaptureFramesRequest{}
