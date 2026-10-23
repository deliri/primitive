package gotoolchain_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gotoolchain"
	"github.com/deliri/primitive/v2026/hostfacts"
	"github.com/deliri/primitive/v2026/testserial"
)

func TestSSAReferrersAllocateNoReferencePopulation(t *testing.T) {
	testserial.Declare(t, core.TestIsolationDeclaration{Hazard: core.TestIsolationHazardRuntimeAllocation, Scope: core.TestIsolationScopePackageProcess})
	for _, population := range []int{32, 8193} {
		t.Run(fmt.Sprint(population), func(t *testing.T) {
			a := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;func sink(int){};func Entry(x int){f:=func(){sink(x)};`+strings.Repeat("f();", population)+`}`)
			err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
				value, queryErr := nativeReferrerValue(ctx, view)
				if queryErr != nil || value == nil {
					t.Fatalf("native closure=%v,%v, want nonnil,nil", value, queryErr)
				}
				before, queryErr := hostfacts.ObserveGoAllocationTotal(ctx)
				if queryErr != nil {
					return queryErr
				}
				uses := 0
				for instruction, queryErr := range view.Referrers(ctx, gotoolchain.SSAReferrerRequest{Instruction: value}) {
					if queryErr != nil || instruction == nil {
						t.Fatalf("native use=%v,%v, want instruction,nil", instruction, queryErr)
					}
					uses++
				}
				after, queryErr := hostfacts.ObserveGoAllocationTotal(ctx)
				if queryErr != nil {
					return queryErr
				}
				allocated := after.Uint64() - before.Uint64()
				if uses != population || allocated != 0 {
					t.Fatalf("native uses/scratch=%d/%d, want %d/0", uses, allocated, population)
				}
				return nil
			}})
			if err != nil {
				t.Fatalf("WithSSAUnit error=%v, want nil", err)
			}
		})
	}
}
