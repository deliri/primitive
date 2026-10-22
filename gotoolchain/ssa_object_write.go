package gotoolchain

import (
	"context"
	"errors"
	"go/ast"
	"go/types"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
	"golang.org/x/tools/go/ssa"
)

// SSAObjectWriteRequest selects one native function and one exact source
// variable. SourceValues must have been requested to obtain write references;
// absent native references grant no observation.
type SSAObjectWriteRequest struct {
	Function *ssa.Function
	Object   *types.Var
}

func (r SSAObjectWriteRequest) Validate() error {
	if r.Function == nil || r.Object == nil || r.Object.Pkg() == nil || r.Object.Type() == nil || r.Object.IsField() {
		return core.ErrGoToolchainContract
	}
	return nil
}
func (SSAObjectWriteRequest) goToolchainInternalFlow() {}

// ObjectWritten observes an actual store paired with Go's adjacent source
// reference for the selected variable. A read, shadow, neighboring cell or
// nested function body grants no write fact. It retains one preceding store,
// without building assignment history or interpreting product cleanup policy.
func (v SSAUnitView) ObjectWritten(ctx context.Context, r SSAObjectWriteRequest) (observation SSARelationObservation, resultErr error) {
	if err := errors.Join(temporal.Wait(temporal.WaitRequest{Context: ctx}), v.Validate(), r.Validate()); err != nil {
		return observation, err
	}
	if r.Function.Prog != v.Program || r.Object.Pkg() != v.Source.Types {
		return observation, core.ErrGoToolchainContract
	}
	defer func() {
		resultErr = errors.Join(resultErr, temporal.Wait(temporal.WaitRequest{Context: ctx}))
		if resultErr != nil {
			observation = SSARelationObservation{}
		}
	}()
	var previous *ssa.Store
	for instruction, err := range v.Instructions(ctx, SSAInstructionRequest{Function: r.Function, Extent: SSAInstructionExtentFunction}) {
		if err != nil {
			return observation, err
		}
		if reference, ok := instruction.(*ssa.DebugRef); ok && previous != nil && reference.Block() == previous.Block() && !reference.IsAddr && reference.Object() == r.Object && reference.X == previous.Val {
			if reference.Expr == nil {
				return observation, core.ErrGoToolchainOutput
			}
			if _, identifier := reference.Expr.(*ast.Ident); identifier && reference.Expr.Pos() == previous.Pos() {
				return SSARelationObservation{Proven: true}, nil
			}
		}
		previous, _ = instruction.(*ssa.Store)
	}
	return observation, nil
}

var _ internalFlow = SSAObjectWriteRequest{}
