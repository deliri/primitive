package gotoolchain

import (
	"context"
	"errors"
	"go/ast"
	"reflect"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
	"golang.org/x/tools/go/ssa"
)

// SSASourceCallRequest borrows one type-checked source expression and its native
// function. Lexical child expressions do not imply calls in the parent frame.
type SSASourceCallRequest struct {
	Function   *ssa.Function
	Expression *ast.CallExpr
}

func (r SSASourceCallRequest) Validate() error {
	if r.Function == nil || r.Expression == nil {
		return core.ErrGoToolchainContract
	}
	return nil
}
func (SSASourceCallRequest) goToolchainInternalFlow() {}

// SSASourceCallObservation lends an actual immediate, deferred or spawned call.
// A nil instruction means that Go supplies no call at the selected expression.
type SSASourceCallObservation struct{ Instruction ssa.CallInstruction }

func (o SSASourceCallObservation) Validate() error {
	if o.Instruction != nil && reflect.ValueOf(o.Instruction).IsNil() {
		return core.ErrGoToolchainOutput
	}
	return nil
}
func (SSASourceCallObservation) goToolchainInternalFlow() {}

// CallForExpression uses Go's debug expression lookup for ordinary calls and
// native instruction positions for deferred, spawned and debug-free calls.
// It retains no expression, instruction or function population. Work follows
// Go's lookup and the selected native block, excluding compiler construction.
func (v SSAUnitView) CallForExpression(ctx context.Context, r SSASourceCallRequest) (out SSASourceCallObservation, err error) {
	defer func() {
		if err == nil {
			err = temporal.Wait(temporal.WaitRequest{Context: ctx})
		}
		if err != nil {
			out = SSASourceCallObservation{}
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
	value, _ := fn.ValueForExpr(r.Expression)
	if call, ok := value.(*ssa.Call); ok {
		if call == nil || call.Parent() != fn || call.Block() == nil || call.Block().Parent() != fn {
			return out, core.ErrGoToolchainContract
		}
		block := call.Block()
		if block.Index < 0 || block.Index >= len(fn.Blocks) || fn.Blocks[block.Index] != block {
			return out, core.ErrGoToolchainOutput
		}
		if argumentCallUntakenBlock(block) {
			return out, nil
		}
		for _, instruction := range block.Instrs {
			if instruction == call {
				return SSASourceCallObservation{Instruction: call}, nil
			}
		}
		return out, core.ErrGoToolchainContract
	}
	return v.sourceCallInInstructions(ctx, r)
}

// Keep iterator captures out of the ordinary Go expression lookup path.
func (v SSAUnitView) sourceCallInInstructions(ctx context.Context, r SSASourceCallRequest) (SSASourceCallObservation, error) {
	for instruction, queryErr := range v.Instructions(ctx, SSAInstructionRequest{Function: r.Function, Extent: SSAInstructionExtentFunction}) {
		if queryErr != nil {
			return SSASourceCallObservation{}, queryErr
		}
		if call, ok := instruction.(ssa.CallInstruction); ok && call.Common().Pos() == r.Expression.Lparen {
			return SSASourceCallObservation{Instruction: call}, nil
		}
	}
	return SSASourceCallObservation{}, nil
}

var _ internalFlow = SSASourceCallRequest{}
var _ internalFlow = SSASourceCallObservation{}
