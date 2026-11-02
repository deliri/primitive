package gotoolchain

import (
	"context"
	"errors"
	"go/ast"
	"go/token"
	"go/types"
	"iter"

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
	Yield     ssa.Value
	YieldSlot int
	Frame     *ssa.Function
	View      SSAUnitView
	Variable  *types.Var
	Field     *types.Var
	Body      *ast.BlockStmt
	Parent    *methodFieldErrorQuery
	Analysis  PackageAnalysis
	Types     *types.Info
	Target    *types.Func
	Index     int
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
			if !propagation.Preserved && query.Yield != nil {
				propagation.Preserved, err = methodFieldErrorYieldCall(ctx, value, query)
				if err != nil {
					return false, err
				}
			}
			if !propagation.Preserved {
				continue
			}
			receiver := methodFieldErrorReceiver(value, query)
			if receiver != nil {
				for binding, err := range methodFieldErrorBind(ctx, receiver, query) {
					if err != nil {
						return false, err
					}
					if binding.Field == nil {
						return true, nil
					}
				}
			}
			if covered, err := methodFieldErrorHelperCovers(ctx, value, query); covered || err != nil {
				return covered, err
			}
		case *ast.RangeStmt:
			if covered, err := methodFieldErrorIterator(ctx, value, query); covered || err != nil {
				return covered, err
			}
			covered, operandErr := methodFieldErrorObservedOperand(ctx, value.X, query)
			if operandErr != nil {
				return false, operandErr
			}
			if !covered || value.Value == nil {
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

// Go's source references lend the exact loaded value of a field. Copies keep
// that value; a whole-record copy may also have a compiler home, whose existing
// stability query rejects intervening writes and escapes. No alias history is
// reconstructed and no source or instruction population is retained.
func methodFieldErrorObservedOperand(ctx context.Context, expression ast.Expr, query methodFieldErrorQuery) (bool, error) {
	if methodFieldErrorOperand(expression, query) {
		return true, nil
	}
	if query.Frame == nil || query.Frame.Pkg != query.View.Package {
		return false, nil
	}
	value, address, err := sourceExpressionValue(query.Frame, expression)
	if err != nil || value == nil || address {
		return false, err
	}
	if _, function := value.(*ssa.Function); function {
		return false, nil
	}
	for instruction, err := range query.View.Instructions(ctx, SSAInstructionRequest{Function: query.Frame, Extent: SSAInstructionExtentFunction}) {
		if err != nil {
			return false, err
		}
		reference, ok := instruction.(*ssa.DebugRef)
		if !ok || reference.IsAddr || reference.Expr == nil {
			continue
		}
		if methodFieldErrorOperand(reference.Expr, query) {
			observation, err := query.View.ArgumentOperand(ctx, SSAArgumentOperandRequest{Function: query.Frame, Value: value, Argument: reference.X})
			if err != nil || observation.Proven {
				return observation.Proven, err
			}
		}
		if query.Field != nil && methodFieldOperand(reference.Expr, query.Variable, query.Types) {
			if covered, err := methodFieldErrorStoredRecord(ctx, value, reference.X, query); covered || err != nil {
				return covered, err
			}
		}
	}
	return false, nil
}

func methodFieldErrorStoredRecord(ctx context.Context, value, record ssa.Value, query methodFieldErrorQuery) (bool, error) {
	read, ok := value.(ssa.Instruction)
	if !ok {
		return false, nil
	}
	for instruction, err := range query.View.Instructions(ctx, SSAInstructionRequest{Function: query.Frame, Extent: SSAInstructionExtentFunction}) {
		if err != nil {
			return false, err
		}
		store, ok := instruction.(*ssa.Store)
		if !ok || store.Val != record {
			continue
		}
		home, ok := store.Addr.(*ssa.Alloc)
		if !ok || home.Heap {
			continue
		}
		field := [1]int{query.Index}
		observation, err := query.View.ArgumentOperand(ctx, SSAArgumentOperandRequest{Function: query.Frame, Value: value, Argument: home, Path: field[:]})
		if err != nil {
			return false, err
		}
		if observation.Proven {
			return receiverGateHomeStable(ctx, query.View, home, store, read)
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

func methodFieldErrorBind(ctx context.Context, expression ast.Expr, query methodFieldErrorQuery) iter.Seq2[methodFieldErrorQuery, error] {
	return func(yield func(methodFieldErrorQuery, error) bool) {
		next := query
		next.Parent = &query
		if query.Field != nil && methodFieldOperand(expression, query.Variable, query.Types) {
			yield(next, nil) // A whole-record conversion preserves its native field slot.
			return
		}
		covered, operandErr := methodFieldErrorObservedOperand(ctx, expression, query)
		if operandErr != nil {
			yield(methodFieldErrorQuery{}, operandErr)
			return
		}
		if covered {
			next.Field, next.Index = nil, 0
			yield(next, nil)
			return
		}
		literal, ok := ast.Unparen(expression).(*ast.CompositeLit)
		if !ok {
			for returned, err := range methodFieldErrorReturnedBindings(ctx, expression, query) {
				if !yield(returned, err) || err != nil {
					return
				}
			}
			return
		}
		nativeType := query.Types.TypeOf(literal)
		if nativeType == nil {
			yield(methodFieldErrorQuery{}, core.ErrGoToolchainOutput)
			return
		}
		record := methodFieldRecord(nativeType)
		if record == nil {
			return
		}
		// Validate every native slot before allowing an earlier field to lend
		// authority. A damaged later key must not be hidden by early success.
		for ordinal, element := range literal.Elts {
			if err := temporal.Wait(temporal.WaitRequest{Context: ctx}); err != nil {
				yield(methodFieldErrorQuery{}, err)
				return
			}
			if _, _, err := methodFieldConstructedElement(ordinal, element, record, query.Types); err != nil {
				yield(methodFieldErrorQuery{}, err)
				return
			}
		}
		for ordinal, element := range literal.Elts {
			if err := temporal.Wait(temporal.WaitRequest{Context: ctx}); err != nil {
				yield(methodFieldErrorQuery{}, err)
				return
			}
			index, value, err := methodFieldConstructedElement(ordinal, element, record, query.Types)
			if err != nil {
				yield(methodFieldErrorQuery{}, err)
				return
			}
			covered, operandErr := methodFieldErrorObservedOperand(ctx, value, query)
			if operandErr != nil {
				yield(methodFieldErrorQuery{}, operandErr)
				return
			}
			if !covered {
				continue
			}
			next.Field, next.Index = record.Field(index), index
			if !yield(next, nil) {
				return
			}
		}
	}
}

// Native field objects, rather than key spelling or literal order, own keyed
// record slots. Unkeyed construction uses Go's declared field ordinal.
func methodFieldConstructedElement(ordinal int, element ast.Expr, record *types.Struct, facts *types.Info) (int, ast.Expr, error) {
	index, value := ordinal, element
	if keyed, ok := element.(*ast.KeyValueExpr); ok {
		key, ok := ast.Unparen(keyed.Key).(*ast.Ident)
		if !ok {
			return 0, nil, core.ErrGoToolchainOutput
		}
		field, ok := facts.Uses[key].(*types.Var)
		if !ok || !field.IsField() {
			return 0, nil, core.ErrGoToolchainOutput
		}
		selected, indices, _ := types.LookupFieldOrMethod(record, true, field.Pkg(), key.Name)
		if selected != field || len(indices) != 1 {
			return 0, nil, core.ErrGoToolchainOutput
		}
		index, value = indices[0], keyed.Value
	}
	if index < 0 || index >= record.NumFields() {
		return 0, nil, core.ErrGoToolchainOutput
	}
	return index, value, nil
}

func methodFieldErrorHelperCovers(ctx context.Context, call *ast.CallExpr, query methodFieldErrorQuery) (bool, error) {
	for next, err := range methodFieldErrorCallBindings(ctx, call, query) {
		if err != nil {
			return false, err
		}
		if covered, err := methodFieldErrorCovers(ctx, next); covered || err != nil {
			return covered, err
		}
	}
	return false, nil
}

func methodFieldErrorCallBindings(ctx context.Context, call *ast.CallExpr, query methodFieldErrorQuery) iter.Seq2[methodFieldErrorQuery, error] {
	return func(yield func(methodFieldErrorQuery, error) bool) {
		if query.Frame == nil {
			yield(methodFieldErrorQuery{}, core.ErrGoToolchainOutput)
			return
		}
		common := methodFieldCallAt(query.Frame, call)
		if common == nil {
			return
		}
		function := methodFieldCallFunction(common)
		if function == nil {
			return
		}
		loan, err := query.Analysis.FunctionSource(ctx, FunctionSourceRequest{Function: function})
		if err != nil {
			yield(methodFieldErrorQuery{}, err)
			return
		}
		if loan.Presence != FunctionSourcePresent {
			return
		}
		native, ok := loan.Types.Defs[loan.Declaration.Name].(*types.Func)
		if !ok {
			yield(methodFieldErrorQuery{}, core.ErrGoToolchainOutput)
			return
		}
		signature, ok := native.Type().(*types.Signature)
		if !ok {
			yield(methodFieldErrorQuery{}, core.ErrGoToolchainOutput)
			return
		}
		body, facts := loan.Declaration.Body, loan.Types
		if body == nil || signature == nil {
			return
		}
		offset := 0
		if selector, ok := ast.Unparen(call.Fun).(*ast.SelectorExpr); ok {
			if selection := query.Types.Selections[selector]; selection != nil {
				receiver := selector.X
				if selection.Kind() == types.MethodExpr {
					if len(call.Args) == 0 {
						return
					}
					receiver, offset = call.Args[0], 1
				}
				for next, bindErr := range methodFieldErrorBind(ctx, receiver, query) {
					if bindErr != nil {
						yield(methodFieldErrorQuery{}, bindErr)
						return
					}
					if signature.Recv() != nil && len(selection.Index()) == 1 {
						next.Variable, next.Body, next.Types, next.Frame = signature.Recv(), body, facts, query.View.Program.FuncValue(native)
						if !yield(next, nil) {
							return
						}
					}
				}
			}
		}
		for index, argument := range call.Args[offset:] {
			if index >= signature.Params().Len() {
				continue
			}
			for next, bindErr := range methodFieldErrorBind(ctx, argument, query) {
				if bindErr != nil {
					yield(methodFieldErrorQuery{}, bindErr)
					return
				}
				next.Variable, next.Body, next.Types, next.Frame = signature.Params().At(index), body, facts, query.View.Program.FuncValue(native)
				if !yield(next, nil) {
					return
				}
			}
		}
	}
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
