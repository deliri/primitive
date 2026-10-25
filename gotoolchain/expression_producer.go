package gotoolchain

import (
	"context"
	"errors"
	"go/types"

	pc "github.com/deliri/primitive/v2026/core"
)

// ExpressionProducerObservation borrows the actual named Go function and its
// result slot. A nil function with zero slot means no exact producer was
// established. Dynamic calls, merged origins and runtime values remain opaque.
type ExpressionProducerObservation struct {
	Function    *types.Func
	ResultIndex uint64
}

func (o ExpressionProducerObservation) Validate() error {
	if o.Function == nil {
		if o.ResultIndex != 0 {
			return pc.ErrGoToolchainContract
		}
		return nil
	}
	if o.Function.Pkg() == nil || o.ResultIndex >= uint64(o.Function.Signature().Results().Len()) {
		return pc.ErrGoToolchainContract
	}
	return nil
}
func (ExpressionProducerObservation) goToolchainInternalFlow() {}

// ExpressionProducer reads native SSA value identity, including copies and
// representation-preserving type changes. It does not replay assignments or
// invent an origin for a merge. Go owns tuple extraction and actual targets.
func (v SSAUnitView) ExpressionProducer(ctx context.Context, request ExpressionRequest) (ExpressionProducerObservation, error) {
	value, _, address, err := v.expressionValue(ctx, request)
	if err != nil {
		return ExpressionProducerObservation{}, err
	}
	if address || value == nil {
		return ExpressionProducerObservation{}, nil
	}
	origin, err := v.valueExpressionCall(ctx, value)
	if err != nil {
		return ExpressionProducerObservation{}, err
	}
	if origin.Call == nil {
		return ExpressionProducerObservation{}, nil
	}
	declaration, err := v.CallFunction(ctx, SSACallFunctionRequest{Call: origin.Call})
	if err != nil {
		return ExpressionProducerObservation{}, err
	}
	if declaration.Function == nil {
		return ExpressionProducerObservation{}, nil
	}
	observation := ExpressionProducerObservation{Function: declaration.Function, ResultIndex: origin.ResultIndex}
	if err := observation.Validate(); err != nil {
		return ExpressionProducerObservation{}, errors.Join(pc.ErrGoToolchainOutput, err)
	}
	return observation, nil
}
