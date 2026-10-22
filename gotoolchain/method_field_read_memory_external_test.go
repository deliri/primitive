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

func TestMethodFieldReadAllocatesNoSyntaxPopulation(t *testing.T) {
	testserial.Declare(t, core.TestIsolationDeclaration{Hazard: core.TestIsolationHazardRuntimeAllocation, Scope: core.TestIsolationScopePackageProcess})
	var small uint64
	for _, extent := range []int{32, 16385} {
		analysis := nativeSymbolSourceAnalysis(t, t.TempDir(), "package sample\ntype Record struct{A,B int}\nfunc(v Record)Inspect(){"+strings.Repeat("_=v.B;", extent)+"}")
		unit := analysis.Units[0]
		method := types.NewMethodSet(unit.Types.Scope().Lookup("Record").Type()).Lookup(nil, "Inspect").Obj().(*types.Func)
		err := analysis.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: unit, SourceValues: true, Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
			request := gotoolchain.MethodFieldReadRequest{Analysis: analysis, Method: method}
			if got, err := view.MethodFieldRead(ctx, request); err != nil || got.Read || got.Source != gotoolchain.FunctionSourcePresent {
				t.Fatalf("warm query=%+v/%v,want present source and unread first field", got, err)
			}
			before, err := hostfacts.ObserveGoAllocationTotal(ctx)
			if err != nil {
				return err
			}
			got, err := view.MethodFieldRead(ctx, request)
			if err != nil || got.Read || got.Source != gotoolchain.FunctionSourcePresent {
				t.Fatalf("query=%+v/%v,want present source and unread first field", got, err)
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
				t.Fatalf("native query allocated %d bytes for %d statements,want baseline %d plus at most64KiB without a syntax population", allocated, extent, small)
			}
			return nil
		}})
		if err != nil {
			t.Fatal(err)
		}
	}
}
