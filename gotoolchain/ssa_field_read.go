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

// SSAFieldReadRequest selects one actual native unary instruction. A valid
// instruction that is not a dereference of a field address is neutral.
type SSAFieldReadRequest struct{ Read *ssa.UnOp }

func (r SSAFieldReadRequest) Validate() error {
	if r.Read == nil || r.Read.Block() == nil || r.Read.Block().Parent() == nil {
		return core.ErrGoToolchainContract
	}
	return nil
}
func (SSAFieldReadRequest) goToolchainInternalFlow() {}

// SSAFieldReadObservation lends the actual base value and Go record field.
// It asserts neither the base's runtime value nor successful runtime execution.
type SSAFieldReadObservation struct {
	Base  ssa.Value
	Field *types.Var
}

func (o SSAFieldReadObservation) Validate() error {
	if o.Base == nil && o.Field == nil {
		return nil
	}
	if !nativeReceiverValuePresent(o.Base) || o.Field == nil || !o.Field.IsField() || o.Field.Type() == nil {
		return core.ErrGoToolchainOutput
	}
	return nil
}
func (SSAFieldReadObservation) goToolchainInternalFlow() {}

// FieldRead authenticates the selected read, address, base, ordinal, types and
// native operand order. An untaken read is neutral. It borrows compiler objects
// with constant scratch and preserves cancellation causes without field policy.
func (v SSAUnitView) FieldRead(ctx context.Context, r SSAFieldReadRequest) (out SSAFieldReadObservation, err error) {
	defer func() {
		if err == nil {
			err = temporal.Wait(temporal.WaitRequest{Context: ctx})
		}
		if err != nil {
			out = SSAFieldReadObservation{}
		}
	}()
	if err = errors.Join(temporal.Wait(temporal.WaitRequest{Context: ctx}), v.Validate(), r.Validate()); err != nil {
		return out, err
	}
	if err = v.referrerInstructionMembership(r.Read); err != nil {
		return out, err
	}
	untaken, queryErr := argumentCallUntakenBlock(r.Read.Block())
	if queryErr != nil {
		return out, queryErr
	}
	if untaken || r.Read.Op != token.MUL {
		return out, nil
	}
	if r.Read.CommaOk {
		return out, core.ErrGoToolchainOutput
	}
	if err = v.fieldReadOperandBefore(ctx, r.Read.X, r.Read); err != nil {
		return out, err
	}
	address, field := r.Read.X.(*ssa.FieldAddr)
	if !field {
		return out, nil
	}
	if err = v.fieldReadOperandBefore(ctx, address.X, address); err != nil {
		return out, err
	}
	pointer, ok := types.Unalias(address.X.Type()).Underlying().(*types.Pointer)
	if !ok {
		return out, core.ErrGoToolchainOutput
	}
	record, ok := types.Unalias(pointer.Elem()).Underlying().(*types.Struct)
	if !ok || address.Field < 0 || address.Field >= record.NumFields() {
		return out, core.ErrGoToolchainOutput
	}
	member := record.Field(address.Field)
	addressType, ok := types.Unalias(address.Type()).Underlying().(*types.Pointer)
	if !ok || !types.Identical(addressType.Elem(), member.Type()) || !types.Identical(r.Read.Type(), member.Type()) {
		return out, core.ErrGoToolchainOutput
	}
	out = SSAFieldReadObservation{Base: address.X, Field: member}
	return out, nil
}

func (v SSAUnitView) fieldReadOperandBefore(ctx context.Context, value ssa.Value, after ssa.Instruction) error {
	if !nativeReceiverValuePresent(value) || !v.ownsStorageValue(value) || value.Parent() != nil && value.Parent() != after.Parent() {
		return core.ErrGoToolchainOutput
	}
	if producer, instruction := value.(ssa.Instruction); instruction {
		if producer == after {
			return core.ErrGoToolchainOutput
		}
		order, queryErr := v.InstructionDominates(ctx, SSAInstructionOrderRequest{Before: producer, After: after})
		if queryErr != nil {
			return errors.Join(core.ErrGoToolchainOutput, queryErr)
		}
		if !order.Proven {
			return core.ErrGoToolchainOutput
		}
	}
	return nil
}

var (
	_ internalFlow = SSAFieldReadRequest{}
	_ internalFlow = SSAFieldReadObservation{}
)
