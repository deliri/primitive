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

func TestSSABlocksAllocateNoBlockPopulation(t *testing.T) {
	testserial.Declare(t, core.TestIsolationDeclaration{Hazard: core.TestIsolationHazardRuntimeAllocation, Scope: core.TestIsolationScopePackageProcess})
	for _, population := range []int{32, 4097} {
		t.Run(fmt.Sprint(population), func(t *testing.T) {
			a := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;func invoke(){};func Entry(flag bool){`+strings.Repeat(`if flag{invoke()};`, population)+`}`)
			err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
				fn := v.Package.Func("Entry")
				if len(fn.Blocks) != 2*population+1 {
					t.Fatalf("native blocks=%d, want %d", len(fn.Blocks), 2*population+1)
				}
				request := gotoolchain.SSABlockRequest{Function: fn}
				for _, first := range []bool{true, false} {
					before, err := hostfacts.ObserveGoAllocationTotal(ctx)
					if err != nil {
						return err
					}
					observed := 0
					for block, queryErr := range v.Blocks(ctx, request) {
						if queryErr != nil || block != fn.Blocks[observed] {
							t.Fatalf("native block=%v,%v, want actual block %d,nil", block, queryErr, observed)
						}
						observed++
						if first {
							break
						}
					}
					after, err := hostfacts.ObserveGoAllocationTotal(ctx)
					if err != nil {
						return err
					}
					want := len(fn.Blocks)
					if first {
						want = 1
					}
					if allocated := after.Uint64() - before.Uint64(); observed != want || allocated != 0 {
						t.Fatalf("native blocks/query storage=%d/%d, want %d/0", observed, allocated, want)
					}
				}
				return nil
			}})
			if err != nil {
				t.Fatalf("WithSSAUnit error=%v, want nil", err)
			}
		})
	}
}
