package gotoolchain_test

import (
	"context"
	"errors"
	"go/ast"
	"io"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/gotoolchain"
)

func TestExpressionSourceCancellationKeepsNativeCause(t *testing.T) {
	t.Parallel()
	a := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;func produce()int{return 1};func observe(int){};func Entry(){observe(produce())}`)
	err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], SourceValues: true, Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
		var expression ast.Expr
		for node := range ast.Preorder(view.Package.Func("Entry").Syntax()) {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				continue
			}
			if name, ok := call.Fun.(*ast.Ident); ok && name.Name == "observe" {
				expression = call.Args[0]
				break
			}
		}
		if expression == nil {
			t.Fatal("fixture lost exact native producer expression")
		}
		cancelled, cancel := context.WithCancelCause(ctx)
		cancel(io.ErrClosedPipe)
		got, err := view.ExpressionProducer(cancelled, gotoolchain.ExpressionRequest{Analysis: a, Expression: expression})
		if got != (gotoolchain.ExpressionProducerObservation{}) || !errors.Is(err, context.Canceled) || !errors.Is(err, io.ErrClosedPipe) {
			t.Fatalf("cancelled source producer got=%+v,%v, want zero with native context and cause", got, err)
		}
		return nil
	}})
	if err != nil {
		t.Fatal(err)
	}
}

func BenchmarkExpressionSourceFirstProducer(b *testing.B) {
	b.ReportAllocs()
	a := nativeSymbolSourceAnalysis(b, b.TempDir(), `package sample;func produce()int{return 1};func observe(int){};func Entry(){value:=produce();`+strings.Repeat(`observe(value);`, 8193)+`}`)
	err := a.WithSSAUnit(b.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], SourceValues: true, Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
		var expression ast.Expr
		count := 0
		for node := range ast.Preorder(view.Package.Func("Entry").Syntax()) {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				continue
			}
			if name, ok := call.Fun.(*ast.Ident); ok && name.Name == "observe" {
				if expression == nil {
					expression = call.Args[0]
				}
				count++
			}
		}
		if expression == nil || count != 8193 {
			b.Fatalf("native source calls=%d expression=%v, want8193 and exact expression", count, expression)
		}
		for b.Loop() {
			got, err := view.ExpressionProducer(ctx, gotoolchain.ExpressionRequest{Analysis: a, Expression: expression})
			if err != nil || got.Function != view.Package.Func("produce").Object() || got.ResultIndex != 0 {
				b.Fatalf("source producer got=%+v,%v, want exact native producer result0,nil", got, err)
			}
		}
		return nil
	}})
	if err != nil {
		b.Fatal(err)
	}
}
