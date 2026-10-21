package gotoolchain

import (
	"context"
	"golang.org/x/tools/go/ssa"
)

// ExpressionNilObservation reports only a compiler-proven nil value. False
// includes opaque runtime values; a typed nil inside an interface is not nil.
type ExpressionNilObservation struct{ Proven bool }

func (ExpressionNilObservation) Validate() error          { return nil }
func (ExpressionNilObservation) goToolchainProtocolFact() {}

func (v SSAUnitView) ExpressionNil(ctx context.Context, request ExpressionRequest) (ExpressionNilObservation, error) {
	value, facts, address, err := v.expressionValue(ctx, request)
	if err != nil {
		return ExpressionNilObservation{}, err
	}
	constant, ok := value.(*ssa.Const)
	return ExpressionNilObservation{Proven: facts.IsNil() || !address && ok && constant.IsNil()}, nil
}
