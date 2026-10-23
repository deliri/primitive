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

func TestSSABlockDominanceRetainsNoFunctionPopulation(t *testing.T) {
	testserial.Declare(t, core.TestIsolationDeclaration{Hazard: core.TestIsolationHazardRuntimeAllocation, Scope: core.TestIsolationScopePackageProcess})
	for _, population := range []int{32, 8193} {
		t.Run(strconv.Itoa(population), func(t *testing.T) {
			a := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;func work(){};func Entry(flag bool){`+strings.Repeat(`if flag{work()};`, population)+`work()}`)
			err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
				fn := v.Package.Func("Entry")
				r := gotoolchain.SSABlockDominanceRequest{Dominator: fn.Blocks[0], Block: fn.Blocks[len(fn.Blocks)-1]}
				before, err := hostfacts.ObserveGoAllocationTotal(ctx)
				if err != nil {
					return err
				}
				got, queryErr := v.BlockDominates(ctx, r)
				after, err := hostfacts.ObserveGoAllocationTotal(ctx)
				if err != nil {
					return err
				}
				allocated := after.Uint64() - before.Uint64()
				t.Logf("native branches=%d relation allocation=%d; compiler construction excluded", population, allocated)
				if queryErr != nil || !got.Proven || allocated != 0 {
					t.Fatalf("native relation got=%+v,%v allocation=%d, want proven,nil and zero bytes", got, queryErr, allocated)
				}
				return nil
			}})
			if err != nil {
				t.Fatalf("WithSSAUnit error got=%v, want nil", err)
			}
		})
	}
}

func BenchmarkSSABlockDominance(b *testing.B) {
	b.ReportAllocs()
	const population = 8193
	a := nativeSymbolSourceAnalysis(b, b.TempDir(), `package sample;func work(){};func Entry(flag bool){`+strings.Repeat(`if flag{work()};`, population)+`work()}`)
	err := a.WithSSAUnit(b.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
		fn := v.Package.Func("Entry")
		r := gotoolchain.SSABlockDominanceRequest{Dominator: fn.Blocks[0], Block: fn.Blocks[len(fn.Blocks)-1]}
		for b.Loop() {
			got, queryErr := v.BlockDominates(ctx, r)
			if queryErr != nil || !got.Proven {
				b.Fatalf("native relation got=%+v,%v, want proven,nil", got, queryErr)
			}
		}
		b.ReportMetric(float64(population), "native_branches/op")
		return nil
	}})
	if err != nil {
		b.Fatalf("WithSSAUnit error got=%v, want nil", err)
	}
}
