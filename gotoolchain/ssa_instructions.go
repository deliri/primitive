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

// SSAInstructionExtent selects one native function or that function and its
// lexical descendants. Lexical selection does not assert runtime invocation.
type SSAInstructionExtent uint8

const (
	SSAInstructionExtentUnknown SSAInstructionExtent = iota
	SSAInstructionExtentFunction
	SSAInstructionExtentLexical
)

func (e SSAInstructionExtent) Validate() error {
	if !e.IsValid() {
		return core.ErrGoToolchainContract
	}
	return nil
}
func (e SSAInstructionExtent) IsValid() bool {
	return e == SSAInstructionExtentFunction || e == SSAInstructionExtentLexical
}
func (SSAInstructionExtent) OffWireEnum() {}
func (e SSAInstructionExtent) String() string {
	switch e {
	case SSAInstructionExtentFunction:
		return "function"
	case SSAInstructionExtentLexical:
		return "lexical"
	default:
		return core.UnknownEnumDiagnostic
	}
}

type SSAInstructionRequest struct {
	Function *ssa.Function
	Extent   SSAInstructionExtent
}

func (r SSAInstructionRequest) Validate() error {
	if r.Function == nil {
		return core.ErrGoToolchainContract
	}
	return r.Extent.Validate()
}
func (SSAInstructionRequest) goToolchainInternalFlow() {}

// Instructions lends native instructions in block order, excluding Go's known
// untaken blocks. Lexical descendants follow their parent. The stream retains
// only active lexical ancestry, with no instruction or function population.
// Imported declarations without source produce no observations.
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
	for index, block := range fn.Blocks {
		if err := temporal.Wait(temporal.WaitRequest{Context: ctx}); err != nil {
			yield(nil, err)
			return false
		}
		if block == nil || block.Parent() != fn || block.Index != index {
			yield(nil, core.ErrGoToolchainOutput)
			return false
		}
		if argumentCallUntakenBlock(block) {
			continue
		}
		for _, instruction := range block.Instrs {
			if err := temporal.Wait(temporal.WaitRequest{Context: ctx}); err != nil {
				yield(nil, err)
				return false
			}
			if instruction == nil || reflect.ValueOf(instruction).IsNil() || instruction.Parent() != fn || instruction.Block() != block {
				yield(nil, core.ErrGoToolchainOutput)
				return false
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
