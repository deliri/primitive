package gotoolchain_test

import (
	"context"
	"fmt"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gotoolchain"
	"github.com/deliri/primitive/v2026/hostfacts"
	"github.com/deliri/primitive/v2026/testserial"
	"golang.org/x/tools/go/ssa"
	"strings"
	"testing"
)

func TestSSAPossibleSuccessorsRetainNoInstructionPopulation(t *testing.T) {
	testserial.Declare(t, core.TestIsolationDeclaration{Hazard: core.TestIsolationHazardRuntimeAllocation, Scope: core.TestIsolationScopePackageProcess})
	var source strings.Builder
	source.WriteString(`package sample;func noise(){};func work(){};`)
	for _, population := range []int{32, 8193} {
		fmt.Fprintf(&source, "func Entry%d(){%sif true{work()};work()};", population, strings.Repeat("noise();", population))
	}
	a := nativeSymbolSourceAnalysis(t, t.TempDir(), source.String())
	err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
		for _, population := range []int{32, 8193} {
			fn := v.Package.Func(fmt.Sprintf("Entry%d", population))
			entry := fn.Blocks[0]
			calls := 0
			for _, instruction := range entry.Instrs {
				if call, ok := instruction.(*ssa.Call); ok && call.Common().StaticCallee() == v.Package.Func("noise") {
					calls++
				}
			}
			if calls != population || len(entry.Succs) != 2 {
				t.Fatalf("native input got=%d calls/%d edges, want %d/2", calls, len(entry.Succs), population)
			}
			start, err := hostfacts.ObserveGoAllocationTotal(ctx)
			if err != nil {
				return err
			}
			seen := 0
			for block, queryErr := range v.BlockLinks(ctx, gotoolchain.SSABlockLinkRequest{Block: entry, Kind: gotoolchain.SSABlockLinkPossibleSuccessors}) {
				if queryErr != nil || block != entry.Succs[0] || seen != 0 {
					t.Fatalf("possible edge got=%v,%v count=%d, want selected native edge,nil", block, queryErr, seen)
				}
				seen++
			}
			end, err := hostfacts.ObserveGoAllocationTotal(ctx)
			if err != nil {
				return err
			}
			allocated := end.Uint64() - start.Uint64()
			t.Logf("native prefix=%d query allocation=%d; compiler construction excluded", population, allocated)
			if seen != 1 || allocated != 0 {
				t.Fatalf("possible edge query got count=%d allocation=%d, want 1/0", seen, allocated)
			}
		}
		return nil
	}})
	if err != nil {
		t.Fatalf("WithSSAUnit got=%v, want nil", err)
	}
}
func BenchmarkSSAPossibleSuccessors(b *testing.B) {
	b.ReportAllocs()
	const population = 8193
	a := nativeSymbolSourceAnalysis(b, b.TempDir(), `package sample;func noise(){};func work(){};func Entry(){`+strings.Repeat("noise();", population)+`if true{work()};work()}`)
	err := a.WithSSAUnit(b.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
		entry := v.Package.Func("Entry").Blocks[0]
		calls := 0
		for _, instruction := range entry.Instrs {
			if call, ok := instruction.(*ssa.Call); ok && call.Common().StaticCallee() == v.Package.Func("noise") {
				calls++
			}
		}
		if calls != population || len(entry.Succs) != 2 {
			b.Fatalf("native workload got=%d calls/%d edges, want %d/2", calls, len(entry.Succs), population)
		}
		for b.Loop() {
			seen := 0
			for block, queryErr := range v.BlockLinks(ctx, gotoolchain.SSABlockLinkRequest{Block: entry, Kind: gotoolchain.SSABlockLinkPossibleSuccessors}) {
				if queryErr != nil || block != entry.Succs[0] || seen != 0 {
					b.Fatalf("possible edge got=%v,%v count=%d, want selected edge,nil", block, queryErr, seen)
				}
				seen++
			}
			if seen != 1 {
				b.Fatalf("possible edge count got=%d, want one", seen)
			}
		}
		b.ReportMetric(population, "prefix_calls/op")
		return nil
	}})
	if err != nil {
		b.Fatalf("WithSSAUnit got=%v, want nil", err)
	}
}
