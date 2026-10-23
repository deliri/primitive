package gotoolchain

import (
	"context"
	"errors"
	"iter"
	"reflect"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
	"golang.org/x/tools/go/ssa"
)

// SSAInstructionExtent selects a native block, its terminator, a function or lexical descendants.
// Lexical selection does not assert runtime invocation.
type SSAInstructionExtent uint8

const (
	SSAInstructionExtentUnknown SSAInstructionExtent = iota
	SSAInstructionExtentFunction
	SSAInstructionExtentLexical
	SSAInstructionExtentBlock
	SSAInstructionExtentBlockTerminator
)

func (e SSAInstructionExtent) Validate() error {
	if !e.IsValid() {
		return core.ErrGoToolchainContract
	}
	return nil
}
func (e SSAInstructionExtent) IsValid() bool {
	return e == SSAInstructionExtentFunction || e == SSAInstructionExtentLexical || e == SSAInstructionExtentBlock || e == SSAInstructionExtentBlockTerminator
}
func (SSAInstructionExtent) OffWireEnum() {}
func (e SSAInstructionExtent) String() string {
	switch e {
	case SSAInstructionExtentFunction:
		return "function"
	case SSAInstructionExtentLexical:
		return "lexical"
	case SSAInstructionExtentBlock:
		return "block"
	case SSAInstructionExtentBlockTerminator:
		return "block_terminator"
	default:
		return core.UnknownEnumDiagnostic
	}
}

type SSAInstructionRequest struct {
	Function *ssa.Function
	Extent   SSAInstructionExtent
	Block    *ssa.BasicBlock
}

func (r SSAInstructionRequest) Validate() error {
	if r.Function == nil {
		return core.ErrGoToolchainContract
	}
	selectedBlock := r.Extent == SSAInstructionExtentBlock || r.Extent == SSAInstructionExtentBlockTerminator
	if selectedBlock != (r.Block != nil) {
		return core.ErrGoToolchainContract
	}
	if r.Block != nil && r.Block.Parent() != r.Function {
		return core.ErrGoToolchainContract
	}
	return r.Extent.Validate()
}
func (SSAInstructionRequest) goToolchainInternalFlow() {}

// Instructions lends native instructions in block order, excluding Go's known
// untaken blocks. Lexical descendants follow their parent. The stream retains
// only active lexical ancestry, with no instruction or function population.
// Imported declarations without source produce no observations. A selected
// terminator borrows only the block's final control instruction; preceding
// instructions remain outside that query, without a scan or copied slice.
func (v SSAUnitView) Instructions(ctx context.Context, r SSAInstructionRequest) iter.Seq2[ssa.Instruction, error] {
	return func(yield func(ssa.Instruction, error) bool) {
		if err := errors.Join(temporal.Wait(temporal.WaitRequest{Context: ctx}), v.Validate(), r.Validate()); err != nil {
			yield(nil, err)
			return
		}
		if !v.instructionsInFunction(ctx, r, nil, yield) {
			return
		}
		if err := temporal.Wait(temporal.WaitRequest{Context: ctx}); err != nil {
			yield(nil, err)
		}
	}
}

// One native callback carries consumer authority through lexical recursion.
// No forwarding iterator or per-function query closure is allocated.
func (v SSAUnitView) instructionsInFunction(ctx context.Context, r SSAInstructionRequest, parent *ssa.Function, yield func(ssa.Instruction, error) bool) bool {
	fn := r.Function
	if fn == nil || fn.Prog != v.Program || (parent != nil && fn.Parent() != parent) {
		yield(nil, core.ErrGoToolchainContract)
		return false
	}
	if pkg := fn.Pkg; pkg != nil && v.Program.Package(pkg.Pkg) != pkg {
		yield(nil, core.ErrGoToolchainContract)
		return false
	}
	blocks, first := fn.Blocks, 0
	if r.Extent == SSAInstructionExtentBlock || r.Extent == SSAInstructionExtentBlockTerminator {
		first = r.Block.Index
		if first < 0 || first >= len(blocks) || blocks[first] != r.Block {
			yield(nil, core.ErrGoToolchainOutput)
			return false
		}
		// Borrow one native slice window; do not rescan or copy the function.
		blocks = blocks[first : first+1]
	}
	for offset, block := range blocks {
		if err := temporal.Wait(temporal.WaitRequest{Context: ctx}); err != nil {
			yield(nil, err)
			return false
		}
		if block == nil || block.Parent() != fn || block.Index != first+offset {
			yield(nil, core.ErrGoToolchainOutput)
			return false
		}
		untaken, queryErr := argumentCallUntakenBlock(block)
		if queryErr != nil {
			yield(nil, queryErr)
			return false
		}
		if untaken {
			continue
		}
		instructions := block.Instrs
		if r.Extent == SSAInstructionExtentBlockTerminator && len(instructions) != 0 {
			instructions = instructions[len(instructions)-1:]
		}
		for _, instruction := range instructions {
			if err := temporal.Wait(temporal.WaitRequest{Context: ctx}); err != nil {
				yield(nil, err)
				return false
			}
			if instruction == nil || reflect.ValueOf(instruction).IsNil() || instruction.Parent() != fn || instruction.Block() != block {
				yield(nil, core.ErrGoToolchainOutput)
				return false
			}
			if r.Extent == SSAInstructionExtentBlockTerminator {
				switch instruction.(type) {
				case *ssa.If, *ssa.Jump, *ssa.Return, *ssa.Panic:
				default:
					yield(nil, core.ErrGoToolchainOutput)
					return false
				}
			}
			if !yield(instruction, nil) {
				return false
			}
		}
	}
	if r.Extent == SSAInstructionExtentLexical {
		for _, child := range fn.AnonFuncs {
			if !v.instructionsInFunction(ctx, SSAInstructionRequest{Function: child, Extent: r.Extent}, fn, yield) {
				return false
			}
		}
	}
	return true
}

var _ internalFlow = SSAInstructionRequest{}
