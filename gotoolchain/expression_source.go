package gotoolchain

import (
	"context"
	"errors"
	"go/ast"
	"go/types"
	"reflect"

	pc "github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
	"golang.org/x/tools/go/ssa"
)

// ExpressionRequest borrows an exact expression from this native compilation.
// Nil-value and producer queries share the same source ownership contract.
type ExpressionRequest struct {
	Analysis   PackageAnalysis
	Expression ast.Expr
}

func (r ExpressionRequest) Validate() error {
	if r.Expression == nil || reflect.ValueOf(r.Expression).IsNil() {
		return pc.ErrGoToolchainContract
	}
	return r.Analysis.Validate()
}
func (ExpressionRequest) goToolchainInternalFlow() {}

// Go owns source membership, function nesting and expression values. This
// synchronous lookup retains no expression or value population.
func (v SSAUnitView) expressionValue(ctx context.Context, request ExpressionRequest) (ssa.Value, types.TypeAndValue, bool, error) {
	if err := errors.Join(temporal.Wait(temporal.WaitRequest{Context: ctx}), v.Validate(), request.Validate()); err != nil {
		return nil, types.TypeAndValue{}, false, err
	}
	if request.Analysis.Incomplete {
		return nil, types.TypeAndValue{}, false, pc.ErrGoToolchainOutput
	}
	owns := false
	for _, unit := range request.Analysis.Units {
		if unit == v.Source {
			owns = true
			break
		}
	}
	if !owns {
		return nil, types.TypeAndValue{}, false, pc.ErrGoToolchainContract
	}
	expression := ast.Unparen(request.Expression)
	for _, file := range v.Source.Syntax {
		if expression.Pos() < file.Pos() || expression.End() > file.End() {
			continue
		}
		// Only the three outer nodes select Go's package-level frame. Borrow
		// them from the standard walker; PathEnclosingInterval constructs a
		// sibling slice proportional to a block's complete statement list.
		var path [3]ast.Node
		found := false
		var walkErr error
		ast.PreorderStack(file, nil, func(node ast.Node, ancestors []ast.Node) bool {
			if found || walkErr != nil {
				return false
			}
			if walkErr = temporal.Wait(temporal.WaitRequest{Context: ctx}); walkErr != nil {
				return false
			}
			if node != expression {
				return true
			}
			found = true
			if len(ancestors) < len(path) {
				walkErr = pc.ErrGoToolchainOutput
				return false
			}
			path = [3]ast.Node{ancestors[2], ancestors[1], ancestors[0]}
			return false
		})
		if walkErr != nil {
			return nil, types.TypeAndValue{}, false, walkErr
		}
		if !found {
			continue
		}
		facts, present := v.Source.TypesInfo.Types[expression]
		if !present || facts.Type == nil {
			return nil, types.TypeAndValue{}, false, pc.ErrGoToolchainOutput
		}
		if facts.IsNil() || facts.Value != nil {
			return nil, facts, false, nil
		}
		// Begin at the native package-level frame. EnclosingFunction follows
		// FuncLit syntax alone and omits Go's generated iterator yield frames.
		frame := ssa.EnclosingFunction(v.Package, path[:])
		if frame == nil {
			return nil, types.TypeAndValue{}, false, pc.ErrGoToolchainOutput
		}
		for {
			if value, address := frame.ValueForExpr(expression); value != nil {
				return value, facts, address, nil
			}
			var enclosing *ssa.Function
			for _, child := range frame.AnonFuncs {
				if syntax := child.Syntax(); syntax != nil && syntax.Pos() <= expression.Pos() && expression.End() <= syntax.End() {
					enclosing = child
					break
				}
			}
			if enclosing == nil {
				return nil, types.TypeAndValue{}, false, pc.ErrGoToolchainOutput
			}
			frame = enclosing
		}
	}
	return nil, types.TypeAndValue{}, false, pc.ErrGoToolchainContract
}
