package gotoolchain

import (
	"context"
	"errors"
	"go/ast"

	pc "github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
	"golang.org/x/tools/go/ast/astutil"
	"golang.org/x/tools/go/ssa"
)

// ExpressionNilRequest borrows an exact expression from the native compilation.
// No expression, alias or value model is copied out of Go's compiler objects.
type ExpressionNilRequest struct {
	Analysis   PackageAnalysis
	Expression ast.Expr
}

func (r ExpressionNilRequest) Validate() error {
	if r.Expression == nil {
		return pc.ErrGoToolchainContract
	}
	return r.Analysis.Validate()
}
func (ExpressionNilRequest) goToolchainInternalFlow() {}

// ExpressionNilObservation reports only a compiler-proven nil value. False
// includes opaque runtime values; a typed nil inside an interface is not nil.
type ExpressionNilObservation struct{ Proven bool }

func (ExpressionNilObservation) Validate() error          { return nil }
func (ExpressionNilObservation) goToolchainProtocolFact() {}

// ExpressionNil uses Go's source lookup and SSA constants. SourceValues must be
// enabled for nonconstant values. Missing native source metadata is a refusal,
// never a negative observation. The loan retains no query population.
func (v SSAUnitView) ExpressionNil(ctx context.Context, request ExpressionNilRequest) (ExpressionNilObservation, error) {
	if err := errors.Join(temporal.Wait(temporal.WaitRequest{Context: ctx}), v.Validate(), request.Validate()); err != nil {
		return ExpressionNilObservation{}, err
	}
	if request.Analysis.Incomplete {
		return ExpressionNilObservation{}, pc.ErrGoToolchainOutput
	}
	owns := false
	for _, unit := range request.Analysis.Units {
		if unit == v.Source {
			owns = true
			break
		}
	}
	if !owns {
		return ExpressionNilObservation{}, pc.ErrGoToolchainContract
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
			return ExpressionNilObservation{}, pc.ErrGoToolchainOutput
		}
		if facts.IsNil() {
			return ExpressionNilObservation{Proven: true}, nil
		}
		if facts.Value != nil {
			return ExpressionNilObservation{}, nil
		}
		frame := ssa.EnclosingFunction(v.Package, path)
		if frame == nil {
			return ExpressionNilObservation{}, pc.ErrGoToolchainOutput
		}
		value, address := frame.ValueForExpr(expression)
		if value == nil {
			return ExpressionNilObservation{}, pc.ErrGoToolchainOutput
		}
		constant, ok := value.(*ssa.Const)
		return ExpressionNilObservation{Proven: !address && ok && constant.IsNil()}, nil
	}
	return ExpressionNilObservation{}, pc.ErrGoToolchainContract
}
