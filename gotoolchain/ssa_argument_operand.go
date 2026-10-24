package gotoolchain

import (
	"context"
	"errors"
	"go/token"
	"go/types"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
	"golang.org/x/tools/go/ssa"
)

// SSAArgumentOperandRequest borrows operands from one native frame. Path is
// the field portion of Go's Selection.Index, excluding its final method slot.
// An empty path compares the argument itself, including explicit native
// representations and capture-cell reads. No runtime alias claim is made.
type SSAArgumentOperandRequest struct {
	Function *ssa.Function
	Value    ssa.Value
	Argument ssa.Value
	Path     []int
}

func (r SSAArgumentOperandRequest) Validate() error {
	if r.Function == nil || !nativeReceiverValuePresent(r.Value) || !nativeReceiverValuePresent(r.Argument) {
		return core.ErrGoToolchainContract
	}
	for _, index := range r.Path {
		if index < 0 {
			return core.ErrGoToolchainContract
		}
	}
	return nil
}
func (SSAArgumentOperandRequest) goToolchainInternalFlow() {}

// ArgumentOperand observes the same native operand relation used by
// ArgumentCall and ReceiverParameter. Later stores do not replace an already
// loaded value; a prior possible replacement supplies no identity proof.
// Input population is never retained or copied.
func (v SSAUnitView) ArgumentOperand(ctx context.Context, r SSAArgumentOperandRequest) (out SSARelationObservation, err error) {
	defer func() {
		err = errors.Join(err, temporal.Wait(temporal.WaitRequest{Context: ctx}))
		if err != nil {
			out = SSARelationObservation{}
		}
	}()
	if err = errors.Join(temporal.Wait(temporal.WaitRequest{Context: ctx}), v.Validate(), r.Validate()); err != nil {
		return out, err
	}
	if r.Function.Prog != v.Program || r.Function.Pkg != v.Package ||
		!v.ownsStorageValue(r.Argument) || r.Argument.Parent() != r.Function ||
		!v.ownsStorageValue(r.Value) || r.Value.Parent() != nil && r.Value.Parent() != r.Function {
		return out, core.ErrGoToolchainContract
	}
	// Authenticate each borrowed representation before the shared matcher reads
	// it. Native instruction ordering also refuses forged operand cycles.
representations:
	for value := r.Value; ; {
		if err = temporal.Wait(temporal.WaitRequest{Context: ctx}); err != nil {
			return out, err
		}
		switch value.(type) {
		case *ssa.Alloc, *ssa.FreeVar:
			if err = v.argumentOperandCellCustody(ctx, value); err != nil {
				return out, err
			}
		}
		instruction, native := value.(ssa.Instruction)
		if !native {
			break
		}
		untaken, queryErr := argumentCallUntakenBlock(instruction.Block())
		if queryErr != nil || untaken {
			return out, queryErr
		}
		var operand ssa.Value
		switch operation := value.(type) {
		case *ssa.MakeInterface:
			operand = operation.X
		case *ssa.ChangeInterface:
			operand = operation.X
		case *ssa.ChangeType:
			operand = operation.X
		case *ssa.UnOp:
			if operation.Op != token.MUL {
				break representations
			}
			operand = operation.X
		case *ssa.FieldAddr:
			if !nativeReceiverValuePresent(operation.X) {
				return out, core.ErrGoToolchainOutput
			}
			member := nativeSSAField(operation.X.Type(), operation.Field, true)
			pointer, ok := types.Unalias(operation.Type()).Underlying().(*types.Pointer)
			if member == nil || !ok || !types.Identical(pointer.Elem(), member.Type()) {
				return out, core.ErrGoToolchainOutput
			}
			operand = operation.X
		case *ssa.Field:
			if !nativeReceiverValuePresent(operation.X) {
				return out, core.ErrGoToolchainOutput
			}
			member := nativeSSAField(operation.X.Type(), operation.Field, false)
			if member == nil || !types.Identical(operation.Type(), member.Type()) {
				return out, core.ErrGoToolchainOutput
			}
			operand = operation.X
		default:
			break representations
		}
		if err = v.fieldReadOperandBefore(ctx, operand, instruction); err != nil {
			return out, err
		}
		value = operand
	}
	out.Proven, err = argumentCallReceiver(r.Value, r.Argument, r.Path)
	return out, err
}

// The shared matcher observes stores through Go's use list. Authenticate the
// same borrowed stores before allowing that list to lend identity.
func (v SSAUnitView) argumentOperandCellCustody(ctx context.Context, cell ssa.Value) error {
	if references := cell.Referrers(); references != nil {
		for _, reference := range *references {
			if err := temporal.Wait(temporal.WaitRequest{Context: ctx}); err != nil {
				return err
			}
			if err := v.referrerInstructionMembership(reference); err != nil {
				return errors.Join(core.ErrGoToolchainOutput, err)
			}
			if reference.Parent() != cell.Parent() {
				return core.ErrGoToolchainOutput
			}
			if store, ok := reference.(*ssa.Store); ok {
				if store.Addr != cell && store.Val != cell {
					return core.ErrGoToolchainOutput
				}
				if err := v.fieldReadOperandBefore(ctx, store.Addr, store); err != nil {
					return err
				}
				if err := v.fieldReadOperandBefore(ctx, store.Val, store); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

var _ internalFlow = SSAArgumentOperandRequest{}
