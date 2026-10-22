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

func TestSSASourceCallRetainsNoInstructionPopulation(t *testing.T) {
	testserial.Declare(t, core.TestIsolationDeclaration{Hazard: core.TestIsolationHazardRuntimeAllocation, Scope: core.TestIsolationScopePackageProcess})
	for _, population := range []int{32, 8193} {
		t.Run(fmt.Sprint(population), func(t *testing.T) {
			a := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;func sink(){};func Entry(){`+strings.Repeat(`sink();`, population)+`}`)
			err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], SourceValues: true, Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
				fn := v.Package.Func("Entry")
				var expression *ast.CallExpr
				expressions, calls := 0, 0
				ast.Inspect(fn.Syntax(), func(node ast.Node) bool {
					if call, ok := node.(*ast.CallExpr); ok {
						expression = call
						expressions++
					}
					return true
				})
				for _, block := range fn.Blocks {
					for _, instruction := range block.Instrs {
						if _, ok := instruction.(*ssa.Call); ok {
							calls++
						}
					}
				}
				if expressions != population || calls != population {
					t.Fatalf("native expression/call counts=%d/%d, want %d/%d", expressions, calls, population, population)
				}
				before, err := hostfacts.ObserveGoAllocationTotal(ctx)
				if err != nil {
					return err
				}
				got, queryErr := v.CallForExpression(ctx, gotoolchain.SSASourceCallRequest{Function: fn, Expression: expression})
				after, err := hostfacts.ObserveGoAllocationTotal(ctx)
				if err != nil {
					return err
				}
				allocated := after.Uint64() - before.Uint64()
				if queryErr != nil || got.Instruction == nil || got.Instruction.Common().Pos() != expression.Lparen || allocated > 4096 {
					t.Fatalf("last source call=(%+v,%v) scratch=%d at %d calls, want exact call,nil and <=4096 bytes", got, queryErr, allocated, calls)
				}
				t.Logf("source-call scratch=%d at %d native calls; compiler construction excluded", allocated, calls)
				return nil
			}})
			if err != nil {
				t.Fatalf("WithSSAUnit error=%v, want nil", err)
			}
		})
	}
}
