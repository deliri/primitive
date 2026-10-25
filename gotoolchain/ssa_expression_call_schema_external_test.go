package gotoolchain_test

import (
	"context"
	"errors"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gotoolchain"
	"go/ast"
	"golang.org/x/tools/go/ssa"
	"testing"
)

func TestSSAExpressionCallCompilerVisibleSchema(t *testing.T) {
	t.Parallel()
	var typedNil *ast.Ident
	for _, row := range []struct {
		name    string
		request gotoolchain.SSAExpressionCallRequest
		want    error
	}{
		{name: "zero request", want: core.ErrGoToolchainContract},
		{name: "missing frame", request: gotoolchain.SSAExpressionCallRequest{Expression: &ast.Ident{Name: "value"}}, want: core.ErrGoToolchainContract},
		{name: "missing expression", request: gotoolchain.SSAExpressionCallRequest{Function: new(ssa.Function)}, want: core.ErrGoToolchainContract},
		{name: "typed nil expression", request: gotoolchain.SSAExpressionCallRequest{Function: new(ssa.Function), Expression: typedNil}, want: core.ErrGoToolchainContract},
		{name: "schema is distinct from custody", request: gotoolchain.SSAExpressionCallRequest{Function: new(ssa.Function), Expression: &ast.Ident{Name: "value"}}},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			if err := row.request.Validate(); !errors.Is(err, row.want) {
				t.Fatalf("request schema=%v, want %v", err, row.want)
			}
		})
	}
	for _, row := range []struct {
		name        string
		observation gotoolchain.SSAExpressionCallObservation
		want        error
	}{
		{name: "opaque origin"},
		{name: "ordinal without producer", observation: gotoolchain.SSAExpressionCallObservation{ResultIndex: 1}, want: core.ErrGoToolchainContract},
		{name: "overflow without producer", observation: gotoolchain.SSAExpressionCallObservation{ResultIndex: ^uint64(0)}, want: core.ErrGoToolchainContract},
		{name: "call without native callable", observation: gotoolchain.SSAExpressionCallObservation{Call: new(ssa.Call)}, want: core.ErrGoToolchainOutput},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			if err := row.observation.Validate(); !errors.Is(err, row.want) {
				t.Fatalf("observation schema=%v, want %v", err, row.want)
			}
		})
	}
}

func TestSSAExpressionCallRefusesUnavailableSourceValues(t *testing.T) {
	t.Parallel()
	a := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;func produce()int{return 1};func observe(int){};func Entry(){observe(produce())}`)
	err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], SourceValues: false, Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
		fn := v.Package.Func("Entry")
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
			t.Fatal("fixture has no exact source expression")
		}
		got, queryErr := v.ExpressionCall(ctx, gotoolchain.SSAExpressionCallRequest{Function: fn, Expression: expression})
		if got != (gotoolchain.SSAExpressionCallObservation{}) || !errors.Is(queryErr, core.ErrGoToolchainOutput) {
			t.Fatalf("debug-free expression got=%+v,%v, want zero and missing-source refusal", got, queryErr)
		}
		return nil
	}})
	if err != nil {
		t.Fatal(err)
	}
}
