package gotoolchain

import (
	"context"
	"errors"
	"go/constant"
	"go/token"
	"go/types"
	"reflect"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
	"golang.org/x/tools/go/ssa"
)

// SSAConditionalBranchRequest borrows one actual native block.
type SSAConditionalBranchRequest struct {
	Block *ssa.BasicBlock
}

func (r SSAConditionalBranchRequest) Validate() error {
	if r.Block == nil || r.Block.Parent() == nil {
		return core.ErrGoToolchainContract
	}
	return nil
}
func (SSAConditionalBranchRequest) goToolchainInternalFlow() {}

// SSAConditionalBranchObservation projects native boolean negation and boolean
// constant equality onto the corresponding true and false edges. Other boolean
// values remain opaque. A nonconditional or known untaken block returns zero.
// This observation makes no claim about the meaning of the condition.
type SSAConditionalBranchObservation struct {
	Condition ssa.Value
	WhenTrue  *ssa.BasicBlock
	WhenFalse *ssa.BasicBlock
}

func (o SSAConditionalBranchObservation) Validate() error {
	if o.Condition == nil && o.WhenTrue == nil && o.WhenFalse == nil {
		return nil
	}
	if !nativeConditionPresent(o.Condition) || !nativeConditionBoolean(o.Condition) || o.WhenTrue == nil || o.WhenFalse == nil || o.WhenTrue.Parent() == nil || o.WhenTrue.Parent() != o.WhenFalse.Parent() {
		return core.ErrGoToolchainOutput
	}
	return nil
}

func (SSAConditionalBranchObservation) goToolchainInternalFlow() {}

// ConditionalBranch authenticates the selected terminator and each normalized
// operand. The cursor moves strictly backward through native definitions; it
// retains no graph, visited set, depth limit or copied instruction population.
func (v SSAUnitView) ConditionalBranch(ctx context.Context, r SSAConditionalBranchRequest) (out SSAConditionalBranchObservation, err error) {
	defer func() {
		if err == nil {
			err = temporal.Wait(temporal.WaitRequest{Context: ctx})
		}
		if err != nil {
			out = SSAConditionalBranchObservation{}
		}
	}()
	if err = errors.Join(temporal.Wait(temporal.WaitRequest{Context: ctx}), v.Validate(), r.Validate()); err != nil {
		return out, err
	}
	fn := r.Block.Parent()
	var branch *ssa.If
	for instruction, queryErr := range v.Instructions(ctx, SSAInstructionRequest{Function: fn, Block: r.Block, Extent: SSAInstructionExtentBlockTerminator}) {
		if queryErr != nil {
			return out, queryErr
		}
		branch, _ = instruction.(*ssa.If)
	}
	if branch == nil {
		return out, nil
	}
	if len(r.Block.Succs) != 2 || !nativeBlockLinkMember(r.Block.Succs[0], fn) || !nativeBlockLinkMember(r.Block.Succs[1], fn) {
		return out, core.ErrGoToolchainOutput
	}
	out = SSAConditionalBranchObservation{Condition: branch.Cond, WhenTrue: r.Block.Succs[0], WhenFalse: r.Block.Succs[1]}
	cursor := nativeConditionCursor{block: r.Block, before: len(r.Block.Instrs) - 1}
	for {
		if err = cursor.borrow(ctx, out.Condition); err != nil {
			return out, err
		}
		switch condition := out.Condition.(type) {
		case *ssa.UnOp:
			if condition.Op != token.NOT {
				return out, nil
			}
			out.Condition = condition.X
			out.WhenTrue, out.WhenFalse = out.WhenFalse, out.WhenTrue
		case *ssa.BinOp:
			if condition.Op != token.EQL && condition.Op != token.NEQ {
				return out, nil
			}
			if !nativeConditionPresent(condition.X) || !nativeConditionPresent(condition.Y) {
				return out, core.ErrGoToolchainOutput
			}
			xBool, yBool := nativeConditionBoolean(condition.X), nativeConditionBoolean(condition.Y)
			if !xBool && !yBool {
				return out, nil
			}
			if !xBool || !yBool {
				return out, core.ErrGoToolchainOutput
			}
			operand, known := condition.X, condition.Y
			boolean, ok := known.(*ssa.Const)
			if !ok {
				operand, known = known, operand
				boolean, ok = known.(*ssa.Const)
			}
			if !ok {
				return out, nil
			}
			if boolean.Value == nil || boolean.Value.Kind() != constant.Bool {
				return out, core.ErrGoToolchainOutput
			}
			if constant.BoolVal(boolean.Value) == (condition.Op == token.NEQ) {
				out.WhenTrue, out.WhenFalse = out.WhenFalse, out.WhenTrue
			}
			out.Condition = operand
		default:
			return out, nil
		}
	}
}

type nativeConditionCursor struct {
	block  *ssa.BasicBlock
	before int
}

func (*nativeConditionCursor) goToolchainInternalFlow() {}

func (c *nativeConditionCursor) borrow(ctx context.Context, value ssa.Value) error {
	if err := temporal.Wait(temporal.WaitRequest{Context: ctx}); err != nil {
		return err
	}
	if !nativeConditionPresent(value) || !nativeConditionBoolean(value) {
		return core.ErrGoToolchainOutput
	}
	fn := c.block.Parent()
	switch native := value.(type) {
	case *ssa.Const:
		if native.Value != nil && native.Value.Kind() == constant.Bool {
			return nil
		}
	case *ssa.Parameter:
		for _, actual := range fn.Params {
			if actual == native && native.Parent() == fn {
				return nil
			}
		}
	case *ssa.FreeVar:
		for _, actual := range fn.FreeVars {
			if actual == native && native.Parent() == fn {
				return nil
			}
		}
	case ssa.Instruction:
		block := native.Block()
		if native.Parent() != fn || !nativeBlockLinkMember(block, fn) {
			return core.ErrGoToolchainOutput
		}
		if block != c.block {
			if !block.Dominates(c.block) {
				return core.ErrGoToolchainOutput
			}
			c.block, c.before = block, len(block.Instrs)
		}
		for c.before > 0 {
			if err := temporal.Wait(temporal.WaitRequest{Context: ctx}); err != nil {
				return err
			}
			c.before--
			if c.block.Instrs[c.before] == native {
				return nil
			}
		}
	}
	return core.ErrGoToolchainOutput
}

func nativeConditionPresent(value ssa.Value) bool {
	return value != nil && !reflect.ValueOf(value).IsNil()
}

func nativeConditionBoolean(value ssa.Value) bool {
	if value.Type() == nil {
		return false
	}
	basic, ok := value.Type().Underlying().(*types.Basic)
	return ok && basic.Info()&types.IsBoolean != 0
}

var (
	_ internalFlow = SSAConditionalBranchRequest{}
	_ internalFlow = SSAConditionalBranchObservation{}
	_ internalFlow = (*nativeConditionCursor)(nil)
)
