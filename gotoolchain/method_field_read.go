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

// MethodFieldReadRequest borrows a native record method and one receiver field
// slot. The query applies no product validation policy or method-name meaning.
type MethodFieldReadRequest struct {
	Analysis PackageAnalysis
	Method   *types.Func
	Field    uint64
}

func (r MethodFieldReadRequest) Validate() error {
	if r.Method == nil || r.Method.Pkg() == nil || r.Method.Signature().Recv() == nil {
		return core.ErrGoToolchainContract
	}
	record := methodFieldRecord(r.Method.Signature().Recv().Type())
	if record == nil || r.Field >= uint64(record.NumFields()) {
		return core.ErrGoToolchainContract
	}
	return r.Analysis.Validate()
}
func (MethodFieldReadRequest) goToolchainInternalFlow() {}

// MethodFieldReadObservation reports a source-identified field read through
// Go-owned receiver identity, record conversions and resolved helper slots.
// Read is a read floor, not proof of an invariant or all-path execution. Opaque
// imported bodies are absent observations and never acquire read authority.
type MethodFieldReadObservation struct {
	Source FunctionSourcePresence
	Read   bool
}

func (o MethodFieldReadObservation) Validate() error {
	if err := o.Source.Validate(); err != nil {
		return err
	}
	if o.Source == FunctionSourceAbsent && o.Read {
		return core.ErrGoToolchainContract
	}
	return nil
}
func (MethodFieldReadObservation) goToolchainProtocolFact() {}

// MethodFieldRead follows only the active source/helper ancestry. It retains
// no declaration, field-use, call-target or instruction population beyond the
// native compiler graph already borrowed by this view.
func (v SSAUnitView) MethodFieldRead(ctx context.Context, request MethodFieldReadRequest) (observation MethodFieldReadObservation, resultErr error) {
	defer func() {
		resultErr = errors.Join(resultErr, temporal.Wait(temporal.WaitRequest{Context: ctx}))
		if resultErr != nil {
			observation = MethodFieldReadObservation{}
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
		return MethodFieldReadObservation{Source: FunctionSourceAbsent}, nil
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
	read, err := methodFieldReads(ctx, methodFieldReadQuery{View: v, Frame: frame, Analysis: request.Analysis, Types: loan.Types, Body: loan.Declaration.Body, Variable: signature.Recv(), Field: int(request.Field)})
	if err != nil {
		return observation, err
	}
	return MethodFieldReadObservation{Source: FunctionSourcePresent, Read: read}, nil
}

type methodFieldReadQuery struct {
	View     SSAUnitView
	Frame    *ssa.Function
	Variable *types.Var
	Body     *ast.BlockStmt
	Parent   *methodFieldReadQuery
	Analysis PackageAnalysis
	Types    *types.Info
	Field    int
}

func (methodFieldReadQuery) goToolchainInternalFlow() {}

func methodFieldRecord(value types.Type) *types.Struct {
	if value == nil {
		return nil
	}
	value = types.Unalias(value)
	if pointer, ok := value.(*types.Pointer); ok {
		value = pointer.Elem()
	}
	record, _ := value.Underlying().(*types.Struct)
	return record
}

func methodFieldReads(ctx context.Context, query methodFieldReadQuery) (bool, error) {
	if query.Body == nil || query.Variable == nil || query.Types == nil {
		return false, nil
	}
	for parent := query.Parent; parent != nil; parent = parent.Parent {
		if parent.Variable == query.Variable && parent.Field == query.Field && parent.Body == query.Body {
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
		read := false
		switch value := syntax.Node.(type) {
		case *ast.SelectorExpr:
			selection := query.Types.Selections[value]
			read = selection != nil && selection.Kind() == types.FieldVal && len(selection.Index()) != 0 && selection.Index()[0] == query.Field && methodFieldOperand(value.X, query)
		case *ast.BinaryExpr:
			if value.Op == token.EQL || value.Op == token.NEQ {
				read = methodFieldComparedRecord(value.X, query) || methodFieldComparedRecord(value.Y, query)
			}
		case *ast.CallExpr:
			read, err = methodFieldCallReads(ctx, value, query)
			if err != nil {
				return false, err
			}
		}
		if read {
			return true, nil
		}
	}
	return false, nil
}

func methodFieldComparedRecord(expression ast.Expr, query methodFieldReadQuery) bool {
	value := query.Types.TypeOf(expression)
	if value == nil {
		return false
	}
	_, record := value.Underlying().(*types.Struct)
	return record && methodFieldOperand(expression, query)
}

// Only native whole-record conversions preserve the field slot. Constructing a
// fresh record, reading a similarly named local, or converting to an interface
// cannot grant provenance to the original receiver.
func methodFieldOperand(expression ast.Expr, query methodFieldReadQuery) bool {
	switch value := ast.Unparen(expression).(type) {
	case *ast.Ident:
		return query.Types.ObjectOf(value) == query.Variable
	case *ast.StarExpr:
		return methodFieldOperand(value.X, query)
	case *ast.UnaryExpr:
		return value.Op == token.AND && methodFieldOperand(value.X, query)
	case *ast.CallExpr:
		return len(value.Args) == 1 && query.Types.Types[value.Fun].IsType() && methodFieldRecord(query.Types.TypeOf(value)) != nil && methodFieldOperand(value.Args[0], query)
	}
	return false
}

func methodFieldCallReads(ctx context.Context, call *ast.CallExpr, query methodFieldReadQuery) (bool, error) {
	callee := ast.Unparen(call.Fun)
	for {
		switch indexed := callee.(type) {
		case *ast.IndexExpr:
			callee = ast.Unparen(indexed.X)
		case *ast.IndexListExpr:
			callee = ast.Unparen(indexed.X)
		default:
			return methodFieldResolvedCallReads(ctx, call, callee, query)
		}
	}
}

func methodFieldResolvedCallReads(ctx context.Context, call *ast.CallExpr, callee ast.Expr, query methodFieldReadQuery) (bool, error) {
	if query.Frame == nil {
		return false, core.ErrGoToolchainOutput
	}
	common := methodFieldCallAt(query.Frame, call)
	if common == nil {
		return false, nil
	}
	frame := common.StaticCallee()
	var body *ast.BlockStmt
	var signature *types.Signature
	facts := query.Types
	var receiver ast.Expr
	offset := 0
	promotedField := -1
	if closure, ok := callee.(*ast.FuncLit); ok {
		if frame == nil || frame.Syntax() != closure {
			return false, nil
		}
		body = closure.Body
		signature, _ = facts.TypeOf(closure).(*types.Signature)
	} else {
		switch value := callee.(type) {
		case *ast.SelectorExpr:
			if selected := facts.Selections[value]; selected != nil {
				if len(selected.Index()) > 1 {
					promotedField = selected.Index()[0]
				}
				if selected.Kind() == types.MethodVal {
					receiver = value.X
				} else if selected.Kind() == types.MethodExpr && len(call.Args) > 0 {
					receiver, offset = call.Args[0], 1
				}
			}
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
		signature, ok = native.Type().(*types.Signature)
		if !ok {
			return false, core.ErrGoToolchainOutput
		}
		body, facts = loan.Declaration.Body, loan.Types
		frame = query.View.Program.FuncValue(native)
	}
	if signature == nil || body == nil {
		return false, nil
	}
	next := methodFieldReadQuery{View: query.View, Frame: frame, Analysis: query.Analysis, Types: facts, Body: body, Field: query.Field, Parent: &query}
	if receiver != nil && signature.Recv() != nil && methodFieldOperand(receiver, query) {
		if promotedField == query.Field {
			return true, nil
		}
		if promotedField < 0 {
			next.Variable = signature.Recv()
			if read, err := methodFieldReads(ctx, next); read || err != nil {
				return read, err
			}
		}
	}
	for index, argument := range call.Args[offset:] {
		if index < signature.Params().Len() && methodFieldOperand(argument, query) {
			next.Variable = signature.Params().At(index)
			if methodFieldRecord(next.Variable.Type()) != nil {
				if read, err := methodFieldReads(ctx, next); read || err != nil {
					return read, err
				}
			}
		}
	}
	return false, nil
}

func methodFieldCallAt(fn *ssa.Function, call *ast.CallExpr) *ssa.CallCommon {
	value, _ := fn.ValueForExpr(call)
	if instruction, ok := value.(*ssa.Call); ok {
		if errorUntakenBlock(instruction.Block()) {
			return nil
		}
		return instruction.Common()
	}
	for _, block := range fn.Blocks {
		if errorUntakenBlock(block) {
			continue
		}
		for _, instruction := range block.Instrs {
			if invoke, ok := instruction.(ssa.CallInstruction); ok && invoke.Common().Pos() == call.Lparen {
				return invoke.Common()
			}
		}
	}
	return nil
}

func methodFieldCallFunction(common *ssa.CallCommon) *types.Func {
	if common.IsInvoke() {
		return common.Method
	}
	if target := common.StaticCallee(); target != nil {
		function, _ := target.Object().(*types.Func)
		return function
	}
	return nil
}
