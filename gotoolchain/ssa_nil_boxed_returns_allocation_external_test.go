package gotoolchain_test

import (
	"context"
	"fmt"
	"go/types"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gotoolchain"
	"github.com/deliri/primitive/v2026/hostfacts"
	"github.com/deliri/primitive/v2026/testserial"
	"golang.org/x/tools/go/ssa"
)

func TestSSANilBoxedReturnsRetainNoReturnPopulation(t *testing.T) {
	testserial.Declare(t, core.TestIsolationDeclaration{Hazard: core.TestIsolationHazardRuntimeAllocation, Scope: core.TestIsolationScopePackageProcess})
	for _, population := range []int{32, 16385} {
		t.Run(fmt.Sprint(population), func(t *testing.T) {
			a := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;type E struct{};func(*E)Error()string{return "error"};func Entry(flag bool)error{`+strings.Repeat(`if flag{return (*E)(nil)};`, population-1)+`return (*E)(nil)}`)
			err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
				r := gotoolchain.SSANilBoxedReturnRequest{Function: v.Package.Func("Entry"), Interface: types.Universe.Lookup("error").Type().Underlying().(*types.Interface)}
				actualReturns := 0
				for _, block := range r.Function.Blocks {
					for _, instruction := range block.Instrs {
						if _, returned := instruction.(*ssa.Return); returned {
							actualReturns++
						}
					}
				}
				if actualReturns != population {
					t.Fatalf("native fixture returns=%d, want %d", actualReturns, population)
				}
				before, err := hostfacts.ObserveGoAllocationTotal(ctx)
				if err != nil {
					return err
				}
				observed := 0
				for returned, err := range v.NilBoxedReturns(ctx, r) {
					if err != nil || returned == nil {
						t.Fatalf("first return=(%v,%v), want native return and nil", returned, err)
					}
					observed++
					break
				}
				after, err := hostfacts.ObserveGoAllocationTotal(ctx)
				if err != nil {
					return err
				}
				allocated := after.Uint64() - before.Uint64()
				if observed != 1 || allocated > 4<<10 {
					t.Fatalf("first return count/scratch=%d/%d at %d returns, want 1 and <=4096 bytes", observed, allocated, population)
				}
				t.Logf("nil-boxed return scratch=%d at %d returns; compiler construction excluded", allocated, population)
				return nil
			}})
			if err != nil {
				t.Fatalf("WithSSAUnit error=%v, want nil", err)
			}
		})
	}
}
