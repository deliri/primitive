package gotoolchain_test

import (
	"context"
	"errors"
	"fmt"
	"go/ast"
	"go/types"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gotoolchain"
)

func nativeExpressionCallMode(t *testing.T, directory string, mode, damage uint8) {
	t.Helper()
	mode %= 54
	damage %= 3
	rows := [...]struct {
		setup, expression, target string
		present, invoke           bool
		slot                      uint64
	}{
		{expression: `ctx.Done()`, target: "Done", present: true, invoke: true},
		{setup: `channel:=ctx.Done();`, expression: `channel`, target: "Done", present: true, invoke: true},
		{setup: `channel:=Signal(ctx.Done());`, expression: `channel`, target: "Done", present: true, invoke: true},
		{expression: `produce()`, target: "produce", present: true},
		{setup: `call:=produce;`, expression: `call()`, target: "produce", present: true},
		{setup: `channel,_:=pair();`, expression: `channel`, target: "pair", present: true},
		{setup: `_,channel:=pair();`, expression: `channel`, target: "pair", present: true, slot: 1},
		{setup: `channel:=ctx.Done();copy:=channel;channel=supplied;_=channel;`, expression: `copy`, target: "Done", present: true, invoke: true},
		{setup: `channel:=ctx.Done();copy:=channel;copy=supplied;`, expression: `copy`},
		{expression: `supplied`},
		{setup: `channel:=ctx.Done();if flag{channel=supplied};`, expression: `channel`},
		{setup: `channel:=ctx.Done();if flag{channel=ctx.Done()};`, expression: `channel`},
		{expression: `dynamic()`, present: true},
		{expression: `(func()<-chan struct{}{return nil})()`, present: true},
		{expression: `(<-chan struct{})(nil)`},
		{setup: `channel:=ctx.Done();`, expression: `(channel)`, target: "Done", present: true, invoke: true},
		{expression: `carry(ctx.Done())`, target: "carry", present: true},
		{expression: `cell.Produce()`, target: "Produce", present: true},
	}
	row := rows[mode%18]
	frame := mode / 18
	body := row.setup + `observe(` + row.expression + `)`
	function := `func Entry(ctx context.Context,supplied <-chan struct{},flag bool,dynamic func()<-chan struct{},cell Cell){` + body + `}`
	if frame == 1 {
		function = `func Entry(ctx context.Context,supplied <-chan struct{},flag bool,dynamic func()<-chan struct{},cell Cell){_=func(){` + body + `}}`
	}
	if frame == 2 {
		function = `func init(){` + body + `}`
	}
	source := `package sample;import "context";type Signal <-chan struct{};type Cell struct{};func(Cell)Produce()<-chan struct{}{return nil};func produce()<-chan struct{}{return nil};func pair()(<-chan struct{},<-chan struct{}){return nil,nil};func carry(v <-chan struct{})<-chan struct{}{return v};func observe(<-chan struct{}){};var ctx context.Context;var supplied <-chan struct{};var flag bool;var dynamic func()<-chan struct{};var cell Cell;` + function
	a := nativeSymbolSourceAnalysis(t, directory, source)
	err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], SourceValues: true, Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
		fn := v.Package.Func("Entry")
		if frame == 1 {
			if fn == nil || len(fn.AnonFuncs) == 0 {
				t.Fatal("fixture has no native child")
			}
			fn = fn.AnonFuncs[0]
		}
		if frame == 2 {
			for _, file := range v.Source.Syntax {
				for _, declaration := range file.Decls {
					decl, ok := declaration.(*ast.FuncDecl)
					if ok && decl.Name.Name == "init" {
						object, ok := v.Source.TypesInfo.Defs[decl.Name].(*types.Func)
						if !ok {
							t.Fatal("init has no native function declaration")
						}
						fn = v.Program.FuncValue(object)
					}
				}
			}
		}
		if fn == nil {
			t.Fatal("fixture has no selected native frame")
		}
		var expression ast.Expr
		for node := range ast.Preorder(fn.Syntax()) {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				continue
			}
			name, ok := call.Fun.(*ast.Ident)
			if ok && name.Name == "observe" {
				expression = call.Args[0]
				break
			}
		}
		if expression == nil {
			t.Fatal("fixture has no observed native source value")
		}
		request := gotoolchain.SSAExpressionCallRequest{Function: fn, Expression: expression}
		if damage == 1 {
			clone := *fn
			request.Function = &clone
			if request.Function == fn {
				t.Fatal("function custody mutation did not change identity")
			}
		}
		if damage == 2 {
			request.Expression = &ast.Ident{NamePos: expression.Pos(), Name: "unowned"}
			if request.Expression == expression {
				t.Fatal("source custody mutation did not change identity")
			}
		}
		got, queryErr := v.ExpressionCall(ctx, request)
		if damage != 0 {
			if got != (gotoolchain.SSAExpressionCallObservation{}) || !errors.Is(queryErr, core.ErrGoToolchainContract) {
				t.Fatalf("source authorization mode%d damage%d got=%+v,%v, want zero and contract refusal", mode, damage, got, queryErr)
			}
			return nil
		}
		if queryErr != nil || got.Validate() != nil || (got.Call != nil) != row.present || got.ResultIndex != row.slot {
			t.Fatalf("expression mode%d got=%+v,%v, want present=%v slot=%d", mode, got, queryErr, row.present, row.slot)
		}
		if got.Call == nil {
			return nil
		}
		if got.Call.Parent() != fn || got.Call.Common().IsInvoke() != row.invoke {
			t.Fatalf("expression mode%d borrowed wrong frame or dispatch: %v", mode, got.Call)
		}
		if row.invoke {
			if got.Call.Common().Method == nil || got.Call.Common().Method.Name() != row.target || got.Call.Common().Method.Pkg().Path() != "context" {
				t.Fatalf("expression mode%d lost actual context method", mode)
			}
			named, err := v.CallFunction(ctx, gotoolchain.SSACallFunctionRequest{Call: got.Call})
			if err != nil || named.Function != nil {
				t.Fatalf("interface declaration became concrete target: %+v,%v", named, err)
			}
		} else {
			named, err := v.CallFunction(ctx, gotoolchain.SSACallFunctionRequest{Call: got.Call})
			if err != nil {
				return err
			}
			if row.target == "" {
				if named.Function != nil {
					t.Fatalf("opaque/literal mode%d manufactured named target %v", mode, named.Function)
				}
			} else if named.Function == nil || named.Function.Name() != row.target || named.Function.Pkg() != v.Source.Types {
				t.Fatalf("expression mode%d got named target=%v, want native %s", mode, named.Function, row.target)
			}
		}
		return nil
	}})
	if err != nil {
		t.Fatal(err)
	}
}

func TestSSAExpressionCallNativeOriginsAndFrames(t *testing.T) {
	t.Parallel()
	for mode := uint8(0); mode < 54; mode++ {
		t.Run(fmt.Sprint(mode), func(t *testing.T) { t.Parallel(); nativeExpressionCallMode(t, t.TempDir(), mode, 0) })
	}
}
func FuzzSSAExpressionCallConservesNativeOrigin(f *testing.F) {
	f.Add(uint8(0), uint8(0))
	f.Fuzz(func(t *testing.T, mode, damage uint8) { nativeExpressionCallMode(t, t.TempDir(), mode, damage) })
}
