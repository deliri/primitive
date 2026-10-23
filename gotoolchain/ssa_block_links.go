package gotoolchain

import (
	"context"
	"errors"
	"iter"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
	"golang.org/x/tools/go/ssa"
)

// SSABlockLinkKind selects existing Go edges, without constructing a graph.
type SSABlockLinkKind uint8

const (
	SSABlockLinkUnknown SSABlockLinkKind = iota
	SSABlockLinkSuccessors
	SSABlockLinkDominatorAncestry
	SSABlockLinkPredecessors
)

func (k SSABlockLinkKind) IsValid() bool {
	return k == SSABlockLinkSuccessors || k == SSABlockLinkDominatorAncestry || k == SSABlockLinkPredecessors
}
func (k SSABlockLinkKind) Validate() error {
	if !k.IsValid() {
		return core.ErrGoToolchainContract
	}
	return nil
}
func (SSABlockLinkKind) OffWireEnum() {}
func (k SSABlockLinkKind) String() string {
	switch k {
	case SSABlockLinkSuccessors:
		return "successors"
	case SSABlockLinkDominatorAncestry:
		return "dominator_ancestry"
	case SSABlockLinkPredecessors:
		return "predecessors"
	default:
		return core.UnknownEnumDiagnostic
	}
}

type SSABlockLinkRequest struct {
	Block *ssa.BasicBlock
	Kind  SSABlockLinkKind
}

func (r SSABlockLinkRequest) Validate() error {
	if r.Block == nil || r.Block.Parent() == nil {
		return core.ErrGoToolchainContract
	}
	return r.Kind.Validate()
}
func (SSABlockLinkRequest) goToolchainInternalFlow() {}

// BlockLinks lends actual same-function native blocks. Successors retain Go's
// edge order, excluding known untaken blocks. Dominator ancestry begins with
// the selected block and follows Idom to the root. A known untaken selected
// block has no observations. Scratch is constant; no graph or path is retained.
// Predecessors retain Go's structural incoming-edge order and multiplicity,
// including incoming edges whose source is known untaken. This is a structural
// projection, not an assertion that every incoming edge executes at runtime.
func (v SSAUnitView) BlockLinks(ctx context.Context, r SSABlockLinkRequest) iter.Seq2[*ssa.BasicBlock, error] {
	return func(yield func(*ssa.BasicBlock, error) bool) {
		if err := errors.Join(temporal.Wait(temporal.WaitRequest{Context: ctx}), v.Validate(), r.Validate()); err != nil {
			yield(nil, err)
			return
		}
		fn := r.Block.Parent()
		if fn.Prog != v.Program || fn.Pkg != nil && v.Program.Package(fn.Pkg.Pkg) != fn.Pkg {
			yield(nil, core.ErrGoToolchainContract)
			return
		}
		if !nativeBlockLinkMember(r.Block, fn) {
			yield(nil, core.ErrGoToolchainOutput)
			return
		}
		untaken, queryErr := argumentCallUntakenBlock(r.Block)
		if queryErr != nil {
			yield(nil, queryErr)
			return
		}
		if untaken {
			if err := temporal.Wait(temporal.WaitRequest{Context: ctx}); err != nil {
				yield(nil, err)
			}
			return
		}
		switch r.Kind {
		case SSABlockLinkSuccessors:
			for _, successor := range r.Block.Succs {
				if !nativeBlockLinkYield(ctx, fn, successor, true, yield) {
					return
				}
			}
		case SSABlockLinkDominatorAncestry:
			for block := r.Block; block != nil; block = block.Idom() {
				if !nativeBlockLinkYield(ctx, fn, block, false, yield) {
					return
				}
			}
		case SSABlockLinkPredecessors:
			for _, predecessor := range r.Block.Preds {
				if !nativeBlockLinkYield(ctx, fn, predecessor, false, yield) {
					return
				}
			}
		default:
			yield(nil, core.ErrGoToolchainContract)
			return
		}
		if err := temporal.Wait(temporal.WaitRequest{Context: ctx}); err != nil {
			yield(nil, err)
		}
	}
}

func nativeBlockLinkMember(block *ssa.BasicBlock, fn *ssa.Function) bool {
	return block != nil && block.Parent() == fn && block.Index >= 0 && block.Index < len(fn.Blocks) && fn.Blocks[block.Index] == block
}

func nativeBlockLinkYield(ctx context.Context, fn *ssa.Function, block *ssa.BasicBlock, excludeUntaken bool, yield func(*ssa.BasicBlock, error) bool) bool {
	if err := temporal.Wait(temporal.WaitRequest{Context: ctx}); err != nil {
		yield(nil, err)
		return false
	}
	if !nativeBlockLinkMember(block, fn) {
		yield(nil, core.ErrGoToolchainOutput)
		return false
	}
	if excludeUntaken {
		untaken, queryErr := argumentCallUntakenBlock(block)
		if queryErr != nil {
			yield(nil, queryErr)
			return false
		}
		if untaken {
			return true
		}
	}
	return yield(block, nil)
}

var _ internalFlow = SSABlockLinkRequest{}
var _ core.OffWireEnum = SSABlockLinkUnknown
