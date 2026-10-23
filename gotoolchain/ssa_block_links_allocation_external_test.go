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

func TestSSABlockLinksRetainNoAncestryPopulation(t *testing.T) {
	testserial.Declare(t, core.TestIsolationDeclaration{Hazard: core.TestIsolationHazardRuntimeAllocation, Scope: core.TestIsolationScopePackageProcess})
	for _, depth := range []int{32, 4097} {
		t.Run(strconv.Itoa(depth), func(t *testing.T) {
			a := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;func invoke(){};func Entry(flag bool){`+strings.Repeat(`if flag{`, depth)+`invoke()`+strings.Repeat(`}`, depth)+`}`)
			err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
				fn := v.Package.Func("Entry")
				var selected *ssa.BasicBlock
				for _, block := range fn.Blocks {
					for _, instruction := range block.Instrs {
						if call, ok := instruction.(*ssa.Call); ok && call.Common().StaticCallee() == v.Package.Func("invoke") {
							selected = block
						}
					}
				}
				if selected == nil || len(fn.Blocks) != depth+2 {
					t.Fatalf("nested fixture selected=%v blocks=%d, want call block and %d blocks", selected, len(fn.Blocks), depth+2)
				}
				for _, first := range []bool{true, false} {
					before, err := hostfacts.ObserveGoAllocationTotal(ctx)
					if err != nil {
						return err
					}
					observed := 0
					for block, queryErr := range v.BlockLinks(ctx, gotoolchain.SSABlockLinkRequest{Block: selected, Kind: gotoolchain.SSABlockLinkDominatorAncestry}) {
						if queryErr != nil || block == nil || block.Parent() != fn || !block.Dominates(selected) {
							t.Fatalf("native ancestor=%v,%v, want actual dominating block,nil", block, queryErr)
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
					want := depth + 1
					if first {
						want = 1
					}
					allocated := after.Uint64() - before.Uint64()
					t.Logf("native ancestry depth=%d prefix=%t observations=%d query allocation=%d; native compiler construction excluded", depth, first, observed, allocated)
					if observed != want || allocated > 256 {
						t.Fatalf("ancestry/query allocation=%d/%d, want %d and at most 256 bytes", observed, allocated, want)
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

func BenchmarkSSABlockLinks(b *testing.B) {
	b.ReportAllocs()
	const depth = 257
	a := nativeSymbolSourceAnalysis(b, b.TempDir(), `package sample;func invoke(){};func Entry(flag bool){`+strings.Repeat(`if flag{`, depth)+`invoke()`+strings.Repeat(`}`, depth)+`}`)
	err := a.WithSSAUnit(b.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
		fn := v.Package.Func("Entry")
		var selected *ssa.BasicBlock
		for _, block := range fn.Blocks {
			for _, instruction := range block.Instrs {
				if call, ok := instruction.(*ssa.Call); ok && call.Common().StaticCallee() == v.Package.Func("invoke") {
					selected = block
				}
			}
		}
		if selected == nil || len(fn.Blocks) != depth+2 {
			b.Fatalf("native fixture selected=%v blocks=%d, want call and %d blocks", selected, len(fn.Blocks), depth+2)
		}
		for b.Loop() {
			observed := 0
			for block, queryErr := range v.BlockLinks(ctx, gotoolchain.SSABlockLinkRequest{Block: selected, Kind: gotoolchain.SSABlockLinkDominatorAncestry}) {
				if queryErr != nil || block == nil || !block.Dominates(selected) {
					b.Fatalf("native ancestor=%v,%v, want dominating block,nil", block, queryErr)
				}
				observed++
			}
			if observed != depth+1 {
				b.Fatalf("native ancestors=%d, want %d", observed, depth+1)
			}
		}
		b.ReportMetric(depth+1, "ancestors/op")
		return nil
	}})
	if err != nil {
		b.Fatalf("WithSSAUnit error=%v, want nil", err)
	}
}
