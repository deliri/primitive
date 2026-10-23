package gotoolchain_test

import (
	"context"
	"strconv"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gotoolchain"
	"github.com/deliri/primitive/v2026/hostfacts"
	"github.com/deliri/primitive/v2026/testserial"
	"golang.org/x/tools/go/ssa"
)

func TestSSAInstructionsTerminatorRetainsNoPrefixPopulation(t *testing.T) {
	testserial.Declare(t, core.TestIsolationDeclaration{Hazard: core.TestIsolationHazardRuntimeAllocation, Scope: core.TestIsolationScopePackageProcess})
	for _, population := range []int{32, 8193} {
		t.Run(strconv.Itoa(population), func(t *testing.T) {
			a := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;func invoke(){};func Entry(){`+strings.Repeat(`invoke();`, population)+`}`)
			err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
				fn := v.Package.Func("Entry")
				if len(fn.Blocks) != 1 || len(fn.Blocks[0].Instrs) != population+1 {
					t.Fatalf("native block/instruction count=%d/%d, want 1/%d", len(fn.Blocks), len(fn.Blocks[0].Instrs), population+1)
				}
				r := gotoolchain.SSAInstructionRequest{Function: fn, Block: fn.Blocks[0], Extent: gotoolchain.SSAInstructionExtentBlockTerminator}
				before, err := hostfacts.ObserveGoAllocationTotal(ctx)
				if err != nil {
					return err
				}
				observed := 0
				for instruction, err := range v.Instructions(ctx, r) {
					if _, ok := instruction.(*ssa.Return); err != nil || !ok || instruction.Block() != r.Block {
						t.Fatalf("native terminator=%v,%v, want selected Return,nil", instruction, err)
					}
					observed++
				}
				after, err := hostfacts.ObserveGoAllocationTotal(ctx)
				if err != nil {
					return err
				}
				allocated := after.Uint64() - before.Uint64()
				t.Logf("native prefix=%d terminator query bytes=%d; compiler construction excluded", population, allocated)
				if observed != 1 || allocated != 0 {
					t.Fatalf("terminator count/query bytes=%d/%d, want 1/0", observed, allocated)
				}
				return nil
			}})
			if err != nil {
				t.Fatalf("WithSSAUnit error=%v, want nil", err)
			}
		})
	}
}

func BenchmarkSSAInstructionsBlockTerminator(b *testing.B) {
	b.ReportAllocs()
	const population = 8193
	a := nativeSymbolSourceAnalysis(b, b.TempDir(), `package sample;func invoke(){};func Entry(){`+strings.Repeat(`invoke();`, population)+`}`)
	err := a.WithSSAUnit(b.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
		fn := v.Package.Func("Entry")
		if len(fn.Blocks[0].Instrs) != population+1 {
			b.Fatalf("native instructions=%d, want %d", len(fn.Blocks[0].Instrs), population+1)
		}
		r := gotoolchain.SSAInstructionRequest{Function: fn, Block: fn.Blocks[0], Extent: gotoolchain.SSAInstructionExtentBlockTerminator}
		for b.Loop() {
			observed := 0
			for instruction, err := range v.Instructions(ctx, r) {
				if _, ok := instruction.(*ssa.Return); err != nil || !ok || instruction.Block() != r.Block {
					b.Fatalf("native terminator=%v,%v, want selected Return,nil", instruction, err)
				}
				observed++
			}
			if observed != 1 {
				b.Fatalf("native terminators=%d, want 1", observed)
			}
		}
		b.ReportMetric(population, "preceding_instructions/op")
		return nil
	}})
	if err != nil {
		b.Fatalf("WithSSAUnit error=%v, want nil", err)
	}
}
