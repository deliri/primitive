package gotoolchain_test

import (
	"context"
	"fmt"
	"go/ast"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gotoolchain"
	"github.com/deliri/primitive/v2026/hostfacts"
	"github.com/deliri/primitive/v2026/testserial"
	"golang.org/x/tools/go/ssa"
)

func TestSSAExpressionCallRetainsNoInstructionPopulation(t *testing.T) {
	testserial.Declare(t, core.TestIsolationDeclaration{Hazard: core.TestIsolationHazardRuntimeAllocation, Scope: core.TestIsolationScopePackageProcess})
	for _, population := range []int{32, 8193} {
		t.Run(fmt.Sprint(population), func(t *testing.T) {
			a := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;import "context";func observe(<-chan struct{}){};func Entry(ctx context.Context){channel:=ctx.Done();`+strings.Repeat(`observe(channel);`, population)+`}`)
			err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], SourceValues: true, Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
				fn := v.Package.Func("Entry")
				var expression ast.Expr
				calls := 0
				for node := range ast.Preorder(fn.Syntax()) {
					call, ok := node.(*ast.CallExpr)
					if !ok {
						continue
					}
					name, ok := call.Fun.(*ast.Ident)
					if ok && name.Name == "observe" {
						expression = call.Args[0]
						calls++
					}
				}
				if expression == nil || calls != population {
					t.Fatalf("source-call workload=%d expression=%v, want %d and native expression", calls, expression, population)
				}
				native, _ := fn.ValueForExpr(expression)
				actual, ok := native.(*ssa.Call)
				if !ok || !actual.Common().IsInvoke() || actual.Common().Method.Name() != "Done" {
					t.Fatal("fixture has no actual interface cancellation call")
				}
				before, err := hostfacts.ObserveGoAllocationTotal(ctx)
				if err != nil {
					return err
				}
				got, queryErr := v.ExpressionCall(ctx, gotoolchain.SSAExpressionCallRequest{Function: fn, Expression: expression})
				after, err := hostfacts.ObserveGoAllocationTotal(ctx)
				if err != nil {
					return err
				}
				allocated := after.Uint64() - before.Uint64()
				if queryErr != nil || got.Call != actual || got.ResultIndex != 0 || allocated > 1024 {
					t.Fatalf("source call got=%+v,%v allocation=%d width=%d, want actual call/result0 and <=1024 bytes", got, queryErr, allocated, population)
				}
				t.Logf("native call origin width=%d allocated=%d; compiler construction excluded", population, allocated)
				return nil
			}})
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func BenchmarkSSAExpressionCallCopiedInterface(b *testing.B) {
	b.ReportAllocs()
	const population = 257
	a := nativeSymbolSourceAnalysis(b, b.TempDir(), `package sample;import "context";func observe(<-chan struct{}){};func Entry(ctx context.Context){channel:=ctx.Done();`+strings.Repeat(`observe(channel);`, population)+`}`)
	err := a.WithSSAUnit(b.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], SourceValues: true, Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
		fn := v.Package.Func("Entry")
		var expression ast.Expr
		calls := 0
		for node := range ast.Preorder(fn.Syntax()) {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				continue
			}
			name, ok := call.Fun.(*ast.Ident)
			if ok && name.Name == "observe" {
				expression = call.Args[0]
				calls++
			}
		}
		if expression == nil || calls != population {
			b.Fatalf("native call workload=%d, want %d", calls, population)
		}
		request := gotoolchain.SSAExpressionCallRequest{Function: fn, Expression: expression}
		native, _ := fn.ValueForExpr(expression)
		actual, ok := native.(*ssa.Call)
		if !ok || !actual.Common().IsInvoke() || actual.Common().Method.Name() != "Done" {
			b.Fatal("fixture has no native interface call")
		}
		for b.Loop() {
			got, queryErr := v.ExpressionCall(ctx, request)
			if queryErr != nil || got.Call != actual || got.ResultIndex != 0 {
				b.Fatalf("native source-call query=%+v,%v, want actual call/result0", got, queryErr)
			}
		}
		b.ReportMetric(1, "origins/op")
		b.ReportMetric(population, "source_uses/op")
		return nil
	}})
	if err != nil {
		b.Fatal(err)
	}
}
