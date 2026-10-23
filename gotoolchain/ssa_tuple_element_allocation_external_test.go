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

func TestSSATupleElementRetainsNoInstructionPopulation(t *testing.T) {
	testserial.Declare(t, core.TestIsolationDeclaration{Hazard: core.TestIsolationHazardRuntimeAllocation, Scope: core.TestIsolationScopePackageProcess})
	for _, population := range []int{32, 8193} {
		t.Run(strconv.Itoa(population), func(t *testing.T) {
			a := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;func work(){};func pair(flag bool)(int,bool){return 7,flag};func consume(int,bool){};func Entry(flag bool){`+strings.Repeat(`if flag{work()};`, population)+`n,ok:=pair(flag);consume(n,ok)}`)
			err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
				fn := v.Package.Func("Entry")
				var element *ssa.Extract
				for _, block := range fn.Blocks {
					for _, instruction := range block.Instrs {
						if current, ok := instruction.(*ssa.Extract); ok && current.Index == 1 {
							element = current
						}
					}
				}
				if element == nil {
					t.Fatalf("allocation fixture got=%v, want native bool result", element)
				}
				before, err := hostfacts.ObserveGoAllocationTotal(ctx)
				if err != nil {
					return err
				}
				got, queryErr := v.TupleElement(ctx, gotoolchain.SSATupleElementRequest{Element: element})
				after, err := hostfacts.ObserveGoAllocationTotal(ctx)
				if err != nil {
					return err
				}
				allocated := after.Uint64() - before.Uint64()
				t.Logf("native prefix branches=%d tuple query bytes=%d; compiler construction excluded", population, allocated)
				if queryErr != nil || got.Tuple != element.Tuple || got.Index != 1 || allocated > 128 {
					t.Fatalf("tuple allocation got=%+v,%v bytes=%d, want original producer,index 1 and at most 128 bytes independent of population", got, queryErr, allocated)
				}
				return nil
			}})
			if err != nil {
				t.Fatalf("WithSSAUnit got=%v, want nil", err)
			}
		})
	}
}

func BenchmarkSSATupleElement(b *testing.B) {
	b.ReportAllocs()
	const population = 8193
	a := nativeSymbolSourceAnalysis(b, b.TempDir(), `package sample;func work(){};func pair(flag bool)(int,bool){return 7,flag};func consume(int,bool){};func Entry(flag bool){`+strings.Repeat(`if flag{work()};`, population)+`n,ok:=pair(flag);consume(n,ok)}`)
	err := a.WithSSAUnit(b.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
		var element *ssa.Extract
		for _, block := range v.Package.Func("Entry").Blocks {
			for _, instruction := range block.Instrs {
				if current, ok := instruction.(*ssa.Extract); ok && current.Index == 1 {
					element = current
				}
			}
		}
		if element == nil {
			b.Fatalf("benchmark fixture got=%v, want native bool result", element)
		}
		r := gotoolchain.SSATupleElementRequest{Element: element}
		for b.Loop() {
			got, queryErr := v.TupleElement(ctx, r)
			if queryErr != nil || got.Tuple != element.Tuple || got.Index != 1 {
				b.Fatalf("tuple got=%+v,%v, want original producer,index 1,nil", got, queryErr)
			}
		}
		b.ReportMetric(population, "native_branches/op")
		return nil
	}})
	if err != nil {
		b.Fatalf("WithSSAUnit got=%v, want nil", err)
	}
}
