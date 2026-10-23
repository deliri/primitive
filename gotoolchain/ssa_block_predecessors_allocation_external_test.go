package gotoolchain_test

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gotoolchain"
	"github.com/deliri/primitive/v2026/hostfacts"
	"github.com/deliri/primitive/v2026/testserial"
	"golang.org/x/tools/go/ssa"
)

func nativePredecessorFanInSource(population int) string {
	var source strings.Builder
	source.WriteString(`package sample;func work(){};func finish(){};func Entry(choice int){switch choice{`)
	for index := 0; index < population; index++ {
		fmt.Fprintf(&source, `case %d:work();`, index)
	}
	source.WriteString(`};finish()}`)
	return source.String()
}

func TestSSABlockPredecessorsRetainNoIncomingPopulation(t *testing.T) {
	testserial.Declare(t, core.TestIsolationDeclaration{Hazard: core.TestIsolationHazardRuntimeAllocation, Scope: core.TestIsolationScopePackageProcess})
	for _, population := range []int{32, 8193} {
		t.Run(strconv.Itoa(population), func(t *testing.T) {
			a := nativeSymbolSourceAnalysis(t, t.TempDir(), nativePredecessorFanInSource(population))
			err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
				var join *ssa.BasicBlock
				for _, block := range v.Package.Func("Entry").Blocks {
					for _, instruction := range block.Instrs {
						if call, ok := instruction.(*ssa.Call); ok && call.Common().StaticCallee() != nil && call.Common().StaticCallee().Name() == "finish" {
							join = block
						}
					}
				}
				if join == nil || len(join.Preds) != population+1 {
					t.Fatalf("fan-in fixture got=%v, want %d incoming source edges", join, population+1)
				}
				before, err := hostfacts.ObserveGoAllocationTotal(ctx)
				if err != nil {
					return err
				}
				seen := 0
				for predecessor, queryErr := range v.BlockLinks(ctx, gotoolchain.SSABlockLinkRequest{Block: join, Kind: gotoolchain.SSABlockLinkPredecessors}) {
					if queryErr != nil || seen >= len(join.Preds) || predecessor != join.Preds[seen] {
						t.Fatalf("incoming[%d] got=%v,%v, want exact native order", seen, predecessor, queryErr)
					}
					seen++
				}
				after, err := hostfacts.ObserveGoAllocationTotal(ctx)
				if err != nil {
					return err
				}
				allocated := after.Uint64() - before.Uint64()
				t.Logf("native incoming edges=%d query bytes=%d; compiler construction excluded", population+1, allocated)
				if seen != population+1 || allocated > 128 {
					t.Fatalf("fan-in got edges=%d bytes=%d, want %d edges and at most128 bytes independent of population", seen, allocated, population+1)
				}
				return nil
			}})
			if err != nil {
				t.Fatalf("WithSSAUnit got=%v, want nil", err)
			}
		})
	}
}

func BenchmarkSSABlockPredecessors(b *testing.B) {
	b.ReportAllocs()
	const population = 8193
	a := nativeSymbolSourceAnalysis(b, b.TempDir(), nativePredecessorFanInSource(population))
	err := a.WithSSAUnit(b.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
		var join *ssa.BasicBlock
		for _, block := range v.Package.Func("Entry").Blocks {
			for _, instruction := range block.Instrs {
				if call, ok := instruction.(*ssa.Call); ok && call.Common().StaticCallee() != nil && call.Common().StaticCallee().Name() == "finish" {
					join = block
				}
			}
		}
		if join == nil || len(join.Preds) != population+1 {
			b.Fatalf("fan-in fixture got=%v, want %d edges", join, population+1)
		}
		r := gotoolchain.SSABlockLinkRequest{Block: join, Kind: gotoolchain.SSABlockLinkPredecessors}
		for b.Loop() {
			seen := 0
			for predecessor, queryErr := range v.BlockLinks(ctx, r) {
				if queryErr != nil || seen >= len(join.Preds) || predecessor != join.Preds[seen] {
					b.Fatalf("incoming[%d] got=%v,%v, want native edge", seen, predecessor, queryErr)
				}
				seen++
			}
			if seen != population+1 {
				b.Fatalf("incoming count got=%d, want %d", seen, population+1)
			}
		}
		b.ReportMetric(population+1, "incoming_edges/op")
		return nil
	}})
	if err != nil {
		b.Fatalf("WithSSAUnit got=%v, want nil", err)
	}
}
