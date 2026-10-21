package gotoolchain

import (
	"context"
	"errors"
	"go/ast"
	"go/build/constraint"
	"reflect"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

// GoBuildConstraintRequest borrows a native comment. Only the modern go:build
// form is observed; ordinary comments and legacy +build text remain absent.
type GoBuildConstraintRequest struct{ Comment *ast.Comment }

func (r GoBuildConstraintRequest) Validate() error {
	if r.Comment == nil {
		return core.ErrGoToolchainContract
	}
	return nil
}

func (GoBuildConstraintRequest) goToolchainProtocolFact() {}

// GoBuildConstraintObservation lends Go's expression tree. It is syntax data,
// not a decision about build selection or a particular tag's meaning.
type GoBuildConstraintObservation struct {
	Expression constraint.Expr
	Present    bool
}

func (o GoBuildConstraintObservation) Validate() error {
	if o.Expression == nil {
		if o.Present {
			return core.ErrGoToolchainContract
		}
		return nil
	}
	if !o.Present || reflect.ValueOf(o.Expression).IsNil() {
		return core.ErrGoToolchainContract
	}
	return nil
}

func (GoBuildConstraintObservation) goToolchainProtocolFact() {}

var (
	_ protocolFact = GoBuildConstraintRequest{}
	_ protocolFact = GoBuildConstraintObservation{}
)

// ObserveGoBuildConstraint uses Go's recognizer and parser without a copied
// grammar, tag catalog or input quota. Native syntax errors retain their type.
// A refusal never publishes a provisional expression.
func ObserveGoBuildConstraint(ctx context.Context, request GoBuildConstraintRequest) (GoBuildConstraintObservation, error) {
	if err := errors.Join(request.Validate(), temporal.Wait(temporal.WaitRequest{Context: ctx})); err != nil {
		return GoBuildConstraintObservation{}, errors.Join(core.ErrGoToolchainContract, err)
	}
	var observation GoBuildConstraintObservation
	var nativeErr error
	if constraint.IsGoBuild(request.Comment.Text) {
		observation.Expression, nativeErr = constraint.Parse(request.Comment.Text)
		observation.Present = true
		if nativeErr != nil {
			nativeErr = errors.Join(core.ErrGoToolchainOutput, nativeErr)
		}
	}
	if err := errors.Join(nativeErr, temporal.Wait(temporal.WaitRequest{Context: ctx})); err != nil {
		return GoBuildConstraintObservation{}, err
	}
	return observation, nil
}
