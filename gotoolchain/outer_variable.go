package gotoolchain

import (
	"context"
	"errors"
	"go/token"
	"go/types"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
)

// OuterVariableRequest borrows one native declaration and its visibility
// position. Go's checker owns its parent scope; no caller name index is needed.
type OuterVariableRequest struct {
	Variable *types.Var
	Position token.Pos
}

func (OuterVariableRequest) goToolchainCapabilityWrapper() {}

func (r OuterVariableRequest) Validate() error {
	if r.Variable == nil || r.Position <= token.NoPos {
		return core.ErrGoToolchainContract
	}
	return nil
}

// OuterVariableObservation lends the nearest visible outer variable, or nil
// for absence. A nearer non-variable symbol hides more distant variables.
// The compiler analysis retains ownership of the declaration and scopes.
type OuterVariableObservation struct {
	Variable *types.Var
}

func (OuterVariableObservation) goToolchainProtocolFact() {}

// ObserveOuterVariable delegates lexical visibility, declaration order and
// parent traversal to Go's native scope lookup. It retains no scope inventory.
func ObserveOuterVariable(ctx context.Context, request OuterVariableRequest) (OuterVariableObservation, error) {
	if err := errors.Join(contextstate.Validate(ctx), request.Validate()); err != nil {
		if ctx != nil {
			err = errors.Join(err, context.Cause(ctx))
		}
		return OuterVariableObservation{}, errors.Join(core.ErrGoToolchainContract, err)
	}
	var observation OuterVariableObservation
	if scope := request.Variable.Parent(); scope != nil && scope.Parent() != nil {
		_, object := scope.Parent().LookupParent(request.Variable.Name(), request.Position)
		observation.Variable, _ = object.(*types.Var)
	}
	if err := errors.Join(contextstate.Validate(ctx), context.Cause(ctx)); err != nil {
		return OuterVariableObservation{}, errors.Join(core.ErrGoToolchainContract, err)
	}
	return observation, nil
}
