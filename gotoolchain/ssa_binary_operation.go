package gotoolchain

import (
	"context"
	"errors"
	"go/ast"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
	"golang.org/x/tools/go/ssa"
)

// SSABinaryOperationRequest borrows a type-checked source expression and the
// native function in which Go may have emitted its operation.
type SSABinaryOperationRequest struct {
	Function   *ssa.Function
	Expression *ast.BinaryExpr
}

func (r SSABinaryOperationRequest) Validate() error {
	if r.Function == nil || r.Expression == nil {
		return core.ErrGoToolchainContract
	}
	return nil
}
func (SSABinaryOperationRequest) goToolchainInternalFlow() {}

// SSABinaryOperationObservation lends Go's actual instruction. Nil means no
// native binary operation, including constant folding and known untaken code.
type SSABinaryOperationObservation struct{ Operation *ssa.BinOp }

func (o SSABinaryOperationObservation) Validate() error {
	if operation := o.Operation; operation != nil {
		if operation.Block() == nil || operation.Parent() == nil || operation.Block().Parent() != operation.Parent() {
			return core.ErrGoToolchainOutput
		}
	}
	return nil
}
func (SSABinaryOperationObservation) goToolchainInternalFlow() {}

// BinaryOperationForExpression owns native lookup and instruction custody.
// Debug-free lookup follows Go's operation positions. Neither path constructs
// an expression index or retains an instruction population.
func (v SSAUnitView) BinaryOperationForExpression(ctx context.Context, r SSABinaryOperationRequest) (out SSABinaryOperationObservation, err error) {
	defer func() {
		if err == nil {
			err = temporal.Wait(temporal.WaitRequest{Context: ctx})
		}
		if err != nil {
			out = SSABinaryOperationObservation{}
		}
	}()
	if err = errors.Join(temporal.Wait(temporal.WaitRequest{Context: ctx}), v.Validate(), r.Validate()); err != nil {
		return out, err
	}
	fn := r.Function
	if fn.Prog != v.Program || fn.Pkg != v.Package {
		return out, core.ErrGoToolchainContract
	}
	if _, owned := v.Source.TypesInfo.Types[r.Expression]; !owned {
		return out, core.ErrGoToolchainContract
	}
	if syntax := fn.Syntax(); syntax != nil {
		if r.Expression.Pos() < syntax.Pos() || r.Expression.End() > syntax.End() {
			return out, core.ErrGoToolchainContract
		}
	} else if fn != v.Package.Func("init") {
		return out, core.ErrGoToolchainContract
	}
	value, lookupErr := sourceExpressionValue(fn, r.Expression)
	if lookupErr != nil {
		return out, lookupErr
	}
	if operation, native := value.(*ssa.BinOp); native {
		if operation == nil || operation.Parent() != fn || operation.Pos() != r.Expression.OpPos || operation.Op != r.Expression.Op {
			return out, core.ErrGoToolchainContract
		}
		if membershipErr := v.referrerInstructionMembership(operation); membershipErr != nil {
			return out, membershipErr
		}
		untaken, queryErr := argumentCallUntakenBlock(operation.Block())
		if queryErr != nil {
			return out, queryErr
		}
		if !untaken {
			out.Operation = operation
		}
		return out, nil
	}
	for instruction, instructionErr := range v.Instructions(ctx, SSAInstructionRequest{Function: fn, Extent: SSAInstructionExtentFunction}) {
		if instructionErr != nil {
			return out, instructionErr
		}
		if operation, native := instruction.(*ssa.BinOp); native && operation.Pos() == r.Expression.OpPos && operation.Op == r.Expression.Op {
			return SSABinaryOperationObservation{Operation: operation}, nil
		}
	}
	return out, nil
}

var _ internalFlow = SSABinaryOperationRequest{}
var _ internalFlow = SSABinaryOperationObservation{}
