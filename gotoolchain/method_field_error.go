package gotoolchain

import (
	"context"
	"errors"
	"go/ast"
	"go/token"
	"go/types"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
	"golang.org/x/tools/go/ssa"
)

// MethodFieldErrorPropagationRequest borrows a record method, one receiver field
// and the exact method selected by its caller on that field's terminal record.
// Pointer, slice, array and map elements retain the native field origin. Method
// names and product validation requirements have no meaning to this query.
type MethodFieldErrorPropagationRequest struct {
	Analysis PackageAnalysis
	Method   *types.Func
	Target   *types.Func
	Field    uint64
}

func (r MethodFieldErrorPropagationRequest) Validate() error {
	if r.Method == nil || r.Method.Pkg() == nil || r.Method.Signature().Recv() == nil || r.Target == nil || r.Target.Pkg() == nil || r.Target.Signature().Recv() == nil {
		return core.ErrGoToolchainContract
	}
	record := methodFieldRecord(r.Method.Signature().Recv().Type())
	if record == nil || r.Field >= uint64(record.NumFields()) {
		return core.ErrGoToolchainContract
	}
	leaf := methodFieldErrorLeaf(record.Field(int(r.Field)).Type())
	if leaf == nil {
		return core.ErrGoToolchainContract
	}
	selected := types.NewMethodSet(types.NewPointer(leaf)).Lookup(r.Target.Pkg(), r.Target.Name())
	if selected == nil {
		return core.ErrGoToolchainContract
	}
	target, ok := selected.Obj().(*types.Func)
	if !ok || target.Origin() != r.Target.Origin() {
		return core.ErrGoToolchainContract
	}
	return r.Analysis.Validate()
}
func (MethodFieldErrorPropagationRequest) goToolchainInternalFlow() {}

// MethodFieldErrorPropagationObservation reports a source-identified call on the
// selected field whose native error cause is returned by the enclosing method.
// It is an observed call/error-flow floor, not proof of every path or invariant.
// Unavailable bodies and opaque dynamic targets grant no propagation authority.
type MethodFieldErrorPropagationObservation struct {
	Source     FunctionSourcePresence
	Propagated bool
}

func (o MethodFieldErrorPropagationObservation) Validate() error {
	if err := o.Source.Validate(); err != nil {
		return err
	}
	if o.Source == FunctionSourceAbsent && o.Propagated {
		return core.ErrGoToolchainContract
	}
	return nil
}
func (MethodFieldErrorPropagationObservation) goToolchainProtocolFact() {}

func (v SSAUnitView) MethodFieldErrorPropagation(ctx context.Context, request MethodFieldErrorPropagationRequest) (observation MethodFieldErrorPropagationObservation, resultErr error) {
	defer func() {
		resultErr = errors.Join(resultErr, temporal.Wait(temporal.WaitRequest{Context: ctx}))
		if resultErr != nil {
			observation = MethodFieldErrorPropagationObservation{}
		}
	}()
	if err := errors.Join(v.errorAnalysis(ctx, request.Analysis), request.Validate()); err != nil {
		return observation, err
	}
	loan, err := request.Analysis.FunctionSource(ctx, FunctionSourceRequest{Function: request.Method})
	if err != nil {
		return observation, err
	}
	if loan.Presence == FunctionSourceAbsent {
		return MethodFieldErrorPropagationObservation{Source: FunctionSourceAbsent}, nil
	}
	native, ok := loan.Types.Defs[loan.Declaration.Name].(*types.Func)
	if !ok || native.Pkg() != v.Source.Types {
		return observation, core.ErrGoToolchainContract
	}
	signature := native.Signature()
	if signature.Recv() == nil {
		return observation, core.ErrGoToolchainOutput
	}
	frame := v.Program.FuncValue(native)
	if loan.Declaration.Body != nil && frame == nil {
		return observation, core.ErrGoToolchainOutput
	}
	propagated, err := methodFieldErrorCovers(ctx, methodFieldErrorQuery{View: v, Frame: frame, Analysis: request.Analysis, Types: loan.Types, Body: loan.Declaration.Body, Variable: signature.Recv(), Field: methodFieldRecord(signature.Recv().Type()).Field(int(request.Field)), Index: int(request.Field), Target: request.Target})
	if err != nil {
		return observation, err
	}
	return MethodFieldErrorPropagationObservation{Source: FunctionSourcePresent, Propagated: propagated}, nil
}

type methodFieldErrorQuery struct {
	Frame    *ssa.Function
	View     SSAUnitView
	Variable *types.Var
	Field    *types.Var
	Body     *ast.BlockStmt
	Parent   *methodFieldErrorQuery
	Analysis PackageAnalysis
	Types    *types.Info
	Target   *types.Func
	Index    int
}

func (methodFieldErrorQuery) goToolchainInternalFlow() {}

func methodFieldErrorCovers(ctx context.Context, query methodFieldErrorQuery) (bool, error) {
	if query.Body == nil || query.Variable == nil || query.Types == nil {
		return false, nil
	}
	for parent := query.Parent; parent != nil; parent = parent.Parent {
		if parent.Body == query.Body && parent.Variable == query.Variable && parent.Field == query.Field && parent.Index == query.Index {
			return false, nil
		}
	}
	for syntax, err := range WalkSyntax(ctx, SyntaxWalkRequest{Root: query.Body}) {
		if err != nil {
			return false, err
		}
		closureBody := false
		for _, ancestor := range syntax.Ancestors {
			if _, ok := ancestor.(*ast.FuncLit); ok {
				closureBody = true
				break
			}
		}
		if closureBody {
			continue
		}
		switch value := syntax.Node.(type) {
		case *ast.CallExpr:
			propagation, err := query.View.ErrorPropagation(ctx, ErrorPropagationRequest{Analysis: query.Analysis, Call: value})
			if err != nil {
				return false, err
			}
			if !propagation.Preserved {
				continue
			}
			receiver := methodFieldErrorReceiver(value, query)
			if receiver != nil && methodFieldErrorOperand(receiver, query) {
				return true, nil
			}
			if covered, err := methodFieldErrorHelperCovers(ctx, value, query); covered || err != nil {
				return covered, err
			}
		case *ast.RangeStmt:
			if !methodFieldErrorOperand(value.X, query) || value.Value == nil {
				continue
			}
			variable := methodFieldVariable(value.Value, query.Types)
			if variable != nil {
				if covered, err := methodFieldErrorCovers(ctx, methodFieldErrorQuery{View: query.View, Frame: query.Frame, Analysis: query.Analysis, Types: query.Types, Body: value.Body, Variable: variable, Target: query.Target, Parent: &query}); covered || err != nil {
					return covered, err
				}
			}
		}
	}
	return false, nil
}

func methodFieldErrorOperand(expression ast.Expr, query methodFieldErrorQuery) bool {
	if query.Field == nil {
		return methodFieldVariable(expression, query.Types) == query.Variable
	}
	switch value := ast.Unparen(expression).(type) {
	case *ast.StarExpr:
		return methodFieldErrorOperand(value.X, query)
	case *ast.UnaryExpr:
		return value.Op == token.AND && methodFieldErrorOperand(value.X, query)
	case *ast.SelectorExpr:
		selection := query.Types.Selections[value]
		return selection != nil && selection.Kind() == types.FieldVal && len(selection.Index()) != 0 && selection.Index()[0] == query.Index && methodFieldOperand(value.X, query.Variable, query.Types)
	}
	return false
}

func methodFieldErrorBind(expression ast.Expr, query methodFieldErrorQuery) (methodFieldErrorQuery, bool) {
	next := query
	next.Parent = &query
	if query.Field != nil && methodFieldOperand(expression, query.Variable, query.Types) {
		return next, true // The native whole-record layout preserves the field slot.
	}
	if methodFieldErrorOperand(expression, query) {
		next.Field, next.Index = nil, 0
		return next, true
	}
	return methodFieldErrorQuery{}, false
}

func methodFieldErrorHelperCovers(ctx context.Context, call *ast.CallExpr, query methodFieldErrorQuery) (bool, error) {
	if query.Frame == nil {
		return false, core.ErrGoToolchainOutput
	}
	common := methodFieldCallAt(query.Frame, call)
	if common == nil {
		return false, nil
	}
	function := methodFieldCallFunction(common)
	if function == nil {
		return false, nil
	}
	loan, err := query.Analysis.FunctionSource(ctx, FunctionSourceRequest{Function: function})
	if err != nil {
		return false, err
	}
	if loan.Presence != FunctionSourcePresent {
		return false, nil
	}
	native, ok := loan.Types.Defs[loan.Declaration.Name].(*types.Func)
	if !ok {
		return false, core.ErrGoToolchainOutput
	}
	signature, ok := native.Type().(*types.Signature)
	if !ok {
		return false, core.ErrGoToolchainOutput
	}
	body, facts := loan.Declaration.Body, loan.Types
	if body == nil || signature == nil {
		return false, nil
	}
	offset := 0
	if selector, ok := ast.Unparen(call.Fun).(*ast.SelectorExpr); ok {
		if selection := query.Types.Selections[selector]; selection != nil {
			receiver := selector.X
			if selection.Kind() == types.MethodExpr {
				if len(call.Args) == 0 {
					return false, nil
				}
				receiver, offset = call.Args[0], 1
			}
			if next, matched := methodFieldErrorBind(receiver, query); matched && signature.Recv() != nil {
				if len(selection.Index()) == 1 {
					next.Variable, next.Body, next.Types, next.Frame = signature.Recv(), body, facts, query.View.Program.FuncValue(native)
					if covered, err := methodFieldErrorCovers(ctx, next); covered || err != nil {
						return covered, err
					}
				}
			}
		}
	}
	for index, argument := range call.Args[offset:] {
		if index >= signature.Params().Len() {
			continue
		}
		if next, matched := methodFieldErrorBind(argument, query); matched {
			next.Variable, next.Body, next.Types, next.Frame = signature.Params().At(index), body, facts, query.View.Program.FuncValue(native)
			if covered, err := methodFieldErrorCovers(ctx, next); covered || err != nil {
				return covered, err
			}
		}
	}
	return false, nil
}

func methodFieldErrorReceiver(call *ast.CallExpr, query methodFieldErrorQuery) ast.Expr {
	selector, ok := ast.Unparen(call.Fun).(*ast.SelectorExpr)
	if !ok {
		return nil
	}
	selected := query.Types.Selections[selector]
	if selected == nil {
		return nil
	}
	target, ok := selected.Obj().(*types.Func)
	if !ok || target.Origin() != query.Target.Origin() {
		return nil
	}
	if selected.Kind() == types.MethodVal {
		return selector.X
	}
	if selected.Kind() == types.MethodExpr && len(call.Args) > 0 {
		return call.Args[0]
	}
	return nil
}

func methodFieldVariable(expression ast.Expr, facts *types.Info) *types.Var {
	switch value := ast.Unparen(expression).(type) {
	case *ast.Ident:
		variable, _ := facts.ObjectOf(value).(*types.Var)
		return variable
	case *ast.StarExpr:
		return methodFieldVariable(value.X, facts)
	case *ast.UnaryExpr:
		if value.Op == token.AND {
			return methodFieldVariable(value.X, facts)
		}
	}
	return nil
}

func methodFieldErrorLeaf(value types.Type) types.Type {
	slow, fast := value, value
	for value != nil {
		value = types.Unalias(value)
		if _, record := value.Underlying().(*types.Struct); record {
			return value
		}
		value = methodFieldErrorElement(value)
		slow = methodFieldErrorElement(slow)
		fast = methodFieldErrorElement(methodFieldErrorElement(fast))
		if slow != nil && slow == fast {
			return nil
		}
	}
	return nil
}
func methodFieldErrorElement(value types.Type) types.Type {
	if value == nil {
		return nil
	}
	switch backing := types.Unalias(value).Underlying().(type) {
	case *types.Pointer:
		return backing.Elem()
	case *types.Slice:
		return backing.Elem()
	case *types.Array:
		return backing.Elem()
	case *types.Map:
		return backing.Elem()
	}
	return nil
}
