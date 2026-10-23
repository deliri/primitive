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

// SSAReferrerRequest selects one actual native instruction that produces a
// value. Go owns its use list; no reference index is constructed or copied.
type SSAReferrerRequest struct{ Instruction ssa.Instruction }

func (r SSAReferrerRequest) Validate() error {
	if r.Instruction == nil || reflect.ValueOf(r.Instruction).IsNil() {
		return core.ErrGoToolchainContract
	}
	if _, value := r.Instruction.(ssa.Value); !value {
		return core.ErrGoToolchainContract
	}
	return nil
}
func (SSAReferrerRequest) goToolchainInternalFlow() {}

// Referrers streams Go's existing uses in their native order, excluding known
// untaken instructions. Nil use lists are ordinary absence. Each yielded use
// belongs to the same native function as the selected value instruction.
func (v SSAUnitView) Referrers(ctx context.Context, r SSAReferrerRequest) iter.Seq2[ssa.Instruction, error] {
	return func(yield func(ssa.Instruction, error) bool) {
		if err := errors.Join(temporal.Wait(temporal.WaitRequest{Context: ctx}), v.Validate(), r.Validate()); err != nil {
			yield(nil, err)
			return
		}
		if err := v.referrerInstructionMembership(r.Instruction); err != nil {
			yield(nil, err)
			return
		}
		value := r.Instruction.(ssa.Value)
		if references := value.Referrers(); references != nil {
			for _, instruction := range *references {
				if err := temporal.Wait(temporal.WaitRequest{Context: ctx}); err != nil {
					yield(nil, err)
					return
				}
				if err := v.referrerInstructionMembership(instruction); err != nil {
					yield(nil, errors.Join(core.ErrGoToolchainOutput, err))
					return
				}
				if instruction.Parent() != r.Instruction.Parent() {
					yield(nil, core.ErrGoToolchainOutput)
					return
				}
				untaken, queryErr := argumentCallUntakenBlock(instruction.Block())
				if queryErr != nil {
					yield(nil, queryErr)
					return
				}
				if untaken {
					continue
				}
				if !yield(instruction, nil) {
					return
				}
			}
		}
		if err := temporal.Wait(temporal.WaitRequest{Context: ctx}); err != nil {
			yield(nil, err)
		}
	}
}

// Membership uses native block identity and ordinal, then compares existing
// instruction pointers. It retains no instruction population or lookup table.
func (v SSAUnitView) referrerInstructionMembership(instruction ssa.Instruction) error {
	if instruction == nil || reflect.ValueOf(instruction).IsNil() {
		return core.ErrGoToolchainContract
	}
	fn, block := instruction.Parent(), instruction.Block()
	if fn == nil || fn.Prog != v.Program || fn.Pkg != v.Package || block == nil || block.Parent() != fn {
		return core.ErrGoToolchainContract
	}
	if block.Index < 0 || block.Index >= len(fn.Blocks) || fn.Blocks[block.Index] != block {
		return core.ErrGoToolchainOutput
	}
	for _, candidate := range block.Instrs {
		if candidate == instruction {
			return nil
		}
	}
	return core.ErrGoToolchainContract
}

var _ internalFlow = SSAReferrerRequest{}
