package gotoolchain

import (
	"context"
	"errors"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
	"golang.org/x/tools/go/ssa"
)

// SSABlockDominanceRequest borrows two actual blocks in one native function.
// Go owns the dominator tree; this request carries no graph or execution history.
type SSABlockDominanceRequest struct {
	Dominator *ssa.BasicBlock
	Block     *ssa.BasicBlock
}

func (r SSABlockDominanceRequest) Validate() error {
	if r.Dominator == nil || r.Block == nil || r.Block.Parent() == nil || r.Dominator.Parent() != r.Block.Parent() {
		return core.ErrGoToolchainContract
	}
	return nil
}
func (SSABlockDominanceRequest) goToolchainInternalFlow() {}

// BlockDominates observes Go's structural dominance relation in constant
// scratch, without scanning blocks or instructions. Constant branch operands
// do not turn this relation into a claim about runtime reachability. A valid
// pair with no dominance produces Proven=false and no error.
func (v SSAUnitView) BlockDominates(ctx context.Context, r SSABlockDominanceRequest) (out SSARelationObservation, err error) {
	defer func() {
		if err == nil {
			err = temporal.Wait(temporal.WaitRequest{Context: ctx})
		}
		if err != nil {
			out = SSARelationObservation{}
		}
	}()
	if err = errors.Join(temporal.Wait(temporal.WaitRequest{Context: ctx}), v.Validate(), r.Validate()); err != nil {
		return out, err
	}
	fn := r.Block.Parent()
	if fn.Prog != v.Program || fn.Pkg != nil && v.Program.Package(fn.Pkg.Pkg) != fn.Pkg {
		return out, core.ErrGoToolchainContract
	}
	if !nativeBlockLinkMember(r.Dominator, fn) || !nativeBlockLinkMember(r.Block, fn) {
		return out, core.ErrGoToolchainOutput
	}
	out.Proven = r.Dominator.Dominates(r.Block)
	return out, nil
}

var _ internalFlow = SSABlockDominanceRequest{}
