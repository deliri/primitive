package gotoolchain

import (
	"context"
	"errors"
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"sort"

	pc "github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
	"golang.org/x/tools/go/ssa"
)

// ErrorPropagationRequest borrows one call from the selected native compilation.
// Preserved means the native facts establish return of the original error;
// opaque dynamic behavior remains unproven. No product rule is applied here.
type ErrorPropagationRequest struct {
	Analysis PackageAnalysis
	Call     *ast.CallExpr
}

func (r ErrorPropagationRequest) Validate() error {
	if r.Call == nil {
		return pc.ErrGoToolchainContract
	}
	return r.Analysis.Validate()
}
func (ErrorPropagationRequest) goToolchainInternalFlow() {}

// ErrorIdentityRequest asks whether one expression preserves the actual error
// parameter of a native function, including Go-owned copies and local helpers.
type ErrorIdentityRequest struct {
	Analysis   PackageAnalysis
	Function   *types.Func
	Parameter  *types.Var
	Expression ast.Expr
}

func (r ErrorIdentityRequest) Validate() error {
	if r.Function == nil || r.Parameter == nil || r.Expression == nil {
		return pc.ErrGoToolchainContract
	}
	return r.Analysis.Validate()
}
func (ErrorIdentityRequest) goToolchainInternalFlow() {}

// ErrorIdentityObservation reports a compiler-established preservation fact.
// False includes opaque contracts, and does not invent their runtime behavior.
type ErrorIdentityObservation struct{ Preserved bool }

func (ErrorIdentityObservation) Validate() error          { return nil }
func (ErrorIdentityObservation) goToolchainProtocolFact() {}

func (v SSAUnitView) errorAnalysis(ctx context.Context, analysis PackageAnalysis) error {
	if err := errors.Join(temporal.Wait(temporal.WaitRequest{Context: ctx}), v.Validate(), analysis.Validate()); err != nil {
		return err
	}
	if analysis.Incomplete {
		return pc.ErrGoToolchainOutput
	}
	for _, unit := range analysis.Units {
		if unit == v.Source {
			return nil
		}
	}
	return pc.ErrGoToolchainContract
}

func (v SSAUnitView) ErrorPropagation(ctx context.Context, request ErrorPropagationRequest) (ErrorIdentityObservation, error) {
	if err := errors.Join(v.errorAnalysis(ctx, request.Analysis), request.Validate()); err != nil {
		return ErrorIdentityObservation{}, err
	}
	call := request.Call
	if v.Source.TypesInfo.TypeOf(call) == nil {
		return ErrorIdentityObservation{}, pc.ErrGoToolchainOutput
	}
	physical := v.Source.Fset.File(call.Pos())
	for _, file := range v.Source.Syntax {
		if physical == nil || v.Source.Fset.File(file.Pos()) != physical {
			continue
		}
		index := sort.Search(len(file.Decls), func(index int) bool { return file.Decls[index].End() > call.Pos() })
		if index == len(file.Decls) {
			return ErrorIdentityObservation{}, pc.ErrGoToolchainOutput
		}
		function, ok := file.Decls[index].(*ast.FuncDecl)
		if !ok || call.Pos() < function.Pos() || call.End() > function.End() {
			return ErrorIdentityObservation{}, pc.ErrGoToolchainOutput
		}
		object, ok := v.Source.TypesInfo.Defs[function.Name].(*types.Func)
		if !ok {
			return ErrorIdentityObservation{}, pc.ErrGoToolchainOutput
		}
		preserved, err := errorCallPropagation(ctx, call, v.Program.FuncValue(object), errorIdentityQuery{Analysis: request.Analysis, View: v})
		if err != nil {
			return ErrorIdentityObservation{}, err
		}
		return ErrorIdentityObservation{Preserved: preserved}, nil
	}
	return ErrorIdentityObservation{}, pc.ErrGoToolchainOutput
}

func (v SSAUnitView) ErrorIdentity(ctx context.Context, request ErrorIdentityRequest) (ErrorIdentityObservation, error) {
	if err := errors.Join(v.errorAnalysis(ctx, request.Analysis), request.Validate()); err != nil {
		return ErrorIdentityObservation{}, err
	}
	if v.Source.TypesInfo.TypeOf(request.Expression) == nil {
		return ErrorIdentityObservation{}, pc.ErrGoToolchainOutput
	}
	fn := v.Program.FuncValue(request.Function)
	if fn == nil || fn.Pkg != v.Package {
		return ErrorIdentityObservation{}, pc.ErrGoToolchainContract
	}
	for _, parameter := range fn.Params {
		if parameter.Object() != request.Parameter {
			continue
		}
		if !types.AssignableTo(parameter.Type(), types.Universe.Lookup("error").Type()) {
			return ErrorIdentityObservation{}, pc.ErrGoToolchainContract
		}
		query := errorIdentityQuery{Analysis: request.Analysis, Frame: fn, Origin: parameter, View: v}
		preserved, err := query.Preserved(ctx, request.Expression)
		if err != nil {
			return ErrorIdentityObservation{}, err
		}
		return ErrorIdentityObservation{Preserved: preserved}, nil
	}
	return ErrorIdentityObservation{}, pc.ErrGoToolchainContract
}

// This lookup borrows the actual SSA call target, including aliases. A local
// selector spelling cannot acquire standard-library constructor authority.
func errorNativeCallee(fn *ssa.Function, expression *ast.CallExpr) *types.Func {
	value, _ := fn.ValueForExpr(expression)
	call, ok := value.(*ssa.Call)
	if !ok {
		return nil
	}
	target := call.Common().StaticCallee()
	if target == nil {
		return nil
	}
	function, _ := target.Object().(*types.Func)
	return function
}

// Borrow the compiler's exact variable and result tuple. A same-spelled local
// is a different Go object and cannot carry this refusal.
// errorIdentityQuery borrows native values only for the synchronous query.
// Its parent is the active helper-call ancestry, never a retained call graph.
type errorIdentityQuery struct {
	View     SSAUnitView
	Analysis PackageAnalysis
	Parent   *errorIdentityQuery
	Frame    *ssa.Function
	Origin   ssa.Value
}

func (r errorIdentityQuery) Matches(expression ast.Expr) bool {
	value, _ := r.Frame.ValueForExpr(expression)
	return value != nil && errorSameValue(value, r.Origin)
}

// Preserved asks whether the expression carries this exact refusal. Merely
// using the variable in prose, an ignored parameter or a closure grants no
// authority. Local helpers borrow their actual Go parameter and body.
func (r errorIdentityQuery) Preserved(ctx context.Context, expression ast.Expr) (bool, error) {
	if err := temporal.Wait(temporal.WaitRequest{Context: ctx}); err != nil {
		return false, err
	}
	if r.Matches(expression) {
		return true, nil
	}
	call, ok := ast.Unparen(expression).(*ast.CallExpr)
	if !ok || r.View.Source.TypesInfo == nil {
		return false, nil
	}
	function := errorNativeCallee(r.Frame, call)
	if function == nil {
		return false, nil
	}
	if function.Pkg() != nil && function.Pkg().Path() == "errors" && function.Name() == "Join" {
		for _, argument := range call.Args {
			if preserved, err := r.Preserved(ctx, argument); preserved || err != nil {
				return preserved, err
			}
		}
		return false, nil
	}
	if function.Pkg() != nil && function.Pkg().Path() == "fmt" && function.Name() == "Errorf" {
		return errorFormatPreserves(ctx, call, r)
	}
	loan, err := r.Analysis.FunctionSource(ctx, FunctionSourceRequest{Function: function})
	if err != nil {
		return false, err
	}
	if loan.Presence != FunctionSourcePresent || loan.Declaration.Body == nil {
		return false, nil
	}
	if loan.Types != r.View.Source.TypesInfo {
		return false, pc.ErrGoToolchainOutput
	}
	native, ok := loan.Types.Defs[loan.Declaration.Name].(*types.Func)
	if !ok {
		return false, pc.ErrGoToolchainOutput
	}
	signature, ok := native.Type().(*types.Signature)
	if !ok {
		return false, pc.ErrGoToolchainOutput
	}
	if signature.Variadic() || signature.Results().Len() != 1 || !types.AssignableTo(signature.Results().At(0).Type(), types.Universe.Lookup("error").Type()) {
		return false, nil
	}
	offset := 0
	if selector, ok := ast.Unparen(call.Fun).(*ast.SelectorExpr); ok {
		if selection := r.View.Source.TypesInfo.Selections[selector]; selection != nil && selection.Kind() == types.MethodExpr {
			offset = 1
		}
	}
	for index, argument := range call.Args {
		slot := index - offset
		if slot < 0 || slot >= signature.Params().Len() {
			continue
		}
		preserved, err := r.Preserved(ctx, argument)
		if err != nil {
			return false, err
		}
		if !preserved {
			continue
		}
		parameter := signature.Params().At(slot)
		cycle := false
		for parent := &r; parent != nil; parent = parent.Parent {
			if actual, ok := parent.Origin.(*ssa.Parameter); ok && actual.Object() == parameter {
				cycle = true
				break
			}
		}
		if cycle {
			continue
		}
		next := errorIdentityQuery{Analysis: r.Analysis, Parent: &r, View: r.View}
		preserved, err = errorHelperReturns(ctx, r.View.Program.FuncValue(native), parameter, next)
		if err != nil {
			return false, err
		}
		if preserved {
			return true, nil
		}
	}
	return false, nil
}

func (errorIdentityQuery) goToolchainInternalFlow() {}
func errorHelperReturns(ctx context.Context, fn *ssa.Function, parameterObject *types.Var, reference errorIdentityQuery) (bool, error) {
	if err := temporal.Wait(temporal.WaitRequest{Context: ctx}); err != nil {
		return false, err
	}
	if fn == nil || fn.Prog != reference.View.Program {
		return false, pc.ErrGoToolchainOutput
	}
	for _, parameter := range fn.Params {
		if parameter.Object() == parameterObject {
			reference.Frame, reference.Origin = fn, parameter
			break
		}
	}
	if reference.Origin == nil {
		return false, pc.ErrGoToolchainOutput
	}
	if len(fn.Blocks) == 0 {
		return false, nil
	}
	return errorReturnRegion(ctx, reference, fn.Blocks[0])
}

func errorReturnRegion(ctx context.Context, reference errorIdentityQuery, region *ssa.BasicBlock) (bool, error) {
	fn := reference.Frame
	observed := false
	for _, block := range fn.Blocks {
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
			if err := temporal.Wait(temporal.WaitRequest{Context: ctx}); err != nil {
				return false, err
			}
			returned, ok := instruction.(*ssa.Return)
			if !ok {
				continue
			}
			observed = true
			if len(returned.Results) != fn.Signature.Results().Len() {
				return false, pc.ErrGoToolchainOutput
			}
			carries := false
			for _, value := range returned.Results {
				if !types.AssignableTo(value.Type(), types.Universe.Lookup("error").Type()) {
					continue
				}
				var err error
				carries, err = errorResultPreserves(ctx, value, &reference)
				if err != nil {
					return false, err
				}
				if carries {
					break
				}
			}
			if !carries {
				return false, nil
			}
		}
	}
	return observed, nil
}

// Go's dominator tree proves a nil-input arm. A merged successor or a guard on
// a different value cannot turn a lost refusal into neutral input.
func errorNilAt(ctx context.Context, reference *errorIdentityQuery, target *ssa.BasicBlock) (bool, error) {
	for dominator := target.Idom(); dominator != nil; dominator = dominator.Idom() {
		arm, err := errorNilSuccessor(ctx, reference, dominator)
		if err != nil {
			return false, err
		}
		if arm != nil && len(arm.Preds) == 1 && arm.Preds[0] == dominator && arm.Dominates(target) {
			return true, nil
		}
	}
	return false, nil
}

func errorNilSuccessor(ctx context.Context, reference *errorIdentityQuery, block *ssa.BasicBlock) (*ssa.BasicBlock, error) {
	if len(block.Instrs) == 0 || len(block.Succs) != 2 {
		return nil, nil
	}
	branch, ok := block.Instrs[len(block.Instrs)-1].(*ssa.If)
	if !ok {
		return nil, nil
	}
	comparison, ok := branch.Cond.(*ssa.BinOp)
	if !ok || comparison.Op != token.EQL && comparison.Op != token.NEQ {
		return nil, nil
	}
	value, zero := comparison.X, comparison.Y
	constant, ok := zero.(*ssa.Const)
	if !ok || !constant.IsNil() {
		value, zero = zero, value
		constant, ok = zero.(*ssa.Const)
	}
	if !ok || !constant.IsNil() {
		return nil, nil
	}
	carries, err := errorResultPreserves(ctx, value, reference)
	if err != nil || !carries {
		return nil, err
	}
	nilArm := 0
	if comparison.Op == token.NEQ {
		nilArm = 1
	}
	return block.Succs[nilArm], nil
}

// Interface conversions preserve the same Go value. A merged result must carry
// that value on every incoming edge; an unrelated or replaced value cannot do so.
func errorSameValue(value, origin ssa.Value) bool {
	for {
		if value == origin {
			return origin != nil
		}
		switch converted := value.(type) {
		case *ssa.ChangeType:
			value = converted.X
		case *ssa.ChangeInterface:
			value = converted.X
		case *ssa.MakeInterface:
			value = converted.X
		default:
			return false
		}
	}
}

func errorResultPreserves(ctx context.Context, value ssa.Value, reference *errorIdentityQuery) (bool, error) {
	if errorSameValue(value, reference.Origin) {
		return true, nil
	}
	if merged, ok := value.(*ssa.Phi); ok {
		for _, edge := range merged.Edges {
			if !errorSameValue(edge, reference.Origin) {
				return false, nil
			}
		}
		return len(merged.Edges) != 0, nil
	}
	call, ok := value.(*ssa.Call)
	if !ok {
		return false, nil
	}
	return errorCallPreserves(ctx, call, reference)
}

func errorCallPreserves(ctx context.Context, call *ssa.Call, reference *errorIdentityQuery) (bool, error) {
	for syntax, err := range WalkSyntax(ctx, SyntaxWalkRequest{Root: reference.Frame.Syntax()}) {
		if err != nil {
			return false, err
		}
		expression, ok := syntax.Node.(*ast.CallExpr)
		if ok && expression.Lparen == call.Common().Pos() {
			return reference.Preserved(ctx, expression)
		}
	}
	return false, pc.ErrGoToolchainOutput
}

// Go's formatter owns operand selection. The query observes whether a selected
// argument preserves the actual compiler-owned error. Unknown runtime formats
// and opaque variadic sources cannot certify that identity.
func errorFormatPreserves(ctx context.Context, call *ast.CallExpr, reference errorIdentityQuery) (bool, error) {
	if err := temporal.Wait(temporal.WaitRequest{Context: ctx}); err != nil {
		return false, err
	}
	if call == nil || reference.View.Source.TypesInfo == nil {
		return false, pc.ErrGoToolchainContract
	}
	if len(call.Args) == 0 || call.Ellipsis.IsValid() {
		return false, nil
	}
	format, present := reference.View.Source.TypesInfo.Types[call.Args[0]]
	if !present && reference.View.Source.TypesInfo.TypeOf(call.Args[0]) == nil {
		return false, pc.ErrGoToolchainOutput
	}
	if format.Value == nil || format.Value.Kind() != constant.String {
		return false, nil
	}
	request := ErrorFormatRequest{Format: constant.StringVal(format.Value), ArgumentCount: len(call.Args) - 1}
	for operand, err := range ErrorFormatOperands(ctx, request) {
		if err != nil {
			return false, err
		}
		if operand.Index < 0 || operand.Index >= len(call.Args)-1 {
			return false, pc.ErrGoToolchainOutput
		}
		if preserved, err := reference.Preserved(ctx, call.Args[operand.Index+1]); preserved || err != nil {
			return preserved, err
		}
	}
	return false, nil
}
func errorCallPropagation(ctx context.Context, call *ast.CallExpr, fn *ssa.Function, facts errorIdentityQuery) (bool, error) {
	if fn == nil {
		return false, pc.ErrGoToolchainOutput
	}
	for _, child := range fn.AnonFuncs {
		if root := child.Syntax(); root != nil && root.Pos() <= call.Pos() && root.End() >= call.End() {
			return errorCallPropagation(ctx, call, child, facts)
		}
	}
	value, _ := fn.ValueForExpr(call)
	origin, ok := value.(*ssa.Call)
	if !ok {
		// An actual ordinary call without its requested source value is missing
		// compiler metadata. Deferred and launched calls have no result to lend.
		for _, block := range fn.Blocks {
			for _, instruction := range block.Instrs {
				if actual, ok := instruction.(*ssa.Call); ok && actual.Common().Pos() == call.Lparen {
					return false, pc.ErrGoToolchainOutput
				}
			}
		}
		return false, nil
	}
	if errorUntakenBlock(origin.Block()) {
		return false, nil
	}
	if types.AssignableTo(origin.Type(), types.Universe.Lookup("error").Type()) {
		return errorReturnRegion(ctx, errorIdentityQuery{Analysis: facts.Analysis, Frame: fn, Origin: origin, View: facts.View}, origin.Block())
	}
	tuple, ok := origin.Type().(*types.Tuple)
	if !ok || origin.Referrers() == nil {
		return false, nil
	}
	for slot := range tuple.Len() {
		if !types.AssignableTo(tuple.At(slot).Type(), types.Universe.Lookup("error").Type()) {
			continue
		}
		for _, use := range *origin.Referrers() {
			if extracted, ok := use.(*ssa.Extract); ok && extracted.Index == slot {
				return errorReturnRegion(ctx, errorIdentityQuery{Analysis: facts.Analysis, Frame: fn, Origin: extracted, View: facts.View}, origin.Block())
			}
		}
		return false, nil
	}
	return false, nil
}
func errorUntakenBlock(block *ssa.BasicBlock) bool {
	for dominator := block.Idom(); dominator != nil; dominator = dominator.Idom() {
		if len(dominator.Instrs) == 0 {
			continue
		}
		branch, ok := dominator.Instrs[len(dominator.Instrs)-1].(*ssa.If)
		if !ok || len(dominator.Succs) != 2 {
			continue
		}
		condition, ok := branch.Cond.(*ssa.Const)
		if !ok || condition.Value == nil || condition.Value.Kind() != constant.Bool {
			continue
		}
		successor := dominator.Succs[0]
		if constant.BoolVal(condition.Value) {
			successor = dominator.Succs[1]
		}
		contained := true
		for _, predecessor := range successor.Preds {
			if predecessor != dominator && !successor.Dominates(predecessor) {
				contained = false
				break
			}
		}
		if contained && successor.Dominates(block) {
			return true
		}
	}
	return false
}
