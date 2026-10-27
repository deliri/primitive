package gotoolchain

import (
	"context"
	"go/ast"
	"iter"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
	"golang.org/x/tools/go/ssa"
)

// Returned values borrow the actual producer, selected result ordinal and
// parameter origin. A getter's spelling or a read it discards grants nothing.
func methodFieldErrorReturnedBindings(ctx context.Context, expression ast.Expr, query methodFieldErrorQuery) iter.Seq2[methodFieldErrorQuery, error] {
	return func(yield func(methodFieldErrorQuery, error) bool) {
		if query.Frame == nil || query.Frame.Pkg != query.View.Package {
			return
		}
		value, address, err := sourceExpressionValue(query.Frame, expression)
		if err != nil {
			yield(methodFieldErrorQuery{}, err)
			return
		}
		if value == nil || address {
			return
		}
		producer, err := query.View.ExpressionCall(ctx, SSAExpressionCallRequest{Function: query.Frame, Expression: expression})
		if err != nil {
			yield(methodFieldErrorQuery{}, err)
			return
		}
		if producer.Call == nil {
			return
		}
		for syntax, err := range WalkSyntax(ctx, SyntaxWalkRequest{Root: query.Body}) {
			if err != nil {
				yield(methodFieldErrorQuery{}, err)
				return
			}
			call, ok := syntax.Node.(*ast.CallExpr)
			if !ok || methodFieldCallAt(query.Frame, call) != producer.Call.Common() {
				continue
			}
			for binding, err := range methodFieldErrorCallBindings(ctx, call, query) {
				if err != nil {
					yield(methodFieldErrorQuery{}, err)
					return
				}
				repeated := false
				for parent := binding.Parent; parent != nil; parent = parent.Parent {
					if parent.Body == binding.Body && parent.Variable == binding.Variable && parent.Field == binding.Field && parent.Index == binding.Index {
						repeated = true
						break
					}
				}
				if repeated || binding.Frame == nil || binding.Frame.Pkg != query.View.Package {
					continue
				}
				for returned, err := range WalkSyntax(ctx, SyntaxWalkRequest{Root: binding.Body}) {
					if err != nil {
						yield(methodFieldErrorQuery{}, err)
						return
					}
					statement, ok := returned.Node.(*ast.ReturnStmt)
					if !ok || len(statement.Results) != binding.Frame.Signature.Results().Len() || producer.ResultIndex >= uint64(len(statement.Results)) {
						continue
					}
					closure := false
					for _, ancestor := range returned.Ancestors {
						if _, ok := ancestor.(*ast.FuncLit); ok {
							closure = true
							break
						}
					}
					if closure {
						continue
					}
					selected := statement.Results[producer.ResultIndex]
					next := query
					next.Parent = &binding
					covered, err := methodFieldErrorReturnedOperand(ctx, selected, binding)
					if err != nil {
						yield(methodFieldErrorQuery{}, err)
						return
					}
					if covered {
						next.Field, next.Index = nil, 0
						if !yield(next, nil) {
							return
						}
						continue
					}
					literal, ok := ast.Unparen(selected).(*ast.CompositeLit)
					if !ok {
						continue
					}
					nativeType := binding.Types.TypeOf(literal)
					if nativeType == nil {
						yield(methodFieldErrorQuery{}, core.ErrGoToolchainOutput)
						return
					}
					record := methodFieldRecord(nativeType)
					if record == nil {
						continue
					}
					for ordinal, element := range literal.Elts {
						if err := temporal.Wait(temporal.WaitRequest{Context: ctx}); err != nil {
							yield(methodFieldErrorQuery{}, err)
							return
						}
						if _, _, err := methodFieldConstructedElement(ordinal, element, record, binding.Types); err != nil {
							yield(methodFieldErrorQuery{}, err)
							return
						}
					}
					for ordinal, element := range literal.Elts {
						index, field, err := methodFieldConstructedElement(ordinal, element, record, binding.Types)
						if err != nil {
							yield(methodFieldErrorQuery{}, err)
							return
						}
						covered, err := methodFieldErrorReturnedOperand(ctx, field, binding)
						if err != nil {
							yield(methodFieldErrorQuery{}, err)
							return
						}
						if covered {
							next.Field, next.Index = record.Field(index), index
							if !yield(next, nil) {
								return
							}
						}
					}
				}
			}
			return
		}
	}
}

// Compare with the original native parameter, never a same-spelled later
// field read. The existing argument loan rejects overwritten copies and paths.
func methodFieldErrorReturnedOperand(ctx context.Context, expression ast.Expr, query methodFieldErrorQuery) (bool, error) {
	value, address, err := sourceExpressionValue(query.Frame, expression)
	if err != nil || value == nil || address {
		return false, err
	}
	var parameter *ssa.Parameter
	for _, candidate := range query.Frame.Params {
		if candidate.Object() == query.Variable {
			parameter = candidate
			break
		}
	}
	if parameter == nil {
		return false, core.ErrGoToolchainOutput
	}
	var path []int
	field := [1]int{query.Index}
	if query.Field != nil {
		path = field[:]
	}
	observation, err := query.View.ArgumentOperand(ctx, SSAArgumentOperandRequest{Function: query.Frame, Value: value, Argument: parameter, Path: path})
	if err != nil || observation.Proven || query.Field == nil {
		return observation.Proven, err
	}
	return methodFieldErrorStoredRecord(ctx, value, parameter, query)
}
