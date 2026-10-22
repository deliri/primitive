package gotoolchain_test

import (
	"context"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gotoolchain"
	"github.com/deliri/primitive/v2026/hostfacts"
	"github.com/deliri/primitive/v2026/testserial"
)

func TestSSASynchronousCallsRetainNoInstructionPopulation(t *testing.T) {
	testserial.Declare(t, core.TestIsolationDeclaration{Hazard: core.TestIsolationHazardRuntimeAllocation, Scope: core.TestIsolationScopePackageProcess})
	var small uint64
	for _, extent := range []int{32, 16385} {
		analysis := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;func invoke(){};func Entry(){`+strings.Repeat(`invoke();`, extent)+`}`)
		unit := analysis.Units[0]
		err := analysis.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: unit, Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
			request := gotoolchain.SSASynchronousCallRequest{Function: view.Package.Func("Entry"), Timing: gotoolchain.SSACallTimingSynchronous}
			before, err := hostfacts.ObserveGoAllocationTotal(ctx)
			if err != nil {
				return err
			}
			calls := 0
			for call, err := range view.SynchronousCalls(ctx, request) {
				if err != nil || call == nil {
					t.Fatalf("native call=(%v, %v), want actual call and nil", call, err)
				}
				calls++
				break
			}
			after, err := hostfacts.ObserveGoAllocationTotal(ctx)
			if err != nil {
				return err
			}
			allocated := after.Uint64() - before.Uint64()
			if extent == 32 {
				small = allocated
			}
			if calls != 1 || allocated > small+(64<<10) {
				t.Fatalf("first call count=%d allocated=%d at %d instructions, want 1 and baseline %d plus64KiB", calls, allocated, extent, small)
			}
			t.Logf("instructions=%d first-call scratch=%d bytes baseline=%d", extent, allocated, small)
			return nil
		}})
		if err != nil {
			t.Fatalf("WithSSAUnit(call-stream allocation) error = %v, want nil", err)
		}
	}
}
