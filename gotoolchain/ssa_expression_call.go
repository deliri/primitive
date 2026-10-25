package gotoolchain

import (
	"context"
	"errors"
	"go/ast"
	"go/types"
	"reflect"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
	"golang.org/x/tools/go/ssa"
)

// SSAExpressionCallRequest selects a source value in its actual native frame.
// It asks for the call producing that value, rather than executing a call at
// the expression's source position. SourceValues must have been requested.
type SSAExpressionCallRequest struct {
	Function   *ssa.Function
	Expression ast.Expr
}

func (r SSAExpressionCallRequest) Validate() error {
	if r.Function == nil || r.Expression == nil || reflect.ValueOf(r.Expression).Kind() != reflect.Pointer || reflect.ValueOf(r.Expression).IsNil() {
		return core.ErrGoToolchainContract
	}
	return nil
}
func (SSAExpressionCallRequest) goToolchainInternalFlow() {}

// SSAExpressionCallObservation borrows the actual producer and result ordinal.
// Copies and representation-preserving changes retain that origin; parameters,
// constants and merges supply no exact call. A call does not establish a named
// target or source body. Interface dispatch keeps its actual native call.
type SSAExpressionCallObservation struct {
	Call        *ssa.Call
	ResultIndex uint64
}

func (o SSAExpressionCallObservation) Validate() error {
	if o.Call == nil {
		if o.ResultIndex != 0 {
			return core.ErrGoToolchainContract
		}
		return nil
	}
	signature, err := nativeArgumentCallSignature(o.Call.Common())
	if err != nil {
		return err
	}
	if o.ResultIndex >= uint64(signature.Results().Len()) {
		return core.ErrGoToolchainOutput
	}
	return nil
}
func (SSAExpressionCallObservation) goToolchainInternalFlow() {}

// ExpressionCall owns Go's source-value lookup and follows only native copies,
// type changes and tuple projections. It retains no origin graph or population.
func (v SSAUnitView) ExpressionCall(ctx context.Context, r SSAExpressionCallRequest) (out SSAExpressionCallObservation, err error) {
	defer func() {
		err = errors.Join(err, temporal.Wait(temporal.WaitRequest{Context: ctx}))
		if err != nil {
			out = SSAExpressionCallObservation{}
		}
	}()
	if err = errors.Join(temporal.Wait(temporal.WaitRequest{Context: ctx}), v.Validate(), r.Validate()); err != nil {
		return out, err
	}
	fn := r.Function
	if fn.Prog != v.Program || fn.Pkg != v.Package || fn != v.Package.Func("init") && !v.argumentBindingFunctionOwned(fn) {
		return out, core.ErrGoToolchainContract
	}
	expression := ast.Unparen(r.Expression)
	facts, owned := v.Source.TypesInfo.Types[expression]
	if !owned {
		return out, core.ErrGoToolchainContract
	}
	if facts.Type == nil {
		return out, core.ErrGoToolchainOutput
	}
	if syntax := fn.Syntax(); syntax != nil && (expression.Pos() < syntax.Pos() || expression.End() > syntax.End()) {
		return out, core.ErrGoToolchainContract
	}
	value, address, queryErr := sourceExpressionValue(fn, expression)
	if queryErr != nil {
		return out, queryErr
	}
	if value == nil {
		if facts.IsNil() || facts.Value != nil {
			return out, nil
		}
		return out, core.ErrGoToolchainOutput
	}
	if address {
		return out, core.ErrGoToolchainOutput
	}
	nativeType, typeErr := expressionCallValueType(value)
	if typeErr != nil {
		return out, typeErr
	}
	if !types.Identical(nativeType, facts.Type) {
		return out, core.ErrGoToolchainOutput
	}
	if value.Parent() != nil && value.Parent() != fn {
		return out, core.ErrGoToolchainContract
	}
	return v.valueExpressionCall(ctx, value)
}

// Both named producer and actual-call observations share this origin walk.
// Existing conversion and tuple loans own operand custody and definition order.
func (v SSAUnitView) valueExpressionCall(ctx context.Context, value ssa.Value) (SSAExpressionCallObservation, error) {
	if value != nil {
		if _, err := expressionCallValueType(value); err != nil {
			return SSAExpressionCallObservation{}, err
		}
	}
	for {
		if err := temporal.Wait(temporal.WaitRequest{Context: ctx}); err != nil {
			return SSAExpressionCallObservation{}, err
		}
		converted, ok := value.(*ssa.ChangeType)
		if !ok {
			break
		}
		loan, err := v.ConversionOperand(ctx, SSAConversionOperandRequest{Function: converted.Parent(), Conversion: converted})
		if err != nil || loan.Operand == nil {
			return SSAExpressionCallObservation{}, err
		}
		value = loan.Operand
	}
	index := uint64(0)
	call, scalar := value.(*ssa.Call)
	if !scalar {
		extracted, ok := value.(*ssa.Extract)
		if !ok {
			return SSAExpressionCallObservation{}, nil
		}
		loan, err := v.TupleElement(ctx, SSATupleElementRequest{Element: extracted})
		if err != nil || loan.Tuple == nil {
			return SSAExpressionCallObservation{}, err
		}
		call, ok = loan.Tuple.(*ssa.Call)
		if !ok {
			return SSAExpressionCallObservation{}, nil
		}
		index = uint64(loan.Index)
	}
	// The named-target loan validates actual callable and operand custody even
	// when interface dispatch or a dynamic value has no exact named target.
	if _, err := v.CallFunction(ctx, SSACallFunctionRequest{Call: call}); err != nil {
		return SSAExpressionCallObservation{}, err
	}
	untaken, err := argumentCallUntakenBlock(call.Block())
	if err != nil || untaken {
		return SSAExpressionCallObservation{}, err
	}
	signature, err := nativeArgumentCallSignature(call.Common())
	if err != nil {
		return SSAExpressionCallObservation{}, err
	}
	if scalar && signature.Results().Len() != 1 {
		return SSAExpressionCallObservation{}, nil
	}
	out := SSAExpressionCallObservation{Call: call, ResultIndex: index}
	return out, out.Validate()
}

// Contain only the native value's type accessor, including Go assertions caused
// by malformed callable metadata. No caller callback executes in this boundary.
func expressionCallValueType(value ssa.Value) (typ types.Type, err error) {
	defer func() {
		if failure := recover(); failure != nil {
			typ, err = nil, core.ErrGoToolchainOutput
			if cause, native := failure.(error); native {
				err = errors.Join(err, cause)
			}
		}
	}()
	if value == nil || reflect.ValueOf(value).Kind() != reflect.Pointer || reflect.ValueOf(value).IsNil() {
		return nil, core.ErrGoToolchainOutput
	}
	typ = value.Type()
	if typ == nil {
		return nil, core.ErrGoToolchainOutput
	}
	return typ, nil
}

var (
	_ internalFlow = SSAExpressionCallRequest{}
	_ internalFlow = SSAExpressionCallObservation{}
)
