package gotoolchain_test

import (
	"context"
	"fmt"
	"go/types"
	"strconv"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gotoolchain"
	"github.com/deliri/primitive/v2026/hostfacts"
	"github.com/deliri/primitive/v2026/testserial"
)

func nativeWideFieldReadSource(population int) string {
	var source strings.Builder
	source.WriteString("package sample;type S struct{")
	for index := 0; index < population; index++ {
		fmt.Fprintf(&source, "F%d int;", index)
	}
	fmt.Fprintf(&source, "};func Entry(s *S)int{return s.F%d}", population-1)
	return source.String()
}

func TestSSAFieldReadRetainsNoFieldPopulation(t *testing.T) {
	testserial.Declare(t, core.TestIsolationDeclaration{Hazard: core.TestIsolationHazardRuntimeAllocation, Scope: core.TestIsolationScopePackageProcess})
	for _, population := range []int{32, 8193} {
		t.Run(strconv.Itoa(population), func(t *testing.T) {
			a := nativeSymbolSourceAnalysis(t, t.TempDir(), nativeWideFieldReadSource(population))
			err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
				fn := view.Package.Func("Entry")
				read := nativeFieldReadInstruction(t, fn, true)
				record := view.Source.Types.Scope().Lookup("S").Type().Underlying().(*types.Struct)
				if record.NumFields() != population {
					t.Fatalf("native field width got=%d, want %d", record.NumFields(), population)
				}
				before, queryErr := hostfacts.ObserveGoAllocationTotal(ctx)
				if queryErr != nil {
					return queryErr
				}
				got, queryErr := view.FieldRead(ctx, gotoolchain.SSAFieldReadRequest{Read: read})
				if queryErr != nil || got.Base != fn.Params[0] || got.Field != record.Field(population-1) {
					t.Fatalf("wide field got=%+v,%v, want base=%v field=%v,nil", got, queryErr, fn.Params[0], record.Field(population-1))
				}
				after, queryErr := hostfacts.ObserveGoAllocationTotal(ctx)
				if queryErr != nil {
					return queryErr
				}
				allocated := after.Uint64() - before.Uint64()
				t.Logf("native field width=%d query bytes=%d; compiler construction excluded", population, allocated)
				if allocated > 128 {
					t.Fatalf("field width=%d query bytes=%d, want at most128 independent of record population", population, allocated)
				}
				return nil
			}})
			if err != nil {
				t.Fatalf("WithSSAUnit got=%v, want nil", err)
			}
		})
	}
}

func BenchmarkSSAFieldRead(b *testing.B) {
	b.ReportAllocs()
	const population = 8193
	a := nativeSymbolSourceAnalysis(b, b.TempDir(), nativeWideFieldReadSource(population))
	err := a.WithSSAUnit(b.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
		fn := view.Package.Func("Entry")
		read := nativeFieldReadInstruction(b, fn, true)
		record := view.Source.Types.Scope().Lookup("S").Type().Underlying().(*types.Struct)
		if record.NumFields() != population {
			b.Fatalf("native field width got=%d, want %d", record.NumFields(), population)
		}
		for b.Loop() {
			got, queryErr := view.FieldRead(ctx, gotoolchain.SSAFieldReadRequest{Read: read})
			if queryErr != nil || got.Base != fn.Params[0] || got.Field != record.Field(population-1) {
				b.Fatalf("field query got=%+v,%v, want actual base=%v final field=%v,nil", got, queryErr, fn.Params[0], record.Field(population-1))
			}
		}
		b.ReportMetric(population, "native_fields/op")
		return nil
	}})
	if err != nil {
		b.Fatalf("WithSSAUnit got=%v, want nil", err)
	}
}
