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

// LiteralInvocationRequest borrows one actual call from the selected native
// analysis. A separately parsed call with identical positions is not that call.
type LiteralInvocationRequest struct{ Call *ast.CallExpr }

func (r LiteralInvocationRequest) Validate() error {
	if r.Call == nil || !r.Call.Lparen.IsValid() {
		return core.ErrGoToolchainContract
	}
	return nil
}
func (LiteralInvocationRequest) goToolchainProtocolFact() {}

// LiteralInvocationObservation lends only a synchronous call's exact native
// literal target. Absent includes unknown/merged targets, named functions,
// eliminated calls, defer and go. Absent never certifies runtime behavior.
type LiteralInvocationObservation struct {
	Literal   *ast.FuncLit
	Types     *types.Info
	Positions *token.FileSet
	Presence  FunctionSourcePresence
}

func (o LiteralInvocationObservation) Validate() error {
	if err := o.Presence.Validate(); err != nil {
		return err
	}
	if o.Presence == FunctionSourceAbsent {
		if o.Literal != nil || o.Types != nil || o.Positions != nil {
			return core.ErrGoToolchainContract
		}
		return nil
	}
	if o.Literal == nil || o.Types == nil || o.Positions == nil {
		return core.ErrGoToolchainContract
	}
	if _, ok := o.Types.TypeOf(o.Literal).(*types.Signature); !ok {
		return core.ErrGoToolchainContract
	}
	position := o.Positions.PositionFor(o.Literal.Pos(), false)
	if !position.IsValid() {
		return core.ErrGoToolchainContract
	}
	return nil
}
func (LiteralInvocationObservation) goToolchainProtocolFact() {}

// LiteralInvocation uses Go tools' native SSA call target, including native
// function values and captures. It builds one owning compiler unit; native SSA
// construction is an explicit whole-unit allocation, not an O(1) compiler claim.
// No product variable inventory, alias simulator or function cache is retained.
// The native builder is synchronous; cancellation is checked around construction
// and before each query traversal/instruction.
func (a PackageAnalysis) LiteralInvocation(ctx context.Context, request LiteralInvocationRequest) (LiteralInvocationObservation, error) {
	if err := errors.Join(temporal.Wait(temporal.WaitRequest{Context: ctx}), a.Validate(), request.Validate()); err != nil {
		return LiteralInvocationObservation{}, err
	}
	if a.Incomplete {
		return LiteralInvocationObservation{}, core.ErrGoToolchainOutput
	}
	for _, unit := range a.Units {
		owns := false
		for _, file := range unit.Syntax {
			for syntax, err := range WalkSyntax(ctx, SyntaxWalkRequest{Root: file}) {
				if err != nil {
					return LiteralInvocationObservation{}, err
				}
				if syntax.Node == request.Call {
					owns = true
					break
				}
			}
			if owns {
				break
			}
		}
		if !owns {
			continue
		}
		calleeType := unit.TypesInfo.TypeOf(request.Call.Fun)
		if _, exists := unit.TypesInfo.Types[request.Call]; !exists || calleeType == nil {
			return LiteralInvocationObservation{}, core.ErrGoToolchainOutput
		}
		if _, ok := calleeType.Underlying().(*types.Signature); !ok {
			return LiteralInvocationObservation{Presence: FunctionSourceAbsent}, nil
		}
		program := ssa.NewProgram(unit.Fset, ssa.BuilderMode(0))
		for _, imported := range unit.Types.Imports() {
			program.CreatePackage(imported, nil, nil, true)
		}
		compiled := program.CreatePackage(unit.Types, unit.Syntax, unit.TypesInfo, false)
		compiled.Build()
		if err := temporal.Wait(temporal.WaitRequest{Context: ctx}); err != nil {
			return LiteralInvocationObservation{}, err
		}
		if observation, found, err := nativeLiteralInvocation(ctx, request.Call, compiled.Func("init"), unit.TypesInfo, unit.Fset); found || err != nil {
			return observation, err
		}
		for _, file := range unit.Syntax {
			for _, declaration := range file.Decls {
				fn, ok := declaration.(*ast.FuncDecl)
				if !ok || fn.Body == nil {
					continue
				}
				object, ok := unit.TypesInfo.Defs[fn.Name].(*types.Func)
				if !ok {
					return LiteralInvocationObservation{}, core.ErrGoToolchainOutput
				}
				function := program.FuncValue(object)
				if function == nil {
					return LiteralInvocationObservation{}, core.ErrGoToolchainOutput
				}
				if observation, found, err := nativeLiteralInvocation(ctx, request.Call, function, unit.TypesInfo, unit.Fset); found || err != nil {
					return observation, err
				}
			}
		}
		return LiteralInvocationObservation{Presence: FunctionSourceAbsent}, nil
	}
	return LiteralInvocationObservation{}, core.ErrGoToolchainContract
}

func nativeLiteralInvocation(ctx context.Context, call *ast.CallExpr, function *ssa.Function, facts *types.Info, positions *token.FileSet) (LiteralInvocationObservation, bool, error) {
	if function == nil {
		return LiteralInvocationObservation{}, false, core.ErrGoToolchainOutput
	}
	for _, block := range function.Blocks {
		for _, instruction := range block.Instrs {
			if err := temporal.Wait(temporal.WaitRequest{Context: ctx}); err != nil {
				return LiteralInvocationObservation{}, false, err
			}
			invocation, ok := instruction.(ssa.CallInstruction)
			if !ok || invocation.Common().Pos() != call.Lparen {
				continue
			}
			if _, synchronous := instruction.(*ssa.Call); !synchronous {
				return LiteralInvocationObservation{Presence: FunctionSourceAbsent}, true, nil
			}
			target := invocation.Common().StaticCallee()
			if target == nil {
				return LiteralInvocationObservation{Presence: FunctionSourceAbsent}, true, nil
			}
			literal, ok := target.Syntax().(*ast.FuncLit)
			if !ok {
				return LiteralInvocationObservation{Presence: FunctionSourceAbsent}, true, nil
			}
			observation := LiteralInvocationObservation{Literal: literal, Types: facts, Positions: positions, Presence: FunctionSourcePresent}
			if err := observation.Validate(); err != nil {
				return LiteralInvocationObservation{}, false, err
			}
			return observation, true, nil
		}
	}
	for _, anonymous := range function.AnonFuncs {
		if observation, found, err := nativeLiteralInvocation(ctx, call, anonymous, facts, positions); found || err != nil {
			return observation, found, err
		}
	}
	return LiteralInvocationObservation{}, false, nil
}

var (
	_ protocolFact = LiteralInvocationRequest{}
	_ protocolFact = LiteralInvocationObservation{}
)
