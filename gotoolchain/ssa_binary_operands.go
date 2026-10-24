package gotoolchain

import (
	"context"
	"errors"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
	"golang.org/x/tools/go/ssa"
)

// SSABinaryOperandsRequest selects one actual binary operation in its owning
// function. It borrows native operands without interpreting an expression tree.
type SSABinaryOperandsRequest struct {
	Function  *ssa.Function
	Operation *ssa.BinOp
}

func (r SSABinaryOperandsRequest) Validate() error {
	if r.Function == nil || r.Operation == nil || r.Operation.Parent() != r.Function {
		return core.ErrGoToolchainContract
	}
	return nil
}
func (SSABinaryOperandsRequest) goToolchainInternalFlow() {}

// SSABinaryOperandsObservation lends both native inputs. Zero is ordinary
// absence for an untaken operation. No input receives independent authority.
type SSABinaryOperandsObservation struct{ Left, Right ssa.Value }

func (o SSABinaryOperandsObservation) Validate() error {
	if o.Left == nil && o.Right == nil {
		return nil
	}
	if !nativeReceiverValuePresent(o.Left) || !nativeReceiverValuePresent(o.Right) {
		return core.ErrGoToolchainOutput
	}
	return nil
}
func (SSABinaryOperandsObservation) goToolchainInternalFlow() {}

// BinaryOperands validates the enclosing instruction and both input identities
// before lending either. Native ordering and representation remain Go-owned;
// no expression index, operand catalog or copied instruction population exists.
func (v SSAUnitView) BinaryOperands(ctx context.Context, r SSABinaryOperandsRequest) (out SSABinaryOperandsObservation, err error) {
	defer func() {
		if err == nil {
			err = temporal.Wait(temporal.WaitRequest{Context: ctx})
		}
		if err != nil {
			out = SSABinaryOperandsObservation{}
		}
	}()
	if err = errors.Join(temporal.Wait(temporal.WaitRequest{Context: ctx}), v.Validate(), r.Validate()); err != nil {
		return out, err
	}
	if r.Function.Prog != v.Program || r.Function.Pkg != v.Package {
		return out, core.ErrGoToolchainContract
	}
	if err = v.referrerInstructionMembership(r.Operation); err != nil {
		return out, err
	}
	untaken, queryErr := argumentCallUntakenBlock(r.Operation.Block())
	if queryErr != nil {
		return out, queryErr
	}
	if untaken {
		return out, nil
	}
	for _, operand := range [2]ssa.Value{r.Operation.X, r.Operation.Y} {
		if err = v.fieldReadOperandBefore(ctx, operand, r.Operation); err != nil {
			return out, err
		}
	}
	return SSABinaryOperandsObservation{Left: r.Operation.X, Right: r.Operation.Y}, nil
}

var _ internalFlow = SSABinaryOperandsRequest{}
var _ internalFlow = SSABinaryOperandsObservation{}
