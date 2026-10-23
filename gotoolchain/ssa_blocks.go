package gotoolchain

import (
	"context"
	"errors"
	"iter"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
	"golang.org/x/tools/go/ssa"
)

// SSABlockRequest selects the actual blocks of one native function. Imported
// declarations without a body have no blocks; this says nothing about execution.
// DominatedBy optionally narrows observations to one native dominator region.
// A nil selector includes all blocks. No region or graph is materialized.
type SSABlockRequest struct {
	Function    *ssa.Function
	DominatedBy *ssa.BasicBlock
}

func (r SSABlockRequest) Validate() error {
	if r.Function == nil {
		return core.ErrGoToolchainContract
	}
	if r.DominatedBy != nil && r.DominatedBy.Parent() != r.Function {
		return core.ErrGoToolchainContract
	}
	return nil
}
func (SSABlockRequest) goToolchainInternalFlow() {}

// Blocks lends Go's existing blocks in ordinal order, excluding known untaken
// blocks. It validates block custody and retains no graph, block population or
// traversal state. Instructions and edge relations remain native Go objects.
func (v SSAUnitView) Blocks(ctx context.Context, r SSABlockRequest) iter.Seq2[*ssa.BasicBlock, error] {
	return func(yield func(*ssa.BasicBlock, error) bool) {
		if err := errors.Join(temporal.Wait(temporal.WaitRequest{Context: ctx}), v.Validate(), r.Validate()); err != nil {
			yield(nil, err)
			return
		}
		fn := r.Function
		if fn.Prog != v.Program {
			yield(nil, core.ErrGoToolchainContract)
			return
		}
		if pkg := fn.Pkg; pkg != nil && v.Program.Package(pkg.Pkg) != pkg {
			yield(nil, core.ErrGoToolchainContract)
			return
		}
		if block := r.DominatedBy; block != nil && (block.Index < 0 || block.Index >= len(fn.Blocks) || fn.Blocks[block.Index] != block) {
			yield(nil, core.ErrGoToolchainOutput)
			return
		}
		for index, block := range fn.Blocks {
			if err := temporal.Wait(temporal.WaitRequest{Context: ctx}); err != nil {
				yield(nil, err)
				return
			}
			if block == nil || block.Parent() != fn || block.Index != index {
				yield(nil, core.ErrGoToolchainOutput)
				return
			}
			if r.DominatedBy != nil && !r.DominatedBy.Dominates(block) {
				continue
			}
			untaken, queryErr := argumentCallUntakenBlock(block)
			if queryErr != nil {
				yield(nil, queryErr)
				return
			}
			if untaken {
				continue
			}
			if !yield(block, nil) {
				return
			}
		}
		if err := temporal.Wait(temporal.WaitRequest{Context: ctx}); err != nil {
			yield(nil, err)
		}
	}
}

var _ internalFlow = SSABlockRequest{}
