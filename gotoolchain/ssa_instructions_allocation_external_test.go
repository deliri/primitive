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

func TestSSAInstructionsFirstObservationRetainsNoPopulation(t *testing.T) {
	testserial.Declare(t, core.TestIsolationDeclaration{Hazard: core.TestIsolationHazardRuntimeAllocation, Scope: core.TestIsolationScopePackageProcess})
	for _, population := range []int{32, 16385} {
		t.Run(fmt.Sprint(population), func(t *testing.T) {
			a := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;func invoke(){};func Entry(){`+strings.Repeat("invoke();", population)+`}`)
			err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
				r := gotoolchain.SSAInstructionRequest{Function: v.Package.Func("Entry"), Extent: gotoolchain.SSAInstructionExtentLexical}
				before, err := hostfacts.ObserveGoAllocationTotal(ctx)
				if err != nil {
					return err
				}
				observed := 0
				for instruction, err := range v.Instructions(ctx, r) {
					if err != nil || instruction == nil {
						t.Fatalf("first observation=(%v,%v), want native instruction and nil", instruction, err)
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
					t.Fatalf("first observation count/scratch=%d/%d at %d instructions, want 1 and <=4096 bytes", observed, allocated, population)
				}
				t.Logf("first observation scratch=%d at %d instructions; compiler construction excluded", allocated, population)
				return nil
			}})
			if err != nil {
				t.Fatalf("WithSSAUnit error=%v, want nil", err)
			}
		})
	}
}
