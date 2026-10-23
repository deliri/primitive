package gotoolchain

import (
	"context"
	"errors"
	"go/ast"
	"iter"
	"reflect"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
	"golang.org/x/tools/go/ssa"
)

// SSASynchronousCallRequest borrows one native function and explicitly admits
// immediate calls or synchronous calls including defers. It contains no product
// target names or copied instruction/call population.
type SSASynchronousCallRequest struct {
	Function *ssa.Function
	Timing   SSACallTiming
}

func (r SSASynchronousCallRequest) Validate() error {
	if r.Function == nil {
		return core.ErrGoToolchainContract
	}
	return r.Timing.Validate()
}
func (SSASynchronousCallRequest) goToolchainInternalFlow() {}

// SynchronousCalls lends actual native calls in instruction order, following
// only invoked literals and bound wrappers. A named helper's body and uninvoked
// callback supply no call authority. Imported declarations without a body yield
// no calls. The stream adds no call graph, alias table or deduplication world;
// repeated actual invocations can lend the same native site more than once.
// Memory follows active invocation ancestry, excluding Go's compiler graph.
func (v SSAUnitView) SynchronousCalls(ctx context.Context, r SSASynchronousCallRequest) iter.Seq2[ssa.CallInstruction, error] {
	return func(yield func(ssa.CallInstruction, error) bool) {
		if err := errors.Join(temporal.Wait(temporal.WaitRequest{Context: ctx}), v.Validate(), r.Validate()); err != nil {
			yield(nil, err)
			return
		}
		if r.Function.Prog != v.Program {
			yield(nil, core.ErrGoToolchainContract)
			return
		}
		if pkg := r.Function.Pkg; pkg != nil && v.Program.Package(pkg.Pkg) != pkg {
			yield(nil, core.ErrGoToolchainContract)
			return
		}
		for call, err := range synchronousCallsInFrame(ctx, synchronousCallFrame{function: r.Function}, r.Timing) {
			if !yield(call, err) || err != nil {
				return
			}
		}
		if err := temporal.Wait(temporal.WaitRequest{Context: ctx}); err != nil {
			yield(nil, err)
		}
	}
}

type synchronousCallFrame struct {
	function *ssa.Function
	parent   *synchronousCallFrame
}

func (synchronousCallFrame) goToolchainInternalFlow() {}

func synchronousCallsInFrame(ctx context.Context, frame synchronousCallFrame, timing SSACallTiming) iter.Seq2[ssa.CallInstruction, error] {
	return func(yield func(ssa.CallInstruction, error) bool) {
		for parent := frame.parent; parent != nil; parent = parent.parent {
			if parent.function == frame.function {
				return
			}
		}
		for index, block := range frame.function.Blocks {
			if err := temporal.Wait(temporal.WaitRequest{Context: ctx}); err != nil {
				yield(nil, err)
				return
			}
			if block == nil || block.Parent() != frame.function || block.Index != index {
				yield(nil, core.ErrGoToolchainOutput)
				return
			}
			untaken, queryErr := argumentCallUntakenBlock(block)
			if queryErr != nil {
				yield(nil, queryErr)
				return
			}
			if untaken {
				continue
			}
			for _, instruction := range block.Instrs {
				if err := temporal.Wait(temporal.WaitRequest{Context: ctx}); err != nil {
					yield(nil, err)
					return
				}
				if instruction == nil || reflect.ValueOf(instruction).IsNil() || instruction.Parent() != frame.function || instruction.Block() != block {
					yield(nil, core.ErrGoToolchainOutput)
					return
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
				if !yield(call, nil) {
					return
				}
				target := call.Common().StaticCallee()
				if target == nil {
					continue
				}
				_, literal := target.Syntax().(*ast.FuncLit)
				wrapper := target.Syntax() == nil && len(target.FreeVars) > 0
				if !literal && !wrapper {
					continue
				}
				for nested, err := range synchronousCallsInFrame(ctx, synchronousCallFrame{function: target, parent: &frame}, timing) {
					if !yield(nested, err) || err != nil {
						return
					}
				}
			}
		}
	}
}

var _ internalFlow = SSASynchronousCallRequest{}
var _ internalFlow = synchronousCallFrame{}
