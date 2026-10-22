package gotoolchain_test

import (
	"context"
	"go/types"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gotoolchain"
	"github.com/deliri/primitive/v2026/hostfacts"
	"github.com/deliri/primitive/v2026/testserial"
)

func TestReceiverMethodGateRetainsNoSyntaxPopulation(t *testing.T) {
	testserial.Declare(t, core.TestIsolationDeclaration{Hazard: core.TestIsolationHazardRuntimeAllocation, Scope: core.TestIsolationScopePackageProcess})
	var small uint64
	for _, extent := range []int{32, 16385} {
		source := `package sample;type Record struct{Count int};func(Record)Inspect()error{return nil};func(v *Record)Apply()error{` + strings.Repeat(`_=v.Count;`, extent) + `return v.Inspect()}`
		analysis := nativeSymbolSourceAnalysis(t, t.TempDir(), source)
		unit := analysis.Units[0]
		record := unit.Types.Scope().Lookup("Record").Type()
		method := types.NewMethodSet(types.NewPointer(record)).Lookup(nil, "Apply").Obj().(*types.Func)
		target := types.NewMethodSet(record).Lookup(nil, "Inspect").Obj().(*types.Func)
		err := analysis.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: unit, SourceValues: true, Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
			request := gotoolchain.ReceiverMethodGateRequest{Analysis: analysis, Method: method, Target: target}
			if got, err := view.ReceiverMethodGate(ctx, request); err != nil || !got.Preserved {
				t.Fatalf("warm receiver gate=%+v/%v,want preserved target", got, err)
			}
			before, err := hostfacts.ObserveGoAllocationTotal(ctx)
			if err != nil {
				return err
			}
			got, err := view.ReceiverMethodGate(ctx, request)
			if err != nil || got.Validate() != nil || !got.Preserved {
				t.Fatalf("measured receiver gate=%+v/%v,want preserved target", got, err)
			}
			after, err := hostfacts.ObserveGoAllocationTotal(ctx)
			if err != nil {
				return err
			}
			allocated := after.Uint64() - before.Uint64()
			if extent == 32 {
				small = allocated
			}
			t.Logf("native syntax statements=%d query allocated bytes=%d baseline=%d", extent, allocated, small)
			if allocated > small+(64<<10) {
				t.Fatalf("receiver query allocated %d bytes for %d statements,want baseline %d plus at most64KiB without syntax population", allocated, extent, small)
			}
			return nil
		}})
		if err != nil {
			t.Fatal(err)
		}
	}
}
