package gotoolchain_test

import (
	"context"
	"errors"
	"go/ast"
	"io"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gotoolchain"
	"golang.org/x/tools/go/ssa"
)

func TestSSASourceCallRequiresActualSourceAndLoan(t *testing.T) {
	t.Parallel()
	a := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;func sink(){};func Entry(){sink()};func Other(){sink()}`)
	err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], SourceValues: true, Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
		fn := v.Package.Func("Entry")
		other := v.Package.Func("Other")
		expression := fn.Syntax().(*ast.FuncDecl).Body.List[0].(*ast.ExprStmt).X.(*ast.CallExpr)
		clone := *expression
		cancelled, cancel := context.WithCancelCause(ctx)
		cancel(io.ErrClosedPipe)
		for _, row := range []struct {
			name       string
			view       gotoolchain.SSAUnitView
			ctx        context.Context
			fn         *ssa.Function
			expression *ast.CallExpr
			want       error
		}{
			{"absent loan", gotoolchain.SSAUnitView{}, ctx, fn, expression, core.ErrGoToolchainContract},
			{"absent function", v, ctx, nil, expression, core.ErrGoToolchainContract},
			{"absent expression", v, ctx, fn, nil, core.ErrGoToolchainContract},
			{"cloned source expression", v, ctx, fn, &clone, core.ErrGoToolchainContract},
			{"different native frame", v, ctx, other, expression, core.ErrGoToolchainContract},
			{"nil context", v, nil, fn, expression, core.ErrNilContext},
			{"custom cancellation cause", v, cancelled, fn, expression, io.ErrClosedPipe},
		} {
			t.Run(row.name, func(t *testing.T) {
				got, queryErr := row.view.CallForExpression(row.ctx, gotoolchain.SSASourceCallRequest{Function: row.fn, Expression: row.expression})
				if got != (gotoolchain.SSASourceCallObservation{}) || !errors.Is(queryErr, row.want) {
					t.Fatalf("source authority=(%+v,%v), want zero and %v", got, queryErr, row.want)
				}
			})
		}
		if err := (gotoolchain.SSASourceCallObservation{Instruction: (*ssa.Call)(nil)}).Validate(); !errors.Is(err, core.ErrGoToolchainOutput) {
			t.Fatalf("typed nil observation error=%v, want output refusal", err)
		}
		fn.Blocks[0].Index = 99
		got, queryErr := v.CallForExpression(ctx, gotoolchain.SSASourceCallRequest{Function: fn, Expression: expression})
		if got != (gotoolchain.SSASourceCallObservation{}) || !errors.Is(queryErr, core.ErrGoToolchainOutput) {
			t.Fatalf("malformed source block=(%+v,%v), want zero and output refusal", got, queryErr)
		}
		return nil
	}})
	if err != nil {
		t.Fatalf("WithSSAUnit error=%v, want nil", err)
	}
}

func TestSSASourceCallRefusesClonedDebugInstruction(t *testing.T) {
	t.Parallel()
	a := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;func value()int{return 1};func Entry(){_=value()}`)
	err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], SourceValues: true, Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
		fn := v.Package.Func("Entry")
		var expression *ast.CallExpr
		replaced := 0
		for _, block := range fn.Blocks {
			for _, instruction := range block.Instrs {
				if ref, ok := instruction.(*ssa.DebugRef); ok {
					if call, ok := ref.X.(*ssa.Call); ok {
						if selected, ok := ref.Expr.(*ast.CallExpr); ok {
							clone := *call
							ref.X = &clone
							expression = selected
							replaced++
						}
					}
				}
			}
		}
		if replaced != 1 {
			t.Fatalf("debug instruction replacements=%d, want 1", replaced)
		}
		got, queryErr := v.CallForExpression(ctx, gotoolchain.SSASourceCallRequest{Function: fn, Expression: expression})
		if got != (gotoolchain.SSASourceCallObservation{}) || !errors.Is(queryErr, core.ErrGoToolchainContract) {
			t.Fatalf("cloned instruction=(%+v,%v), want zero and contract refusal", got, queryErr)
		}
		return nil
	}})
	if err != nil {
		t.Fatalf("WithSSAUnit error=%v, want nil", err)
	}
}
