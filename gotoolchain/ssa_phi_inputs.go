package gotoolchain

import (
	"context"
	"errors"
	"iter"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
	"golang.org/x/tools/go/ssa"
)

// SSAPhiInputsRequest selects one native merge. AncestorOf, when present,
// restricts the merge to a strict native dominator ancestor of that block.
// This permits source-input traversal without retaining a visited population.
type SSAPhiInputsRequest struct {
	Function   *ssa.Function
	Merge      *ssa.Phi
	AncestorOf *ssa.BasicBlock
}

func (r SSAPhiInputsRequest) Validate() error {
	if r.Function == nil || r.Merge == nil || r.Merge.Parent() != r.Function || r.AncestorOf != nil && r.AncestorOf.Parent() != r.Function {
		return core.ErrGoToolchainContract
	}
	return nil
}
func (SSAPhiInputsRequest) goToolchainInternalFlow() {}

// SSAPhiInputObservation lends one non-loop-carried input and its actual merge
// block. Inputs retain native predecessor order and multiplicity. No input or
// predecessor population is copied. Absence is represented by an empty stream.
type SSAPhiInputObservation struct {
	Value ssa.Value
	Block *ssa.BasicBlock
}

func (o SSAPhiInputObservation) Validate() error {
	if !nativeReceiverValuePresent(o.Value) || o.Block == nil || o.Block.Parent() == nil || o.Value.Parent() != nil && o.Value.Parent() != o.Block.Parent() {
		return core.ErrGoToolchainOutput
	}
	return nil
}
func (SSAPhiInputObservation) goToolchainInternalFlow() {}

// PhiInputs authenticates the merge and complete incoming layout before lending
// any value. Go owns instruction order and dominance. Incoming loop backedges
// remain validated but do not become source-input observations.
func (v SSAUnitView) PhiInputs(ctx context.Context, r SSAPhiInputsRequest) iter.Seq2[SSAPhiInputObservation, error] {
	return func(yield func(SSAPhiInputObservation, error) bool) {
		refuse := func(err error) { yield(SSAPhiInputObservation{}, err) }
		if err := errors.Join(temporal.Wait(temporal.WaitRequest{Context: ctx}), v.Validate(), r.Validate()); err != nil {
			refuse(err)
			return
		}
		if r.Function.Prog != v.Program || r.Function.Pkg != v.Package {
			refuse(core.ErrGoToolchainContract)
			return
		}
		if err := v.referrerInstructionMembership(r.Merge); err != nil {
			refuse(err)
			return
		}
		block := r.Merge.Block()
		untaken, err := argumentCallUntakenBlock(block)
		if err != nil {
			refuse(err)
			return
		}
		if untaken {
			return
		}
		if r.AncestorOf != nil {
			relation, err := v.BlockDominates(ctx, SSABlockDominanceRequest{Dominator: block, Block: r.AncestorOf})
			if err != nil {
				refuse(err)
				return
			}
			if block == r.AncestorOf || !relation.Proven {
				return
			}
		}
		if len(r.Merge.Edges) == 0 || len(r.Merge.Edges) != len(block.Preds) {
			refuse(core.ErrGoToolchainOutput)
			return
		}
		index := 0
		for predecessor, err := range v.BlockLinks(ctx, SSABlockLinkRequest{Block: block, Kind: SSABlockLinkPredecessors}) {
			if err != nil {
				refuse(err)
				return
			}
			linked := false
			for _, successor := range predecessor.Succs {
				linked = linked || successor == block
			}
			if !linked {
				refuse(core.ErrGoToolchainOutput)
				return
			}
			if err := v.phiInputBefore(ctx, r.Function, predecessor, r.Merge.Edges[index]); err != nil {
				refuse(err)
				return
			}
			index++
		}
		index = 0
		for predecessor, err := range v.BlockLinks(ctx, SSABlockLinkRequest{Block: block, Kind: SSABlockLinkPredecessors}) {
			if err != nil {
				refuse(err)
				return
			}
			value := r.Merge.Edges[index]
			index++
			relation, err := v.BlockDominates(ctx, SSABlockDominanceRequest{Dominator: block, Block: predecessor})
			if err != nil {
				refuse(err)
				return
			}
			if relation.Proven {
				continue
			}
			if !yield(SSAPhiInputObservation{Value: value, Block: block}, nil) {
				return
			}
		}
		if err := temporal.Wait(temporal.WaitRequest{Context: ctx}); err != nil {
			refuse(err)
		}
	}
}

func (v SSAUnitView) phiInputBefore(ctx context.Context, fn *ssa.Function, predecessor *ssa.BasicBlock, value ssa.Value) error {
	found := false
	for terminator, err := range v.Instructions(ctx, SSAInstructionRequest{Function: fn, Block: predecessor, Extent: SSAInstructionExtentBlockTerminator}) {
		if err != nil {
			return err
		}
		found = true
		if function, ok := value.(*ssa.Function); ok {
			if !v.argumentBindingFunctionOwned(function) {
				return core.ErrGoToolchainOutput
			}
		} else if err := v.fieldReadOperandBefore(ctx, value, terminator); err != nil {
			return err
		}
	}
	if !found {
		return core.ErrGoToolchainOutput
	}
	return nil
}

var _ internalFlow = SSAPhiInputsRequest{}
var _ internalFlow = SSAPhiInputObservation{}
