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

// SSACallTiming distinguishes calls evaluated immediately from synchronous
// invocation including defers. A goroutine call belongs to neither selection.
type SSACallTiming uint8

const (
	SSACallTimingUnknown SSACallTiming = iota
	SSACallTimingImmediate
	SSACallTimingSynchronous
)

func (t SSACallTiming) Validate() error {
	if t != SSACallTimingImmediate && t != SSACallTimingSynchronous {
		return core.ErrGoToolchainContract
	}
	return nil
}
func (t SSACallTiming) IsValid() bool { return t.Validate() == nil }
func (SSACallTiming) OffWireEnum()    {}
func (t SSACallTiming) String() string {
	switch t {
	case SSACallTimingImmediate:
		return "immediate"
	case SSACallTimingSynchronous:
		return "synchronous"
	default:
		return core.UnknownEnumDiagnostic
	}
}

// SSAArgumentCallRequest borrows one native function, its selected operand and
// the actual target function. Index names the target's native argument slot,
// including a method receiver at zero. It contains no product call spelling.
type SSAArgumentCallRequest struct {
	Function *ssa.Function
	Argument ssa.Value
	Target   *types.Func
	Index    uint64
	Timing   SSACallTiming
}

func (r SSAArgumentCallRequest) Validate() error {
	if r.Function == nil || r.Argument == nil || reflect.ValueOf(r.Argument).IsNil() || r.Target == nil || r.Target.Pkg() == nil {
		return core.ErrGoToolchainContract
	}
	if err := r.Timing.Validate(); err != nil {
		return err
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
// calls are synchronous; Timing selects whether they are admitted. Spawned
// calls and uninvoked callbacks are excluded.
// This is source evidence, not proof of every runtime path or eventual return.
// A promoted receiver follows Go's actual selection path. Scratch follows
// active invocation ancestry, not instruction population.
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
	var path []int
	if r.Target.Signature().Recv() != nil {
		if selection, present := types.LookupSelection(r.Argument.Type(), false, r.Target.Pkg(), r.Target.Name()); present {
			if selected, ok := selection.Obj().(*types.Func); ok && selected.Origin() == r.Target.Origin() {
				indices := selection.Index()
				path = indices[:len(indices)-1]
			}
		}
	}
	found, err := argumentCallInFrame(ctx, argumentCallFrame{function: r.Function, argument: r.Argument, receiverPath: path}, r.Target.Origin(), r.Index, r.Timing)
	if err != nil {
		return observation, err
	}
	return SSARelationObservation{Proven: found}, nil
}

type argumentCallFrame struct {
	function     *ssa.Function
	argument     ssa.Value
	parent       *argumentCallFrame
	receiverPath []int
}

func (argumentCallFrame) goToolchainInternalFlow() {}

func argumentCallInFrame(ctx context.Context, frame argumentCallFrame, targetObject *types.Func, slot uint64, timing SSACallTiming) (bool, error) {
	for parent := frame.parent; parent != nil; parent = parent.parent {
		if parent.function == frame.function && parent.argument == frame.argument {
			return false, nil
		}
	}
	for _, block := range frame.function.Blocks {
		untaken, queryErr := argumentCallUntakenBlock(block)
		if queryErr != nil {
			return false, queryErr
		}
		if untaken {
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
			if _, deferred := instruction.(*ssa.Defer); deferred && timing == SSACallTimingImmediate {
				continue
			}
			common := call.Common()
			target := common.StaticCallee()
			if target == nil {
				continue
			}
			if object, ok := target.Object().(*types.Func); ok && object.Origin() == targetObject && slot < uint64(len(common.Args)) {
				matched, queryErr := argumentCallReceiver(common.Args[slot], frame.argument, frame.receiverPath)
				if matched || queryErr != nil {
					return matched, queryErr
				}
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
					matched, queryErr := argumentCallOperand(binding, frame.argument)
					if queryErr != nil {
						return false, queryErr
					}
					if matched {
						if found, err := argumentCallInFrame(ctx, argumentCallFrame{function: target, argument: target.FreeVars[index], parent: &frame, receiverPath: frame.receiverPath}, targetObject, slot, timing); found || err != nil {
							return found, err
						}
					}
					if len(frame.receiverPath) > 0 {
						matched, queryErr = argumentCallReceiver(binding, frame.argument, frame.receiverPath)
						if queryErr != nil {
							return false, queryErr
						}
					}
					if len(frame.receiverPath) > 0 && matched {
						if found, err := argumentCallInFrame(ctx, argumentCallFrame{function: target, argument: target.FreeVars[index], parent: &frame}, targetObject, slot, timing); found || err != nil {
							return found, err
						}
					}
				}
			}
			for index, argument := range common.Args {
				if index >= len(target.Params) {
					continue
				}
				matched, queryErr := argumentCallOperand(argument, frame.argument)
				if queryErr != nil {
					return false, queryErr
				}
				if matched {
					if found, err := argumentCallInFrame(ctx, argumentCallFrame{function: target, argument: target.Params[index], parent: &frame, receiverPath: frame.receiverPath}, targetObject, slot, timing); found || err != nil {
						return found, err
					}
				}
				if len(frame.receiverPath) > 0 {
					matched, queryErr = argumentCallReceiver(argument, frame.argument, frame.receiverPath)
					if queryErr != nil {
						return false, queryErr
					}
				}
				if len(frame.receiverPath) > 0 && matched {
					if found, err := argumentCallInFrame(ctx, argumentCallFrame{function: target, argument: target.Params[index], parent: &frame}, targetObject, slot, timing); found || err != nil {
						return found, err
					}
				}
			}
		}
	}
	return false, temporal.Wait(temporal.WaitRequest{Context: ctx})
}

// SSA retains constant branch operands. A block dominated by the untaken
// successor is unreachable only when that successor has no alternate incoming
// edge. Go owns dominance; a goto must not be erased by a source-level guess.
func argumentCallUntakenBlock(block *ssa.BasicBlock) (bool, error) {
	if block == nil || block.Parent() == nil || !nativeBlockLinkMember(block, block.Parent()) {
		return false, core.ErrGoToolchainOutput
	}
	fn := block.Parent()
	for dominator := block.Idom(); dominator != nil; dominator = dominator.Idom() {
		if !nativeBlockLinkMember(dominator, fn) {
			return false, core.ErrGoToolchainOutput
		}
		if len(dominator.Instrs) == 0 {
			continue
		}
		terminal := dominator.Instrs[len(dominator.Instrs)-1]
		if terminal == nil || reflect.ValueOf(terminal).IsNil() || terminal.Parent() != fn || terminal.Block() != dominator {
			return false, core.ErrGoToolchainOutput
		}
		branch, ok := terminal.(*ssa.If)
		if !ok {
			continue
		}
		if !nativeReceiverValuePresent(branch.Cond) || len(dominator.Succs) != 2 {
			return false, core.ErrGoToolchainOutput
		}
		for _, successor := range dominator.Succs {
			if !nativeBlockLinkMember(successor, fn) {
				return false, core.ErrGoToolchainOutput
			}
		}
		condition, ok := branch.Cond.(*ssa.Const)
		if !ok {
			continue
		}
		if condition.Value == nil || condition.Value.Kind() != constant.Bool {
			return false, core.ErrGoToolchainOutput
		}
		successor := dominator.Succs[0]
		if constant.BoolVal(condition.Value) {
			successor = dominator.Succs[1]
		}
		contained := true
		for _, predecessor := range successor.Preds {
			if !nativeBlockLinkMember(predecessor, fn) {
				return false, core.ErrGoToolchainOutput
			}
			if predecessor != dominator && !successor.Dominates(predecessor) {
				contained = false
				break
			}
		}
		if contained && successor.Dominates(block) {
			return true, nil
		}
	}
	return false, nil
}

// Compare native values and Go's explicit representation-preserving operands.
// A captured parameter uses Go's allocation cell and its actual initial Store;
// a foreign or merged value cannot borrow the parameter's identity by spelling.
func argumentCallOperand(value, runner ssa.Value) (bool, error) {
	if value == nil || runner == nil {
		return false, nil
	}
	if value == runner {
		return true, nil
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
			if operand.X == runner {
				unchanged, err := argumentCallCellUnchangedBeforeRead(operand)
				if !unchanged || err != nil {
					return false, err
				}
			}
			return argumentCallOperand(operand.X, runner)
		}
	case *ssa.Alloc:
		references := operand.Referrers()
		if references == nil {
			return false, nil
		}
		stored := false
		for _, reference := range *references {
			switch reference.(type) {
			case *ssa.Store, *ssa.UnOp, *ssa.DebugRef, *ssa.MakeClosure:
			default:
				// An opaque callee can replace an escaped capture cell. Its
				// initial store no longer proves the operand used by a read.
				return false, nil
			}
			store, ok := reference.(*ssa.Store)
			if !ok || store.Addr != operand {
				continue
			}
			if store.Val != runner {
				return false, nil
			}
			stored = true
		}
		return stored, nil
	}
	return false, nil
}

// A capture cell's native stores cannot silently replace the chosen runner.
// A later store in the same block, or in a block dominated by this read, does
// not change the already loaded native value. No memory state is simulated.
func argumentCallCellUnchangedBeforeRead(read *ssa.UnOp) (bool, error) {
	references := read.X.Referrers()
	if references == nil {
		return false, nil
	}
	for _, reference := range *references {
		store, ok := reference.(*ssa.Store)
		if !ok || store.Addr != read.X {
			continue
		}
		untaken, err := argumentCallUntakenBlock(store.Block())
		if err != nil {
			return false, err
		}
		if untaken {
			continue
		}
		if store.Block() != read.Block() {
			if !read.Block().Dominates(store.Block()) {
				return false, nil
			}
			continue
		}
		for _, instruction := range read.Block().Instrs {
			if instruction == read {
				break
			}
			if instruction == store {
				return false, nil
			}
		}
	}
	return true, nil
}

var _ internalFlow = SSAArgumentCallRequest{}
var _ internalFlow = argumentCallFrame{}

// The promoted path is Go's Selection.Index loan. A neighboring field or
// different owner cannot lend its receiver to the selected target.
func argumentCallReceiver(value, reference ssa.Value, path []int) (bool, error) {
	if len(path) == 0 {
		return argumentCallOperand(value, reference)
	}
	switch field := value.(type) {
	case *ssa.FieldAddr:
		if field.Field != path[len(path)-1] {
			return false, nil
		}
		return argumentCallReceiver(field.X, reference, path[:len(path)-1])
	case *ssa.Field:
		if field.Field != path[len(path)-1] {
			return false, nil
		}
		return argumentCallReceiver(field.X, reference, path[:len(path)-1])
	case *ssa.UnOp:
		if field.Op != token.MUL {
			return false, nil
		}
		return argumentCallReceiver(field.X, reference, path)
	default:
		return false, nil
	}
}

var _ core.OffWireEnum = SSACallTimingUnknown
