package gotoolchain_test

import (
	"context"
	"go/ast"
	"testing"

	"github.com/deliri/primitive/v2026/gotoolchain"
	"golang.org/x/tools/go/ssa"
)

func TestSSASourceCallUsesNativeExpressionIdentity(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name, body     string
		debug, present bool
		kind           uint8
	}{
		{"ordinary void call", `sink()`, true, true, 1},
		{"ordinary valued call", `_=value()`, true, true, 1},
		{"bound method", `bound:=cell.Method;bound()`, true, true, 1},
		{"method expression", `(*Cell).Method(cell)`, true, true, 1},
		{"copied function", `copy:=sink;copy()`, true, true, 1},
		{"interface invoke", `in.Method()`, true, true, 1},
		{"deferred call", `defer sink()`, true, true, 2},
		{"spawned call", `go sink()`, true, true, 3},
		{"conversion is not a call instruction", `_=int(1)`, true, false, 0},
		{"untaken call is absent", `if false{sink()}`, true, false, 0},
		{"debug-free ordinary call", `sink()`, false, true, 1},
		{"debug-free deferred call", `defer sink()`, false, true, 2},
		{"debug-free spawned call", `go sink()`, false, true, 3},
		{"literal invocation", `func(){}()`, true, true, 1},
		{"uninvoked child does not lend its call", `_=func(){sink()}`, true, false, 0},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			a := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;type Cell struct{};func(*Cell)Method(){};type Interface interface{Method()};func sink(){};func value()int{return 1};func Entry(cell *Cell,in Interface){`+row.body+`}`)
			err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], SourceValues: row.debug, Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
				fn := v.Package.Func("Entry")
				var expression *ast.CallExpr
				expressions := 0
				ast.Inspect(fn.Syntax(), func(node ast.Node) bool {
					if call, ok := node.(*ast.CallExpr); ok {
						expression = call
						expressions++
					}
					return true
				})
				if expressions != 1 {
					t.Fatalf("source calls=%d, want 1", expressions)
				}
				got, queryErr := v.CallForExpression(ctx, gotoolchain.SSASourceCallRequest{Function: fn, Expression: expression})
				if queryErr != nil || got.Validate() != nil || (got.Instruction != nil) != row.present {
					t.Fatalf("source call=(%+v,%v), want present=%t and nil", got, queryErr, row.present)
				}
				if got.Instruction != nil {
					var kind uint8
					switch got.Instruction.(type) {
					case *ssa.Call:
						kind = 1
					case *ssa.Defer:
						kind = 2
					case *ssa.Go:
						kind = 3
					}
					if kind != row.kind || got.Instruction.Parent() != fn || got.Instruction.Common().Pos() != expression.Lparen {
						t.Fatalf("source instruction kind/parent/position=%d/%v/%v, want %d/native frame/source call", kind, got.Instruction.Parent(), got.Instruction.Common().Pos(), row.kind)
					}
				}
				return nil
			}})
			if err != nil {
				t.Fatalf("WithSSAUnit error=%v, want nil", err)
			}
		})
	}
}
