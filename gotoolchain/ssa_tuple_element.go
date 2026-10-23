package gotoolchain

import (
	"context"
	"errors"
	"go/types"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
	"golang.org/x/tools/go/ssa"
)

// SSATupleElementRequest borrows one actual native result extraction.
type SSATupleElementRequest struct {
	Element *ssa.Extract
}

func (r SSATupleElementRequest) Validate() error {
	if r.Element == nil || r.Element.Block() == nil || r.Element.Block().Parent() == nil {
		return core.ErrGoToolchainContract
	}
	return nil
}
func (SSATupleElementRequest) goToolchainInternalFlow() {}

// SSATupleElementObservation lends the original native producer and selected
// result ordinal. Zero describes a known untaken extraction, without a result.
// The observation assigns no product meaning to an ordinal or producer kind.
type SSATupleElementObservation struct {
	Tuple ssa.Value
	Index int
}

func (o SSATupleElementObservation) Validate() error {
	if o.Tuple == nil && o.Index == 0 {
		return nil
	}
	if !nativeReceiverValuePresent(o.Tuple) {
		return core.ErrGoToolchainOutput
	}
	tuple, ok := o.Tuple.Type().Underlying().(*types.Tuple)
	if !ok || o.Index < 0 || o.Index >= tuple.Len() {
		return core.ErrGoToolchainOutput
	}
	return nil
}
func (SSATupleElementObservation) goToolchainInternalFlow() {}

// TupleElement authenticates both native instructions and their definition
// order through the existing Primitive relation query. Go owns result types,
// tuple layout and dominance. No instruction or result population is copied.
func (v SSAUnitView) TupleElement(ctx context.Context, r SSATupleElementRequest) (out SSATupleElementObservation, err error) {
	defer func() {
		if err == nil {
			err = temporal.Wait(temporal.WaitRequest{Context: ctx})
		}
		if err != nil {
			out = SSATupleElementObservation{}
		}
	}()
	if err = errors.Join(temporal.Wait(temporal.WaitRequest{Context: ctx}), v.Validate(), r.Validate()); err != nil {
		return out, err
	}
	if err = v.referrerInstructionMembership(r.Element); err != nil {
		return out, err
	}
	untaken, queryErr := argumentCallUntakenBlock(r.Element.Block())
	if queryErr != nil || untaken {
		return out, queryErr
	}
	producer, ok := r.Element.Tuple.(ssa.Instruction)
	if !ok || !nativeReceiverValuePresent(r.Element.Tuple) || producer.Block() == nil || producer.Parent() != r.Element.Parent() || producer == r.Element {
		return out, core.ErrGoToolchainOutput
	}
	tuple, ok := r.Element.Tuple.Type().Underlying().(*types.Tuple)
	if !ok || r.Element.Index < 0 || r.Element.Index >= tuple.Len() || r.Element.Type() == nil || !types.Identical(r.Element.Type(), tuple.At(r.Element.Index).Type()) {
		return out, core.ErrGoToolchainOutput
	}
	relation, queryErr := v.InstructionDominates(ctx, SSAInstructionOrderRequest{Before: producer, After: r.Element})
	if queryErr != nil {
		if errors.Is(queryErr, core.ErrGoToolchainContract) {
			return out, errors.Join(core.ErrGoToolchainOutput, queryErr)
		}
		return out, queryErr
	}
	if !relation.Proven {
		return out, core.ErrGoToolchainOutput
	}
	out = SSATupleElementObservation{Tuple: r.Element.Tuple, Index: r.Element.Index}
	return out, nil
}

var (
	_ internalFlow = SSATupleElementRequest{}
	_ internalFlow = SSATupleElementObservation{}
)
