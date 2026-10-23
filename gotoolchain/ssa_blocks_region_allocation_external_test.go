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

func TestSSABlocksSelectedRegionAllocatesNoPopulation(t *testing.T) {
	testserial.Declare(t, core.TestIsolationDeclaration{Hazard: core.TestIsolationHazardRuntimeAllocation, Scope: core.TestIsolationScopePackageProcess})
	for _, population := range []int{32, 4097} {
		t.Run(fmt.Sprint(population), func(t *testing.T) {
			a := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;func invoke(){};func Entry(flag bool){`+strings.Repeat(`if flag{invoke()};`, population)+`}`)
			err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
				fn := v.Package.Func("Entry")
				if len(fn.Blocks) != 2*population+1 {
					t.Fatalf("native blocks=%d, want %d", len(fn.Blocks), 2*population+1)
				}
				selected := fn.Blocks[len(fn.Blocks)-1]
				request := gotoolchain.SSABlockRequest{Function: fn, DominatedBy: selected}
				before, err := hostfacts.ObserveGoAllocationTotal(ctx)
				if err != nil {
					return err
				}
				observed := 0
				for block, queryErr := range v.Blocks(ctx, request) {
					if queryErr != nil || block != selected {
						t.Fatalf("selected block=%v,%v, want native final join,nil", block, queryErr)
					}
					observed++
				}
				after, err := hostfacts.ObserveGoAllocationTotal(ctx)
				if err != nil {
					return err
				}
				if allocated := after.Uint64() - before.Uint64(); observed != 1 || allocated != 0 {
					t.Fatalf("selected blocks/query storage=%d/%d, want 1/0", observed, allocated)
				}
				return nil
			}})
			if err != nil {
				t.Fatalf("WithSSAUnit error=%v, want nil", err)
			}
		})
	}
}

func BenchmarkSSABlocksSelectedRegion(b *testing.B) {
	b.ReportAllocs()
	const population = 257
	a := nativeSymbolSourceAnalysis(b, b.TempDir(), `package sample;func invoke(){};func Entry(flag bool){`+strings.Repeat(`if flag{invoke()};`, population)+`}`)
	err := a.WithSSAUnit(b.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
		fn := v.Package.Func("Entry")
		if len(fn.Blocks) != 2*population+1 {
			b.Fatalf("native blocks=%d, want %d", len(fn.Blocks), 2*population+1)
		}
		selected := fn.Blocks[len(fn.Blocks)-1]
		request := gotoolchain.SSABlockRequest{Function: fn, DominatedBy: selected}
		for b.Loop() {
			observed := 0
			for block, queryErr := range v.Blocks(ctx, request) {
				if queryErr != nil || block != selected {
					b.Fatalf("selected block=%v,%v, want native final join,nil", block, queryErr)
				}
				observed++
			}
			if observed != 1 {
				b.Fatalf("selected blocks=%d, want 1", observed)
			}
		}
		b.ReportMetric(float64(len(fn.Blocks)), "input_blocks/op")
		b.ReportMetric(1, "blocks/op")
		return nil
	}})
	if err != nil {
		b.Fatalf("WithSSAUnit error=%v, want nil", err)
	}
}
