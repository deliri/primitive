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
type SSABlockRequest struct{ Function *ssa.Function }

func (r SSABlockRequest) Validate() error {
	if r.Function == nil {
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
		for index, block := range fn.Blocks {
			if err := temporal.Wait(temporal.WaitRequest{Context: ctx}); err != nil {
				yield(nil, err)
				return
			}
			if block == nil || block.Parent() != fn || block.Index != index {
				yield(nil, core.ErrGoToolchainOutput)
				return
			}
			if argumentCallUntakenBlock(block) {
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
