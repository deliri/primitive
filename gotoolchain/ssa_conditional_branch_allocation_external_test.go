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
)

func TestSSAConditionalBranchRetainsNoQueryPopulation(t *testing.T) {
	testserial.Declare(t, core.TestIsolationDeclaration{Hazard: core.TestIsolationHazardRuntimeAllocation, Scope: core.TestIsolationScopePackageProcess})
	for _, population := range []int{32, 8193} {
		t.Run(strconv.Itoa(population), func(t *testing.T) {
			a := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;func consume(bool){};func work(){};func Entry(flag bool){a:=flag;`+strings.Repeat(`a=!a;consume(a);`, population)+`if a{work()}}`)
			err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
				fn := v.Package.Func("Entry")
				block := fn.Blocks[0]
				before, err := hostfacts.ObserveGoAllocationTotal(ctx)
				if err != nil {
					return err
				}
				got, queryErr := v.ConditionalBranch(ctx, gotoolchain.SSAConditionalBranchRequest{Block: block})
				after, err := hostfacts.ObserveGoAllocationTotal(ctx)
				if err != nil {
					return err
				}
				// A fixed native/runtime observation budget permits no population copy.
				allocated := after.Uint64() - before.Uint64()
				edge := block.Succs[population%2]
				t.Logf("native boolean operators=%d query allocation=%d; compiler construction excluded", population, allocated)
				if queryErr != nil || got.Condition != fn.Params[0] || got.WhenTrue != edge || allocated > 128 {
					t.Fatalf("query got=%+v,%v allocation=%d, want flag,edge=%d,nil and at most 128 bytes independent of operator population", got, queryErr, allocated, edge.Index)
				}
				return nil
			}})
			if err != nil {
				t.Fatalf("WithSSAUnit got=%v, want nil", err)
			}
		})
	}
}

func BenchmarkSSAConditionalBranch(b *testing.B) {
	b.ReportAllocs()
	const population = 8193
	a := nativeSymbolSourceAnalysis(b, b.TempDir(), `package sample;func consume(bool){};func work(){};func Entry(flag bool){a:=flag;`+strings.Repeat(`a=!a;consume(a);`, population)+`if a{work()}}`)
	err := a.WithSSAUnit(b.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
		fn := v.Package.Func("Entry")
		block := fn.Blocks[0]
		r := gotoolchain.SSAConditionalBranchRequest{Block: block}
		for b.Loop() {
			got, queryErr := v.ConditionalBranch(ctx, r)
			if queryErr != nil || got.Condition != fn.Params[0] || got.WhenTrue != block.Succs[1] {
				b.Fatalf("query got=%+v,%v, want flag and inverted true edge,nil", got, queryErr)
			}
		}
		b.ReportMetric(population, "native_operators/op")
		return nil
	}})
	if err != nil {
		b.Fatalf("WithSSAUnit got=%v, want nil", err)
	}
}
