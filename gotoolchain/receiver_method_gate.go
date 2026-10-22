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

// ReceiverMethodGateRequest borrows a record method and the exact receiver method
// selected by its caller. No decoder, wire format or validation name is assigned
// meaning here. The query follows native storage, values and error propagation.
type ReceiverMethodGateRequest struct {
	Analysis PackageAnalysis
	Method   *types.Func
	Target   *types.Func
}

func (r ReceiverMethodGateRequest) Validate() error {
	if r.Method == nil || r.Method.Pkg() == nil || r.Method.Signature().Recv() == nil || r.Target == nil || r.Target.Pkg() == nil || r.Target.Signature().Recv() == nil {
		return core.ErrGoToolchainContract
	}
	owner := types.Unalias(r.Method.Signature().Recv().Type())
	if methodFieldRecord(owner) == nil {
		return core.ErrGoToolchainContract
	}
	if pointer, ok := owner.(*types.Pointer); ok {
		owner = pointer.Elem()
	}
	selected := types.NewMethodSet(types.NewPointer(owner)).Lookup(r.Target.Pkg(), r.Target.Name())
	if selected == nil {
		return core.ErrGoToolchainContract
	}
	target, ok := selected.Obj().(*types.Func)
	if !ok || target.Origin() != r.Target.Origin() {
		return core.ErrGoToolchainContract
	}
	return r.Analysis.Validate()
}
func (ReceiverMethodGateRequest) goToolchainInternalFlow() {}

// ReceiverMethodGateObservation is a compiler-established call/value/error-flow
// floor. Preserved means the actual receiver, or the staged value committed to it,
// reaches the selected method without losing its error cause. It does not prove
// a product invariant or arbitrary runtime behavior. Opaque origins grant no gate.
type ReceiverMethodGateObservation struct {
	Source    FunctionSourcePresence
	Preserved bool
}

func (o ReceiverMethodGateObservation) Validate() error {
	if err := o.Source.Validate(); err != nil {
		return err
	}
	if o.Source == FunctionSourceAbsent && o.Preserved {
		return core.ErrGoToolchainContract
	}
	return nil
}
func (ReceiverMethodGateObservation) goToolchainProtocolFact() {}

func (v SSAUnitView) ReceiverMethodGate(ctx context.Context, request ReceiverMethodGateRequest) (observation ReceiverMethodGateObservation, resultErr error) {
	defer func() {
		resultErr = errors.Join(resultErr, temporal.Wait(temporal.WaitRequest{Context: ctx}))
		if resultErr != nil {
			observation = ReceiverMethodGateObservation{}
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
		return ReceiverMethodGateObservation{Source: FunctionSourceAbsent}, nil
	}
	native, ok := loan.Types.Defs[loan.Declaration.Name].(*types.Func)
	if !ok || native.Pkg() != v.Source.Types {
		return observation, core.ErrGoToolchainContract
	}
	if loan.Declaration.Body == nil {
		return ReceiverMethodGateObservation{Source: FunctionSourcePresent}, nil
	}
	frame := v.Program.FuncValue(native)
	if frame == nil || len(frame.Params) == 0 {
		return observation, core.ErrGoToolchainOutput
	}
	preserved, err := receiverGatePreserved(ctx, receiverGateQuery{View: v, Receiver: frame.Params[0], Body: loan.Declaration.Body, Types: loan.Types, Analysis: request.Analysis, Frame: frame, Target: request.Target})
	if err != nil {
		return observation, err
	}
	return ReceiverMethodGateObservation{Source: FunctionSourcePresent, Preserved: preserved}, nil
}

type receiverGateQuery struct {
	Target   *types.Func
	Parent   *receiverGateQuery
	View     SSAUnitView
	Receiver ssa.Value
	Body     *ast.BlockStmt
	Types    *types.Info
	Analysis PackageAnalysis
	Frame    *ssa.Function
}

func (receiverGateQuery) goToolchainInternalFlow() {}

// A helper's spelling cannot grant a validation gate. Follow the actual Go
// objects and require the helper to validate the same value it commits before
// assigning the destination. The error must still satisfy propagation policy.
func receiverGatePreserved(ctx context.Context, query receiverGateQuery) (bool, error) {
	if query.Body == nil || query.Receiver == nil {
		return false, nil
	}
	if query.Frame == nil {
		return false, core.ErrGoToolchainOutput
	}
	for parent := query.Parent; parent != nil; parent = parent.Parent {
		if parent.Frame == query.Frame && parent.Receiver == query.Receiver && parent.Body == query.Body {
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
			if receiver := receiverGateCallReceiver(value, query.Types, query.Target); receiver != nil {
				if star, ok := ast.Unparen(receiver).(*ast.StarExpr); ok {
					receiver = star.X
				}
				if same, err := receiverGateReceiverMatches(ctx, receiver, query); same || err != nil {
					return same, err
				}
			}
			if committed, err := receiverGateHelperCommits(ctx, value, query); committed || err != nil {
				return committed, err
			}
			if committed, err := receiverGateParserHelperCommits(ctx, value, query); committed || err != nil {
				return committed, err
			}
			common := methodFieldCallAt(query.Frame, value)
			if common == nil || methodFieldCallFunction(common) == nil {
				if gated, err := receiverGateLiteral(ctx, value, query); gated || err != nil {
					return gated, err
				}
			}
		case *ast.AssignStmt:
			stage, err := receiverGateAssignedValue(ctx, value, query)
			if err != nil {
				return false, err
			}
			if stage != nil {
				if validated, err := receiverGateValueBefore(ctx, receiverGateValueQuery{View: query.View, Body: query.Body, Types: query.Types, Analysis: query.Analysis, Value: value.Rhs[0], Frame: query.Frame, Target: query.Target}); validated || err != nil {
					return validated, err
				}
			}
		}
	}
	return false, nil
}

// Go's native call target selects the body. An uncalled literal, reassigned
// function variable or merged target cannot borrow another closure's authority.
func receiverGateLiteral(ctx context.Context, call *ast.CallExpr, query receiverGateQuery) (bool, error) {
	calleeType := query.Types.TypeOf(call.Fun)
	if calleeType == nil {
		return false, core.ErrGoToolchainOutput
	}
	if query.Types.Types[call.Fun].IsType() {
		return false, nil
	}
	if _, ok := calleeType.Underlying().(*types.Signature); !ok {
		return false, nil
	}
	loan, err := query.Analysis.LiteralInvocation(ctx, LiteralInvocationRequest{Call: call})
	if err != nil {
		return false, err
	}
	if loan.Presence != FunctionSourcePresent {
		return false, nil
	}
	signature, ok := loan.Types.TypeOf(loan.Literal).(*types.Signature)
	if !ok {
		return false, core.ErrGoToolchainOutput
	}
	common := methodFieldCallAt(query.Frame, call)
	if common == nil || common.StaticCallee() == nil || common.StaticCallee().Syntax() != loan.Literal {
		return false, core.ErrGoToolchainOutput
	}
	next := receiverGateQuery{View: query.View, Receiver: query.Receiver, Body: loan.Literal.Body, Types: loan.Types, Analysis: query.Analysis, Frame: common.StaticCallee(), Target: query.Target, Parent: &query}
	if gated, err := receiverGatePreserved(ctx, next); gated || err != nil {
		return gated, err
	}
	for index, argument := range call.Args {
		if index >= signature.Params().Len() {
			continue
		}
		same, err := receiverGateReceiverMatches(ctx, argument, query)
		if err != nil {
			return false, err
		}
		if !same {
			continue
		}
		if index >= len(next.Frame.Params) {
			return false, core.ErrGoToolchainOutput
		}
		next.Receiver = next.Frame.Params[index]
		if gated, err := receiverGatePreserved(ctx, next); gated || err != nil {
			return gated, err
		}
	}
	return false, nil
}

// Native pointer values preserve copies and reject replacements. Primitive owns
// storage relations, including read-only captured pointer cells.
func receiverGateReceiverMatches(ctx context.Context, expression ast.Expr, query receiverGateQuery) (bool, error) {
	if query.Frame == nil || query.Receiver == nil {
		return false, core.ErrGoToolchainOutput
	}
	expression = ast.Unparen(expression)
	targetType := query.Types.TypeOf(expression)
	if targetType == nil {
		return false, core.ErrGoToolchainOutput
	}
	if !types.Identical(targetType, query.Receiver.Type()) {
		return false, nil
	}
	target, address := query.Frame.ValueForExpr(expression)
	if target == nil || address {
		return false, nil
	}
	if _, pointer := query.Receiver.Type().Underlying().(*types.Pointer); !pointer {
		return target == query.Receiver, nil
	}
	same, err := query.View.SameStorage(ctx, SSAStorageRequest{Left: query.Receiver, Right: target})
	return same.Proven, err
}

func receiverGateCallReceiver(call *ast.CallExpr, facts *types.Info, target *types.Func) ast.Expr {
	selector, ok := ast.Unparen(call.Fun).(*ast.SelectorExpr)
	if !ok {
		return nil
	}
	selected := facts.Selections[selector]
	if selected == nil {
		return nil
	}
	function, ok := selected.Obj().(*types.Func)
	if !ok || function.Origin() != target.Origin() {
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

func receiverGateAssignedValue(ctx context.Context, assignment *ast.AssignStmt, query receiverGateQuery) (ast.Expr, error) {
	if assignment.Tok != token.ASSIGN || len(assignment.Lhs) != 1 || len(assignment.Rhs) != 1 {
		return nil, nil
	}
	star, ok := ast.Unparen(assignment.Lhs[0]).(*ast.StarExpr)
	if !ok {
		return nil, nil
	}
	same, err := receiverGateReceiverMatches(ctx, star.X, query)
	if err != nil || !same {
		return nil, err
	}
	if !types.Identical(query.Types.TypeOf(star), query.Types.TypeOf(assignment.Rhs[0])) {
		return nil, nil
	}
	return assignment.Rhs[0], nil
}

type receiverGateValueQuery struct {
	BoundParameter *types.Var
	Target         *types.Func
	View           SSAUnitView
	Analysis       PackageAnalysis
	Body           *ast.BlockStmt
	Types          *types.Info
	Frame          *ssa.Function
	Value          ast.Expr
}

func (receiverGateValueQuery) goToolchainInternalFlow() {}

// The compiler's immutable value at the commit must be the actual validator
// operand. Variable names and source order cannot lend an earlier value's gate.
func receiverGateValueBefore(ctx context.Context, query receiverGateValueQuery) (bool, error) {
	if query.Frame == nil || query.Value == nil {
		return false, core.ErrGoToolchainOutput
	}
	expression := ast.Unparen(query.Value)
	facts, present := query.Types.Types[expression]
	if !present || facts.Type == nil {
		return false, core.ErrGoToolchainOutput
	}
	current, address := query.Frame.ValueForExpr(expression)
	if current == nil {
		if facts.Value != nil || facts.IsNil() {
			return false, nil
		}
		return false, core.ErrGoToolchainOutput
	}
	if address {
		return false, nil
	}
	use := receiverGateExpressionUse(query.Frame, query.Value)
	if use == nil {
		return false, core.ErrGoToolchainOutput
	}
	for syntax, err := range WalkSyntax(ctx, SyntaxWalkRequest{Root: query.Body}) {
		if err != nil {
			return false, err
		}
		closure := false
		for _, ancestor := range syntax.Ancestors {
			if _, ok := ancestor.(*ast.FuncLit); ok {
				closure = true
				break
			}
		}
		if closure {
			continue
		}
		call, ok := syntax.Node.(*ast.CallExpr)
		if !ok {
			continue
		}
		receiver := receiverGateCallReceiver(call, query.Types, query.Target)
		if receiver == nil {
			receiver = receiverGateBoundCallReceiver(call, query)
		}
		if receiver == nil {
			continue
		}
		candidate, indirect := query.Frame.ValueForExpr(ast.Unparen(receiver))
		if indirect || candidate == nil {
			continue
		}
		same, err := receiverGateSameValue(ctx, query.View, candidate, current)
		if err != nil {
			return false, err
		}
		if !same {
			continue
		}
		value, _ := query.Frame.ValueForExpr(call)
		operation, ok := value.(*ssa.Call)
		if !ok {
			continue
		}
		if errorUntakenBlock(operation.Block()) {
			continue
		}
		before, err := query.View.InstructionDominates(ctx, SSAInstructionOrderRequest{Before: operation, After: use})
		if err != nil {
			return false, err
		}
		if !before.Proven {
			continue
		}
		propagation, err := query.View.ErrorPropagation(ctx, ErrorPropagationRequest{Analysis: query.Analysis, Call: call})
		if propagation.Preserved || err != nil {
			return propagation.Preserved, err
		}
	}
	return false, nil
}

func receiverGateHelperCommits(ctx context.Context, call *ast.CallExpr, query receiverGateQuery) (bool, error) {
	if len(call.Args) != 2 {
		return false, nil
	}
	same, err := receiverGateReceiverMatches(ctx, call.Args[0], query)
	if err != nil || !same {
		return false, err
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
	body := loan.Declaration.Body
	if body == nil || signature.Params().Len() != 2 || !receiverGateTupleMatches(signature.Results(), []types.Type{types.Universe.Lookup("error").Type()}) {
		return false, nil
	}
	destination, value := signature.Params().At(0), signature.Params().At(1)
	pointer, ok := types.Unalias(destination.Type()).(*types.Pointer)
	if !ok || !types.Identical(pointer.Elem(), value.Type()) {
		return false, nil
	}
	frame := query.View.Program.FuncValue(native)
	offset := 0
	if signature.Recv() != nil {
		offset = 1
	}
	if frame == nil || len(frame.Params) <= offset {
		return false, core.ErrGoToolchainOutput
	}
	destinationQuery := receiverGateQuery{View: query.View, Receiver: frame.Params[offset], Body: body, Types: loan.Types, Analysis: query.Analysis, Frame: frame, Target: query.Target}
	for syntax, err := range WalkSyntax(ctx, SyntaxWalkRequest{Root: body}) {
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
		assignment, ok := syntax.Node.(*ast.AssignStmt)
		if !ok {
			continue
		}
		stage, err := receiverGateAssignedValue(ctx, assignment, destinationQuery)
		if err != nil {
			return false, err
		}
		if stage != nil {
			if validated, err := receiverGateValueBefore(ctx, receiverGateValueQuery{View: query.View, Body: body, Types: loan.Types, Analysis: query.Analysis, Value: stage, Frame: frame, Target: query.Target, BoundParameter: receiverGateBoundParameter(common, signature, query.Target)}); validated || err != nil {
				return validated, err
			}
		}
	}
	return false, nil
}

// A debug reference is the compiler's actual source-use instruction. Borrow
// that instruction rather than manufacturing a source-position ordering model.
func receiverGateExpressionUse(frame *ssa.Function, expression ast.Expr) *ssa.DebugRef {
	expression = ast.Unparen(expression)
	for _, block := range frame.Blocks {
		for _, instruction := range block.Instrs {
			if reference, ok := instruction.(*ssa.DebugRef); ok && reference.Expr == expression {
				return reference
			}
		}
	}
	return nil
}

func receiverGateParserHelperCommits(ctx context.Context, call *ast.CallExpr, query receiverGateQuery) (bool, error) {
	if len(call.Args) != 3 {
		return false, nil
	}
	same, err := receiverGateReceiverMatches(ctx, call.Args[0], query)
	if err != nil || !same {
		return false, err
	}
	common := methodFieldCallAt(query.Frame, call)
	if common == nil {
		return false, nil
	}
	helper := methodFieldCallFunction(common)
	if helper == nil {
		return false, nil
	}
	offset := 0
	if helper.Signature().Recv() != nil && !common.IsInvoke() {
		offset = 1
	}
	if len(common.Args) != 3+offset {
		return false, core.ErrGoToolchainOutput
	}
	parserFrame, ok := common.Args[2+offset].(*ssa.Function)
	if !ok {
		return false, nil
	}
	parser, ok := parserFrame.Object().(*types.Func)
	if !ok {
		return false, nil
	}
	helperLoan, err := query.Analysis.FunctionSource(ctx, FunctionSourceRequest{Function: helper})
	if err != nil {
		return false, err
	}
	parserLoan, err := query.Analysis.FunctionSource(ctx, FunctionSourceRequest{Function: parser})
	if err != nil {
		return false, err
	}
	if helperLoan.Presence != FunctionSourcePresent || parserLoan.Presence != FunctionSourcePresent {
		return false, nil
	}
	helperNative, ok := helperLoan.Types.Defs[helperLoan.Declaration.Name].(*types.Func)
	if !ok {
		return false, core.ErrGoToolchainOutput
	}
	parserNative, ok := parserLoan.Types.Defs[parserLoan.Declaration.Name].(*types.Func)
	if !ok {
		return false, core.ErrGoToolchainOutput
	}
	signature, ok := helperNative.Type().(*types.Signature)
	if !ok {
		return false, core.ErrGoToolchainOutput
	}
	parserSignature, ok := parserNative.Type().(*types.Signature)
	if !ok {
		return false, core.ErrGoToolchainOutput
	}
	body, parserBody := helperLoan.Declaration.Body, parserLoan.Declaration.Body
	if body == nil || parserBody == nil || signature.Params().Len() != 3 {
		return false, nil
	}
	destination, callback := signature.Params().At(0), signature.Params().At(2)
	pointer, ok := types.Unalias(destination.Type()).(*types.Pointer)
	if !ok || !receiverGateTupleMatches(parserSignature.Results(), []types.Type{pointer.Elem(), types.Universe.Lookup("error").Type()}) || parserSignature.Params().Len() != 1 {
		return false, nil
	}
	callbackSignature, ok := callback.Type().Underlying().(*types.Signature)
	if !ok || !types.Identical(callbackSignature, parserSignature) {
		return false, nil
	}
	validated, err := receiverGateParserReturnsCheckedValue(ctx, parserBody, parserLoan.Types, parserSignature, query.Analysis, query.View, query.View.Program.FuncValue(parserNative), query.Target)
	if err != nil || !validated {
		return false, err
	}
	frame := query.View.Program.FuncValue(helperNative)
	if frame == nil || len(frame.Params) <= offset {
		return false, core.ErrGoToolchainOutput
	}
	destinationQuery := receiverGateQuery{View: query.View, Receiver: frame.Params[offset], Body: body, Types: helperLoan.Types, Analysis: query.Analysis, Frame: frame, Target: query.Target}
	for syntax, err := range WalkSyntax(ctx, SyntaxWalkRequest{Root: body}) {
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
		assignment, ok := syntax.Node.(*ast.AssignStmt)
		if !ok {
			continue
		}
		value, err := receiverGateAssignedValue(ctx, assignment, destinationQuery)
		if err != nil {
			return false, err
		}
		if value != nil {
			if committed, err := receiverGateCallbackResultBefore(ctx, receiverGateCallbackQuery{View: query.View, Body: body, Types: helperLoan.Types, Analysis: query.Analysis, Callback: callback, Value: value, Frame: frame}); committed || err != nil {
				return committed, err
			}
		}
	}
	return false, nil
}

// The actual returned value must be validated. A similarly named parser or a
// different validated local cannot manufacture this observation. Every explicit
// nil-error return participates; one invalid successful return refuses the gate.
func receiverGateParserReturnsCheckedValue(ctx context.Context, body *ast.BlockStmt, facts *types.Info, signature *types.Signature, analysis PackageAnalysis, view SSAUnitView, frame *ssa.Function, target *types.Func) (bool, error) {
	validated := false
	for syntax, err := range WalkSyntax(ctx, SyntaxWalkRequest{Root: body}) {
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
		statement, ok := syntax.Node.(*ast.ReturnStmt)
		if !ok || len(statement.Results) != 2 || !facts.Types[statement.Results[1]].IsNil() {
			continue
		}
		identifier, ok := ast.Unparen(statement.Results[0]).(*ast.Ident)
		if !ok {
			return false, nil
		}
		_, ok = facts.ObjectOf(identifier).(*types.Var)
		if !ok {
			return false, nil
		}
		current, err := receiverGateValueBefore(ctx, receiverGateValueQuery{View: view, Body: body, Types: facts, Analysis: analysis, Value: statement.Results[0], Frame: frame, Target: target})
		if err != nil || !current {
			return false, err
		}
		validated = true
	}
	return validated, nil
}

type receiverGateCallbackQuery struct {
	View     SSAUnitView
	Analysis PackageAnalysis
	Body     *ast.BlockStmt
	Types    *types.Info
	Frame    *ssa.Function
	Callback *types.Var
	Value    ast.Expr
}

func (receiverGateCallbackQuery) goToolchainInternalFlow() {}

// Copies keep their native values. A replaced callback, a modified result or
// a path that bypasses the call cannot borrow an earlier callback's authority.
func receiverGateCallbackResultBefore(ctx context.Context, query receiverGateCallbackQuery) (bool, error) {
	if query.Frame == nil || query.Value == nil || query.Types == nil {
		return false, core.ErrGoToolchainOutput
	}
	expression := ast.Unparen(query.Value)
	facts, present := query.Types.Types[expression]
	if !present || facts.Type == nil {
		return false, core.ErrGoToolchainOutput
	}
	use := receiverGateExpressionUse(query.Frame, expression)
	if use == nil {
		return false, core.ErrGoToolchainOutput
	}
	if use.IsAddr {
		return false, nil
	}
	result, ok := use.X.(*ssa.Extract)
	if !ok || result.Index != 0 {
		return false, nil
	}
	operation, ok := result.Tuple.(*ssa.Call)
	if !ok {
		return false, nil
	}
	var callback *ssa.Parameter
	for _, parameter := range query.Frame.Params {
		if parameter.Object() == query.Callback {
			callback = parameter
			break
		}
	}
	if callback == nil {
		return false, core.ErrGoToolchainOutput
	}
	if operation.Common().Value != callback {
		return false, nil
	}
	before, err := query.View.InstructionDominates(ctx, SSAInstructionOrderRequest{Before: operation, After: use})
	if err != nil || !before.Proven {
		return false, err
	}
	for syntax, err := range WalkSyntax(ctx, SyntaxWalkRequest{Root: query.Body}) {
		if err != nil {
			return false, err
		}
		call, ok := syntax.Node.(*ast.CallExpr)
		if !ok {
			continue
		}
		value, _ := query.Frame.ValueForExpr(call)
		if value != operation {
			continue
		}
		propagation, err := query.View.ErrorPropagation(ctx, ErrorPropagationRequest{Analysis: query.Analysis, Call: call})
		return propagation.Preserved, err
	}
	return false, core.ErrGoToolchainOutput
}

func receiverGateTupleMatches(tuple *types.Tuple, required []types.Type) bool {
	if tuple.Len() != len(required) {
		return false
	}
	for index, expected := range required {
		if !types.Identical(tuple.At(index).Type(), expected) {
			return false
		}
	}
	return true
}

// Separate native loads can represent the same current record after a recheck.
// Only a nonescaping compiler home with no intervening writes lends that fact.
// Address escapes and opaque referrers remain unproven.
func receiverGateSameValue(ctx context.Context, view SSAUnitView, left, right ssa.Value) (bool, error) {
	if left == right {
		return true, nil
	}
	before, ok := left.(*ssa.UnOp)
	if !ok || before.Op != token.MUL {
		return false, nil
	}
	after, ok := right.(*ssa.UnOp)
	if !ok || after.Op != token.MUL || before.X != after.X || !types.Identical(before.Type(), after.Type()) {
		return false, nil
	}
	home, ok := before.X.(*ssa.Alloc)
	if !ok || home.Heap {
		return false, nil
	}
	return receiverGateHomeStable(ctx, view, home, before, after)
}
func receiverGateHomeStable(ctx context.Context, view SSAUnitView, address ssa.Value, before, after ssa.Instruction) (bool, error) {
	refs := address.Referrers()
	if refs == nil {
		return false, nil
	}
	for _, ref := range *refs {
		if err := temporal.Wait(temporal.WaitRequest{Context: ctx}); err != nil {
			return false, err
		}
		switch instruction := ref.(type) {
		case *ssa.DebugRef:
		case *ssa.UnOp:
			if instruction.Op != token.MUL || instruction.X != address {
				return false, nil
			}
		case *ssa.FieldAddr:
			if instruction.X != address {
				return false, nil
			}
			stable, err := receiverGateHomeStable(ctx, view, instruction, before, after)
			if err != nil || !stable {
				return false, err
			}
		case *ssa.Store:
			if instruction.Addr != address {
				return false, nil
			}
			earlier, err := view.InstructionDominates(ctx, SSAInstructionOrderRequest{Before: instruction, After: before})
			if err != nil {
				return false, err
			}
			if earlier.Proven {
				continue
			}
			later, err := view.InstructionDominates(ctx, SSAInstructionOrderRequest{Before: after, After: instruction})
			if err != nil || !later.Proven {
				return false, err
			}
		default:
			return false, nil
		}
	}
	return true, nil
}

// A constrained helper parameter gains only the concrete target supplied through
// its actual native destination/value argument pair. Spelling alone grants none.
func receiverGateBoundParameter(call *ssa.CallCommon, signature *types.Signature, target *types.Func) *types.Var {
	value := signature.Params().At(1)
	if _, ok := types.Unalias(value.Type()).(*types.TypeParam); !ok {
		return nil
	}
	offset := 0
	if signature.Recv() != nil && !call.IsInvoke() {
		offset = 1
	}
	if len(call.Args) <= offset+1 {
		return nil
	}
	owner := types.Unalias(target.Signature().Recv().Type())
	if pointer, ok := owner.(*types.Pointer); ok {
		owner = pointer.Elem()
	}
	if !types.Identical(call.Args[offset].Type(), types.NewPointer(owner)) || !types.Identical(call.Args[offset+1].Type(), owner) {
		return nil
	}
	return value
}
func receiverGateBoundCallReceiver(call *ast.CallExpr, query receiverGateValueQuery) ast.Expr {
	if query.BoundParameter == nil {
		return nil
	}
	selector, ok := ast.Unparen(call.Fun).(*ast.SelectorExpr)
	if !ok {
		return nil
	}
	selected := query.Types.Selections[selector]
	if selected == nil || selected.Kind() != types.MethodVal {
		return nil
	}
	function, ok := selected.Obj().(*types.Func)
	if !ok || function.Id() != query.Target.Id() {
		return nil
	}
	signature, ok := selected.Type().(*types.Signature)
	target := query.Target.Signature()
	if !ok || signature.Variadic() != target.Variadic() || !types.Identical(signature.Params(), target.Params()) || !types.Identical(signature.Results(), target.Results()) {
		return nil
	}
	if !types.Identical(query.Types.TypeOf(selector.X), query.BoundParameter.Type()) {
		return nil
	}
	return selector.X
}
