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

func TestSSAInstructionsSelectedBlockAllocatesNoPopulation(t *testing.T) {
	testserial.Declare(t, core.TestIsolationDeclaration{Hazard: core.TestIsolationHazardRuntimeAllocation, Scope: core.TestIsolationScopePackageProcess})
	for _, population := range []int{32, 4097} {
		t.Run(fmt.Sprint(population), func(t *testing.T) {
			a := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;func invoke(){};func Entry(flag bool){`+strings.Repeat(`if flag{invoke()};`, population)+`}`)
			err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
				fn := v.Package.Func("Entry")
				if len(fn.Blocks) != 2*population+1 || len(fn.Blocks[0].Succs) != 2 {
					t.Fatalf("native blocks=%d, want %d and branch successors", len(fn.Blocks), 2*population+1)
				}
				selected := fn.Blocks[0].Succs[0]
				request := gotoolchain.SSAInstructionRequest{Function: fn, Extent: gotoolchain.SSAInstructionExtentBlock, Block: selected}
				before, err := hostfacts.ObserveGoAllocationTotal(ctx)
				if err != nil {
					return err
				}
				observed := 0
				for instruction, queryErr := range v.Instructions(ctx, request) {
					if queryErr != nil || observed >= len(selected.Instrs) || instruction != selected.Instrs[observed] {
						t.Fatalf("selected instruction=%v,%v at %d, want native pointer,nil", instruction, queryErr, observed)
					}
					observed++
				}
				after, err := hostfacts.ObserveGoAllocationTotal(ctx)
				if err != nil {
					return err
				}
				if allocated := after.Uint64() - before.Uint64(); observed != 2 || allocated != 0 {
					t.Fatalf("selected instructions/query storage=%d/%d, want 2/0", observed, allocated)
				}
				return nil
			}})
			if err != nil {
				t.Fatalf("WithSSAUnit error=%v, want nil", err)
			}
		})
	}
}

func BenchmarkSSAInstructionsSelectedBlock(b *testing.B) {
	b.ReportAllocs()
	const population = 257
	a := nativeSymbolSourceAnalysis(b, b.TempDir(), `package sample;func invoke(){};func Entry(flag bool){`+strings.Repeat(`if flag{invoke()};`, population)+`}`)
	err := a.WithSSAUnit(b.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
		fn := v.Package.Func("Entry")
		if len(fn.Blocks) != 2*population+1 || len(fn.Blocks[0].Succs) != 2 {
			b.Fatalf("native blocks=%d, want %d and branch successors", len(fn.Blocks), 2*population+1)
		}
		selected := fn.Blocks[0].Succs[0]
		request := gotoolchain.SSAInstructionRequest{Function: fn, Extent: gotoolchain.SSAInstructionExtentBlock, Block: selected}
		for b.Loop() {
			observed := 0
			for instruction, queryErr := range v.Instructions(ctx, request) {
				if queryErr != nil || observed >= len(selected.Instrs) || instruction != selected.Instrs[observed] {
					b.Fatalf("selected instruction=%v,%v at %d, want native pointer,nil", instruction, queryErr, observed)
				}
				observed++
			}
			if observed != 2 {
				b.Fatalf("selected instructions=%d, want 2", observed)
			}
		}
		b.ReportMetric(float64(len(fn.Blocks)), "input_blocks/op")
		b.ReportMetric(2, "instructions/op")
		return nil
	}})
	if err != nil {
		b.Fatalf("WithSSAUnit error=%v, want nil", err)
	}
}
