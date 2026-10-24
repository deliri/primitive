package gotoolchain

import (
	"context"
	"errors"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
	"golang.org/x/tools/go/ssa"
)

// SSAConversionOperandRequest selects one native value conversion. Other unary
// operations are outside this contract; the compiler owns conversion semantics.
type SSAConversionOperandRequest struct {
	Function   *ssa.Function
	Conversion ssa.Value
}

func (r SSAConversionOperandRequest) Validate() error {
	if r.Function == nil {
		return core.ErrGoToolchainContract
	}
	switch converted := r.Conversion.(type) {
	case *ssa.ChangeType:
		if converted == nil || converted.Parent() != r.Function {
			return core.ErrGoToolchainContract
		}
	case *ssa.Convert:
		if converted == nil || converted.Parent() != r.Function {
			return core.ErrGoToolchainContract
		}
	default:
		return core.ErrGoToolchainContract
	}
	return nil
}
func (SSAConversionOperandRequest) goToolchainInternalFlow() {}

// SSAConversionOperandObservation borrows the input of an authenticated native
// conversion. Zero means its source block is untaken.
type SSAConversionOperandObservation struct{ Operand ssa.Value }

func (o SSAConversionOperandObservation) Validate() error {
	if o.Operand != nil && !nativeReceiverValuePresent(o.Operand) {
		return core.ErrGoToolchainOutput
	}
	return nil
}
func (SSAConversionOperandObservation) goToolchainInternalFlow() {}

// ConversionOperand projects one source input without constructing a conversion
// chain, applying product policy or implementing Go's conversion rules again.
func (v SSAUnitView) ConversionOperand(ctx context.Context, r SSAConversionOperandRequest) (out SSAConversionOperandObservation, err error) {
	defer func() {
		if err == nil {
			err = temporal.Wait(temporal.WaitRequest{Context: ctx})
		}
		if err != nil {
			out = SSAConversionOperandObservation{}
		}
	}()
	if err = errors.Join(temporal.Wait(temporal.WaitRequest{Context: ctx}), v.Validate(), r.Validate()); err != nil {
		return out, err
	}
	if r.Function.Prog != v.Program || r.Function.Pkg != v.Package {
		return out, core.ErrGoToolchainContract
	}
	var instruction ssa.Instruction
	var operand ssa.Value
	switch converted := r.Conversion.(type) {
	case *ssa.ChangeType:
		instruction, operand = converted, converted.X
	case *ssa.Convert:
		instruction, operand = converted, converted.X
	default:
		return out, core.ErrGoToolchainContract
	}
	if err = v.referrerInstructionMembership(instruction); err != nil {
		return out, err
	}
	untaken, queryErr := argumentCallUntakenBlock(instruction.Block())
	if queryErr != nil {
		return out, queryErr
	}
	if untaken {
		return out, nil
	}
	if err = v.fieldReadOperandBefore(ctx, operand, instruction); err != nil {
		return out, err
	}
	return SSAConversionOperandObservation{Operand: operand}, nil
}

var _ internalFlow = SSAConversionOperandRequest{}
var _ internalFlow = SSAConversionOperandObservation{}
