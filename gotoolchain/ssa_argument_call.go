package gotoolchain

import (
	"context"
	"errors"
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"reflect"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
	"golang.org/x/tools/go/ssa"
)

// SSAArgumentCallRequest borrows one native function, its selected operand and
// the actual target function. Index names the target's native argument slot,
// including a method receiver at zero. It contains no product call spelling.
type SSAArgumentCallRequest struct {
	Function *ssa.Function
	Argument ssa.Value
	Target   *types.Func
	Index    uint64
}

func (r SSAArgumentCallRequest) Validate() error {
	if r.Function == nil || r.Argument == nil || reflect.ValueOf(r.Argument).IsNil() || r.Target == nil || r.Target.Pkg() == nil {
		return core.ErrGoToolchainContract
	}
	count := r.Target.Signature().Params().Len()
	if r.Target.Signature().Recv() != nil {
		count++
	}
	if r.Index >= uint64(count) {
		return core.ErrGoToolchainContract
	}
	return nil
}
func (SSAArgumentCallRequest) goToolchainInternalFlow() {}

// ArgumentCall observes a possible synchronous call of the exact target with
// the selected native argument. Go owns IR, constant branches, dominators,
// closure bindings and bound-method wrappers. Only actual literal/wrapper
// invocations are followed; opaque named helpers grant no observation. Deferred
// calls are synchronous; spawned calls and uninvoked callbacks are excluded.
// This is source evidence, not proof of every runtime path or eventual return.
// Scratch follows active invocation ancestry, not instruction population.
func (v SSAUnitView) ArgumentCall(ctx context.Context, r SSAArgumentCallRequest) (observation SSARelationObservation, resultErr error) {
	if err := errors.Join(temporal.Wait(temporal.WaitRequest{Context: ctx}), v.Validate(), r.Validate()); err != nil {
		return observation, err
	}
	if r.Function.Prog != v.Program || r.Function.Pkg != v.Package || !v.ownsStorageValue(r.Argument) || r.Argument.Parent() != r.Function || v.Program.FuncValue(r.Target) == nil {
		return observation, core.ErrGoToolchainContract
	}
	defer func() {
		resultErr = errors.Join(resultErr, temporal.Wait(temporal.WaitRequest{Context: ctx}))
		if resultErr != nil {
			observation = SSARelationObservation{}
		}
	}()
	found, err := argumentCallInFrame(ctx, argumentCallFrame{function: r.Function, argument: r.Argument}, r.Target.Origin(), r.Index)
	if err != nil {
		return observation, err
	}
	return SSARelationObservation{Proven: found}, nil
}

type argumentCallFrame struct {
	function *ssa.Function
	argument ssa.Value
	parent   *argumentCallFrame
}

func (argumentCallFrame) goToolchainInternalFlow() {}

func argumentCallInFrame(ctx context.Context, frame argumentCallFrame, targetObject *types.Func, slot uint64) (bool, error) {
	for parent := frame.parent; parent != nil; parent = parent.parent {
		if parent.function == frame.function && parent.argument == frame.argument {
			return false, nil
		}
	}
	for _, block := range frame.function.Blocks {
		if argumentCallUntakenBlock(block) {
			continue
		}
		for _, instruction := range block.Instrs {
			if err := temporal.Wait(temporal.WaitRequest{Context: ctx}); err != nil {
				return false, err
			}
			call, ok := instruction.(ssa.CallInstruction)
			if !ok {
				continue
			}
			if _, spawn := instruction.(*ssa.Go); spawn {
				continue
			}
			common := call.Common()
			target := common.StaticCallee()
			if target == nil {
				continue
			}
			if object, ok := target.Object().(*types.Func); ok && object.Origin() == targetObject && slot < uint64(len(common.Args)) && argumentCallOperand(common.Args[slot], frame.argument) {
				return true, nil
			}
			_, literal := target.Syntax().(*ast.FuncLit)
			wrapper := target.Syntax() == nil && len(target.FreeVars) > 0
			if !literal && !wrapper {
				continue
			}
			if closure, ok := common.Value.(*ssa.MakeClosure); ok {
				if len(closure.Bindings) != len(target.FreeVars) {
					return false, core.ErrGoToolchainOutput
				}
				for index, binding := range closure.Bindings {
					if !argumentCallOperand(binding, frame.argument) {
						continue
					}
					if found, err := argumentCallInFrame(ctx, argumentCallFrame{function: target, argument: target.FreeVars[index], parent: &frame}, targetObject, slot); found || err != nil {
						return found, err
					}
				}
			}
			for index, argument := range common.Args {
				if index >= len(target.Params) || !argumentCallOperand(argument, frame.argument) {
					continue
				}
				if found, err := argumentCallInFrame(ctx, argumentCallFrame{function: target, argument: target.Params[index], parent: &frame}, targetObject, slot); found || err != nil {
					return found, err
				}
			}
		}
	}
	return false, temporal.Wait(temporal.WaitRequest{Context: ctx})
}

// SSA retains constant branch operands. A block dominated by the untaken
// successor is unreachable only when that successor has no alternate incoming
// edge. Go owns dominance; a goto must not be erased by a source-level guess.
func argumentCallUntakenBlock(block *ssa.BasicBlock) bool {
	for dominator := block.Idom(); dominator != nil; dominator = dominator.Idom() {
		if len(dominator.Instrs) == 0 {
			continue
		}
		branch, ok := dominator.Instrs[len(dominator.Instrs)-1].(*ssa.If)
		if !ok || len(dominator.Succs) != 2 {
			continue
		}
		condition, ok := branch.Cond.(*ssa.Const)
		if !ok || condition.Value == nil || condition.Value.Kind() != constant.Bool {
			continue
		}
		successor := dominator.Succs[0]
		if constant.BoolVal(condition.Value) {
			successor = dominator.Succs[1]
		}
		contained := true
		for _, predecessor := range successor.Preds {
			if predecessor != dominator && !successor.Dominates(predecessor) {
				contained = false
				break
			}
		}
		if contained && successor.Dominates(block) {
			return true
		}
	}
	return false
}

// Compare native values and Go's explicit representation-preserving operands.
// A captured parameter uses Go's allocation cell and its actual initial Store;
// a foreign or merged value cannot borrow the parameter's identity by spelling.
func argumentCallOperand(value, runner ssa.Value) bool {
	if value == nil || runner == nil {
		return false
	}
	if value == runner {
		return true
	}
	switch operand := value.(type) {
	case *ssa.MakeInterface:
		return argumentCallOperand(operand.X, runner)
	case *ssa.ChangeInterface:
		return argumentCallOperand(operand.X, runner)
	case *ssa.ChangeType:
		return argumentCallOperand(operand.X, runner)
	case *ssa.UnOp:
		if operand.Op == token.MUL {
			if operand.X == runner && !argumentCallCellUnchangedBeforeRead(operand) {
				return false
			}
			return argumentCallOperand(operand.X, runner)
		}
	case *ssa.Alloc:
		references := operand.Referrers()
		if references == nil {
			return false
		}
		stored := false
		for _, reference := range *references {
			switch reference.(type) {
			case *ssa.Store, *ssa.UnOp, *ssa.DebugRef, *ssa.MakeClosure:
			default:
				// An opaque callee can replace an escaped capture cell. Its
				// initial store no longer proves the operand used by a read.
				return false
			}
			store, ok := reference.(*ssa.Store)
			if !ok || store.Addr != operand {
				continue
			}
			if store.Val != runner {
				return false
			}
			stored = true
		}
		return stored
	}
	return false
}

// A capture cell's native stores cannot silently replace the chosen runner.
// A later store in the same block, or in a block dominated by this read, does
// not change the already loaded native value. No memory state is simulated.
func argumentCallCellUnchangedBeforeRead(read *ssa.UnOp) bool {
	references := read.X.Referrers()
	if references == nil {
		return false
	}
	for _, reference := range *references {
		store, ok := reference.(*ssa.Store)
		if !ok || store.Addr != read.X || argumentCallUntakenBlock(store.Block()) {
			continue
		}
		if store.Block() != read.Block() {
			if !read.Block().Dominates(store.Block()) {
				return false
			}
			continue
		}
		for _, instruction := range read.Block().Instrs {
			if instruction == read {
				break
			}
			if instruction == store {
				return false
			}
		}
	}
	return true
}

var _ internalFlow = SSAArgumentCallRequest{}
var _ internalFlow = argumentCallFrame{}
