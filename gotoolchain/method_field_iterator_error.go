package gotoolchain

import (
	"context"
	"go/ast"
	"go/types"
	"iter"

	"github.com/deliri/primitive/v2026/core"
	"golang.org/x/tools/go/ssa"
)

// Go binds the range's variables to this exact generated callback. A source
// return in that callback returns from its enclosing function, even though its
// native result is bool. Break/continue/default callback returns lend no cause.
func errorSourceReturnPreserves(ctx context.Context, returned *ssa.Return, reference errorIdentityQuery) (bool, error) {
	if !returned.Pos().IsValid() {
		return false, nil
	}
	var body *ast.BlockStmt
	switch function := reference.Frame.Syntax().(type) {
	case *ast.FuncDecl:
		body = function.Body
	case *ast.FuncLit:
		body = function.Body
	case *ast.RangeStmt:
		body = function.Body
	}
	if body == nil {
		return false, nil
	}
	// A source return is evaluated before cleanup. Unknown cleanup must not
	// lend authority over a result cell that it can subsequently replace.
	for function := reference.Frame; function != nil; function = function.Parent() {
		for instruction, err := range reference.View.Instructions(ctx, SSAInstructionRequest{Function: function, Extent: SSAInstructionExtentFunction}) {
			if err != nil {
				return false, err
			}
			switch instruction.(type) {
			case *ssa.Defer, *ssa.Go:
				return false, nil
			}
		}
	}
	for syntax, err := range WalkSyntax(ctx, SyntaxWalkRequest{Root: body}) {
		if err != nil {
			return false, err
		}
		statement, ok := syntax.Node.(*ast.ReturnStmt)
		if !ok || statement.Return != returned.Pos() {
			continue
		}
		for _, ancestor := range syntax.Ancestors {
			if _, nested := ancestor.(*ast.FuncLit); nested {
				return false, nil
			}
		}
		for _, expression := range statement.Results {
			value := reference.View.Source.TypesInfo.TypeOf(expression)
			if value != nil && types.AssignableTo(value, types.Universe.Lookup("error").Type()) {
				if preserved, err := reference.Preserved(ctx, expression); preserved || err != nil {
					return preserved, err
				}
			}
		}
	}
	return false, nil
}

func methodFieldErrorIterator(ctx context.Context, ranged *ast.RangeStmt, query methodFieldErrorQuery) (bool, error) {
	call, ok := ast.Unparen(ranged.X).(*ast.CallExpr)
	if !ok {
		return false, nil
	}
	value := query.Types.TypeOf(ranged.X)
	if value == nil {
		return false, core.ErrGoToolchainOutput
	}
	if _, function := value.Underlying().(*types.Signature); !function {
		return false, nil
	}
	for slot, err := range methodFieldErrorRangeSlots(ctx, ranged, query) {
		if err != nil {
			return false, err
		}
		for next, err := range methodFieldErrorCallBindings(ctx, call, query) {
			if err != nil {
				return false, err
			}
			for instruction, err := range query.View.Instructions(ctx, SSAInstructionRequest{Function: next.Frame, Extent: SSAInstructionExtentFunction}) {
				if err != nil {
					return false, err
				}
				returned, ok := instruction.(*ssa.Return)
				if !ok || len(returned.Results) != 1 || errorUntakenBlock(returned.Block()) {
					continue
				}
				loan, _, err := query.View.sourceFunctionValue(ctx, returned.Results[0], returned, nil)
				if err != nil {
					return false, err
				}
				function := loan.Function
				if function == nil || function.Parent() != next.Frame || len(function.Params) != 1 {
					continue
				}
				literal, ok := function.Syntax().(*ast.FuncLit)
				if !ok {
					continue
				}
				child := next
				child.Parent = &next
				child.Frame, child.Body, child.Yield = function, literal.Body, function.Params[0]
				child.YieldSlot = slot
				if proved, err := methodFieldErrorCovers(ctx, child); proved || err != nil {
					return proved, err
				}
			}
		}
	}
	return false, nil
}

// Each native yielded coordinate is checked independently. Seq2[error,error]
// does not let the second callback parameter stand for a returned first one.
func methodFieldErrorRangeSlots(ctx context.Context, ranged *ast.RangeStmt, query methodFieldErrorQuery) iter.Seq2[int, error] {
	return func(yield func(int, error) bool) {
		for slot, expression := range [2]ast.Expr{ranged.Key, ranged.Value} {
			if expression == nil {
				continue
			}
			value := query.Types.TypeOf(expression)
			if value == nil {
				if id, ok := expression.(*ast.Ident); ok && id.Name == "_" {
					continue
				}
				yield(0, core.ErrGoToolchainOutput)
				return
			}
			if !types.AssignableTo(value, types.Universe.Lookup("error").Type()) {
				continue
			}
			for _, function := range query.Frame.AnonFuncs {
				if function == nil {
					yield(0, core.ErrGoToolchainOutput)
					return
				}
				if function.Syntax() != ranged || function.Parent() != query.Frame {
					continue
				}
				if slot >= len(function.Params) || len(function.Blocks) == 0 {
					yield(0, core.ErrGoToolchainOutput)
					return
				}
				for _, err := range query.View.FunctionAncestors(ctx, SSAFunctionAncestorsRequest{Function: function}) {
					if err != nil {
						yield(0, err)
						return
					}
				}
				proved, err := errorReturnRegion(ctx, errorIdentityQuery{View: query.View, Analysis: query.Analysis, Frame: function, Origin: function.Params[slot]}, function.Blocks[0])
				if err != nil {
					yield(0, err)
					return
				}
				if proved && !yield(slot, nil) {
					return
				}
			}
		}
	}
}

// A non-nil cause must reach the exact yielded error coordinate before every
// normal return in its native region. Nil arms retain the existing neutral rule.
func methodFieldErrorYieldCall(ctx context.Context, expression *ast.CallExpr, query methodFieldErrorQuery) (bool, error) {
	value, _ := query.Frame.ValueForExpr(expression)
	origin, ok := value.(*ssa.Call)
	if !ok || errorUntakenBlock(origin.Block()) {
		return false, nil
	}
	reference := errorIdentityQuery{View: query.View, Analysis: query.Analysis, Frame: query.Frame, Origin: origin}
	return methodFieldYieldRegion(ctx, reference, origin.Block(), query.Yield, query.YieldSlot)
}

func methodFieldYieldRegion(ctx context.Context, reference errorIdentityQuery, region *ssa.BasicBlock, yield ssa.Value, slot int) (bool, error) {
	observed := false
	for _, block := range reference.Frame.Blocks {
		if !region.Dominates(block) || errorUntakenBlock(block) {
			continue
		}
		neutral, err := errorNilAt(ctx, &reference, block)
		if err != nil {
			return false, err
		}
		if neutral {
			continue
		}
		for _, successor := range block.Succs {
			if !region.Dominates(successor) && !errorUntakenBlock(successor) {
				nilArm, err := errorNilSuccessor(ctx, &reference, block)
				if err != nil || nilArm != successor {
					return false, err
				}
			}
		}
		for _, instruction := range block.Instrs {
			returned, ok := instruction.(*ssa.Return)
			if !ok {
				continue
			}
			observed = true
			carried := false
			for sink, err := range reference.View.Instructions(ctx, SSAInstructionRequest{Function: reference.Frame, Extent: SSAInstructionExtentFunction}) {
				if err != nil {
					return false, err
				}
				call, ok := sink.(*ssa.Call)
				if !ok || !call.Block().Dominates(returned.Block()) || errorUntakenBlock(call.Block()) {
					continue
				}
				carried, err = methodFieldYieldCallCarries(ctx, call, reference, yield, slot)
				if err != nil {
					return false, err
				}
				if carried {
					break
				}
			}
			if !carried {
				return false, nil
			}
		}
	}
	return observed, nil
}

// An invoked local helper borrows its actual error parameter and callback
// capture. Names, unused closures and a neighboring callback lend no authority.
func methodFieldYieldCallCarries(ctx context.Context, call *ssa.Call, reference errorIdentityQuery, yield ssa.Value, slot int) (bool, error) {
	common := call.Common()
	if _, static := common.Value.(*ssa.Function); !static && common.Value.Parent() == reference.Frame {
		matched, err := reference.View.ArgumentOperand(ctx, SSAArgumentOperandRequest{Function: reference.Frame, Value: common.Value, Argument: yield})
		if err != nil {
			return false, err
		}
		if matched.Proven {
			if slot < 0 || slot >= len(common.Args) {
				return false, core.ErrGoToolchainOutput
			}
			return errorResultPreserves(ctx, common.Args[slot], &reference)
		}
	}
	loan, closure, err := reference.View.sourceFunctionValue(ctx, common.Value, call, nil)
	if err != nil {
		return false, err
	}
	function := loan.Function
	if function == nil || function.Parent() != reference.Frame || closure == nil || len(function.Blocks) == 0 {
		return false, nil
	}
	if len(closure.Bindings) != len(function.FreeVars) || len(common.Args) != len(function.Params) {
		return false, core.ErrGoToolchainOutput
	}
	for ancestor := &reference; ancestor != nil; ancestor = ancestor.Parent {
		if ancestor.Frame == function {
			return false, nil
		}
	}
	var callback ssa.Value
	for binding, err := range reference.View.ClosureBindings(ctx, SSAClosureBindingsRequest{Closure: closure, Argument: yield}) {
		if err != nil {
			return false, err
		}
		if binding.Function == function {
			callback = binding.Local
		}
	}
	for index, argument := range common.Args {
		if errorSameValue(argument, yield) {
			callback = function.Params[index]
		}
	}
	if callback == nil {
		return false, nil
	}
	for index, argument := range common.Args {
		preserved, err := errorResultPreserves(ctx, argument, &reference)
		if err != nil {
			return false, err
		}
		if !preserved {
			continue
		}
		next := errorIdentityQuery{View: reference.View, Analysis: reference.Analysis, Frame: function, Origin: function.Params[index], Parent: &reference}
		if preserved, err := methodFieldYieldRegion(ctx, next, function.Blocks[0], callback, slot); preserved || err != nil {
			return preserved, err
		}
	}
	return false, nil
}
