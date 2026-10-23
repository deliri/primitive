package gotoolchain_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gotoolchain"
	"github.com/deliri/primitive/v2026/testserial"
)

func TestSSABinaryOperationRetainsNoQueryPopulation(t *testing.T) {
	testserial.Declare(t, core.TestIsolationDeclaration{Hazard: core.TestIsolationHazardRuntimeAllocation, Scope: core.TestIsolationScopePackageProcess})
	for _, population := range []int{32, 8193} {
		for _, debug := range []bool{false, true} {
			t.Run(fmt.Sprintf("%d/debug=%t", population, debug), func(t *testing.T) {
				a := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;func consumeBool(bool){};func consumeInt(int){};func Entry(x,y int){`+strings.Repeat(`consumeInt(1);`, population)+`consumeBool(x==y)}`)
				err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], SourceValues: debug, Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
					fn := view.Package.Func("Entry")
					request := gotoolchain.SSABinaryOperationRequest{Function: fn, Expression: nativeBinaryExpression(t, fn)}
					baseline, queryErr := view.BinaryOperationForExpression(ctx, request)
					if queryErr != nil || baseline.Operation == nil {
						t.Fatalf("binary allocation precondition=%+v,%v, want operation,nil", baseline, queryErr)
					}
					var got gotoolchain.SSABinaryOperationObservation
					var gotErr error
					allocations := testing.AllocsPerRun(20, func() {
						got, gotErr = view.BinaryOperationForExpression(ctx, request)
					})
					if gotErr != nil || got.Operation != baseline.Operation || allocations != 0 {
						t.Fatalf("native binary=%+v,%v allocations=%g, want actual operation,nil,0", got, gotErr, allocations)
					}
					t.Logf("native binary prefix=%d debug=%t query allocations=%g", population, debug, allocations)
					return nil
				}})
				if err != nil {
					t.Fatalf("WithSSAUnit error=%v, want nil", err)
				}
			})
		}
	}
}
