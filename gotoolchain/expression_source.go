package gotoolchain

import (
	"context"
	"errors"
	"go/ast"
	"go/types"
	"reflect"

	pc "github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
	"golang.org/x/tools/go/ast/astutil"
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
		path, _ := astutil.PathEnclosingInterval(file, expression.Pos(), expression.End())
		if len(path) == 0 || path[0] != expression {
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
		frame := ssa.EnclosingFunction(v.Package, path[len(path)-3:])
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
