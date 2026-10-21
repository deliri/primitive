package gotoolchain

import (
	"context"
	"errors"
	"go/types"

	pc "github.com/deliri/primitive/v2026/core"
	"golang.org/x/tools/go/ssa"
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
	for {
		converted, ok := value.(*ssa.ChangeType)
		if !ok {
			break
		}
		value = converted.X
	}
	index := uint64(0)
	call, scalar := value.(*ssa.Call)
	if !scalar {
		extracted, ok := value.(*ssa.Extract)
		if !ok {
			return ExpressionProducerObservation{}, nil
		}
		call, ok = extracted.Tuple.(*ssa.Call)
		if !ok {
			return ExpressionProducerObservation{}, nil
		}
		if extracted.Index < 0 {
			return ExpressionProducerObservation{}, pc.ErrGoToolchainOutput
		}
		index = uint64(extracted.Index)
	}
	target := call.Common().StaticCallee()
	if target == nil || scalar && target.Signature.Results().Len() != 1 {
		return ExpressionProducerObservation{}, nil
	}
	function, ok := target.Object().(*types.Func)
	if !ok {
		return ExpressionProducerObservation{}, nil
	}
	observation := ExpressionProducerObservation{Function: function, ResultIndex: index}
	if err := observation.Validate(); err != nil {
		return ExpressionProducerObservation{}, errors.Join(pc.ErrGoToolchainOutput, err)
	}
	return observation, nil
}
