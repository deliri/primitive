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
	"golang.org/x/tools/go/ssa"
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

func TestSSAInstructionsCompleteStreamAllocatesNoQueryStorage(t *testing.T) {
	testserial.Declare(t, core.TestIsolationDeclaration{Hazard: core.TestIsolationHazardRuntimeAllocation, Scope: core.TestIsolationScopePackageProcess})
	for _, row := range []struct {
		extent gotoolchain.SSAInstructionExtent
		body   string
		calls  int
	}{
		{gotoolchain.SSAInstructionExtentFunction, "invoke();", 1},
		{gotoolchain.SSAInstructionExtentLexical, "func(){invoke()}();", 2},
	} {
		for _, population := range []int{32, 8193} {
			t.Run(fmt.Sprintf("%s_%d", row.extent, population), func(t *testing.T) {
				a := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;func invoke(){};func Entry(){`+strings.Repeat(row.body, population)+`}`)
				err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
					request := gotoolchain.SSAInstructionRequest{Function: view.Package.Func("Entry"), Extent: row.extent}
					before, queryErr := hostfacts.ObserveGoAllocationTotal(ctx)
					if queryErr != nil {
						return queryErr
					}
					calls := 0
					for instruction, queryErr := range view.Instructions(ctx, request) {
						if queryErr != nil {
							return queryErr
						}
						switch instruction.(type) {
						case *ssa.Call, *ssa.Defer, *ssa.Go:
							calls++
						}
					}
					after, queryErr := hostfacts.ObserveGoAllocationTotal(ctx)
					if queryErr != nil {
						return queryErr
					}
					allocated := after.Uint64() - before.Uint64()
					if calls != population*row.calls || allocated != 0 {
						t.Fatalf("complete native stream calls/scratch=%d/%d, want %d/0", calls, allocated, population*row.calls)
					}
					return nil
				}})
				if err != nil {
					t.Fatalf("WithSSAUnit error=%v, want nil", err)
				}
			})
		}
	}
}
