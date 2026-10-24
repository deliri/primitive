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
)

func TestExpressionSourceRetainsNoSiblingPopulation(t *testing.T) {
	testserial.Declare(t, core.TestIsolationDeclaration{Hazard: core.TestIsolationHazardRuntimeAllocation, Scope: core.TestIsolationScopePackageProcess})
	for _, width := range []int{32, 8193} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			source := `package sample;func produce()int{return 1};func observe(int){};func Entry(){value:=produce();` + strings.Repeat(`observe(value);`, width) + `}`
			a := nativeSymbolSourceAnalysis(t, t.TempDir(), source)
			err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], SourceValues: true, Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
				fn := view.Package.Func("Entry")
				var expression ast.Expr
				calls := 0
				for node := range ast.Preorder(fn.Syntax()) {
					call, ok := node.(*ast.CallExpr)
					if !ok {
						continue
					}
					name, ok := call.Fun.(*ast.Ident)
					if !ok || name.Name != "observe" {
						continue
					}
					if expression == nil {
						expression = call.Args[0]
					}
					calls++
				}
				if expression == nil || calls != width {
					t.Fatalf("native expression siblings=%d expression=%v, want%d and exact source input", calls, expression, width)
				}
				before, err := hostfacts.ObserveGoAllocationTotal(ctx)
				if err != nil {
					return err
				}
				got, err := view.ExpressionProducer(ctx, gotoolchain.ExpressionRequest{Analysis: a, Expression: expression})
				if err != nil || got.Validate() != nil || got.Function != view.Package.Func("produce").Object() || got.ResultIndex != 0 {
					t.Fatalf("source producer got=%+v,%v, want exact native produce result0,nil", got, err)
				}
				after, err := hostfacts.ObserveGoAllocationTotal(ctx)
				if err != nil {
					return err
				}
				allocated := after.Uint64() - before.Uint64()
				t.Logf("native source siblings=%d allocated=%d bytes", width, allocated)
				if allocated > 2048 {
					t.Fatalf("source siblings=%d allocated=%d, want <=2048 without sibling population", width, allocated)
				}
				return nil
			}})
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}
