package gotoolchain_test

import (
	"context"
	"errors"
	"go/ast"
	"io"
	"strconv"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gotoolchain"
	"github.com/deliri/primitive/v2026/hostfacts"
	"github.com/deliri/primitive/v2026/testserial"
	"golang.org/x/tools/go/ssa"
)

func TestMapIterationNativeExpressionMatrix(t *testing.T) {
	t.Parallel()
	shapes := []struct {
		name   string
		source func(string, string) string
	}{
		{"parameter", func(typ, zero string) string { return "func F(v " + typ + "){for range v{}}" }},
		{"named", func(typ, zero string) string { return "type Values " + typ + ";func F(v Values){for range v{}}" }},
		{"alias", func(typ, zero string) string { return "type Values=" + typ + ";func F(v Values){for range v{}}" }},
		{"field", func(typ, zero string) string {
			return "type Record struct{Values " + typ + "};func F(r Record){for range r.Values{}}"
		}},
		{"function", func(typ, zero string) string {
			return "func values()" + typ + "{return " + zero + "};func F(){for range values(){}}"
		}},
		{"tuple", func(typ, zero string) string {
			return "func values()(int," + typ + "){return 0," + zero + "};func F(){_,v:=values();for range v{}}"
		}},
		{"dereference", func(typ, zero string) string { return "func F(p *" + typ + "){for range *p{}}" }},
		{"copy", func(typ, zero string) string { return "func F(v " + typ + "){next:=v;for range next{}}" }},
	}
	for _, kind := range []struct {
		name, typ, zero string
		want            int
	}{
		{"string_key_map", "map[string]int", "nil", 1},
		{"string_shaped_map", "map[int]rune", "nil", 1},
		{"string", "string", "\"\"", 0},
		{"slice", "[]rune", "nil", 0},
	} {
		for _, shape := range shapes {
			t.Run(kind.name+"/"+shape.name, func(t *testing.T) {
				t.Parallel()
				source := "package sample\n//line invented.go:900\n" + shape.source(kind.typ, kind.zero)
				analysis := nativeSymbolSourceAnalysis(t, t.TempDir(), source)
				count := 0
				for observation, err := range analysis.MapIterations(t.Context()) {
					if err != nil {
						t.Fatal(err)
					}
					offset := strings.Index(source, "for range")
					column := offset - strings.LastIndex(source[:offset], "\n")
					if observation.Validate() != nil || !strings.HasSuffix(observation.File.String(), "/source.go") || observation.Line != 3 || observation.Column != uint64(column) || observation.Offset != uint64(offset) {
						t.Fatalf("native iteration coordinate got%+v want source.go:3:%d offset%d", observation, column, offset)
					}
					count++
				}
				if count != kind.want {
					t.Fatalf("native map iterations got%d want%d", count, kind.want)
				}
			})
		}
	}
}

func TestMapIterationNativeGenericAndFunctionOwnership(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name, source string
		want         int
	}{
		{"generic_map", "func F[M ~map[string]int](v M){for range v{}}", 1},
		{"generic_string_shaped_map", "func F[M ~map[int]rune](v M){for range v{}}", 1},
		{"generic_string", "func F[S ~string](v S){for range v{}}", 0},
		{"generic_slice", "func F[S ~[]int](v S){for range v{}}", 0},
		{"named_union", "type A map[string]int;type B map[string]int;func F[M A|B](v M){for range v{}}", 1},
		{"map_intersection", "func F[M interface{~map[string]int|~[]int;~map[string]int}](v M){for range v{}}", 1},
		{"slice_intersection", "func F[S interface{~map[string]int|~[]int;~[]int}](v S){for range v{}}", 0},
		{"method_constraint", "type Values map[string]int;func(Values)Marker(){};func F[M interface{~map[string]int;Marker()}](v M){for range v{}}", 1},
		{"generic_receiver", "type Box[T comparable]struct{Values map[T]int};func(b Box[T])F(){for range b.Values{}}", 1},
		{"initializer_closure", "var F=func(){for range map[int]rune{}}", 1},
		{"native_init", "func init(){for range map[int]rune{}}", 1},
		{"method_body", "type Record struct{Values map[int]rune};func(r Record)F(){for range r.Values{}}", 1},
		{"nested_closures", "func F(){_=func(){for range map[int]rune{};_=func(){for range map[int]rune{}}}}", 2},
		{"non_map_ranges", "func F(){for range \"abc\"{};for range []int{}{};for range [2]int{}{};for range 3{}}", 0},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			analysis := nativeSymbolSourceAnalysis(t, t.TempDir(), "package sample;"+row.source)
			count := 0
			for observation, err := range analysis.MapIterations(t.Context()) {
				if err != nil || observation.Validate() != nil {
					t.Fatalf("native iteration got%+v/%v", observation, err)
				}
				count++
			}
			if count != row.want {
				t.Fatalf("native function map iterations got%d want%d", count, row.want)
			}
		})
	}
}

func TestMapIterationConsumerAndNativeCauseCustody(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name                     string
		stop, before, cancel     bool
		wantLoans, wantTerminals int
	}{
		{name: "complete", wantLoans: 2},
		{name: "consumer stops", stop: true, wantLoans: 1},
		{name: "cancel before", before: true, wantTerminals: 1},
		{name: "cancel between loans", cancel: true, wantLoans: 1, wantTerminals: 1},
		{name: "consumer stop owns cancellation", stop: true, cancel: true, wantLoans: 1},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			analysis := nativeSymbolSourceAnalysis(t, t.TempDir(), "package sample;func F(v map[string]int){for range v{};for range v{}}")
			ctx, cancel := context.WithCancelCause(t.Context())
			defer cancel(nil)
			if row.before {
				cancel(io.ErrClosedPipe)
			}
			loans, terminals := 0, 0
			for observation, err := range analysis.MapIterations(ctx) {
				if err != nil {
					if observation != (gotoolchain.MapIterationObservation{}) || !errors.Is(err, io.ErrClosedPipe) {
						t.Fatalf("native cause got%+v/%v", observation, err)
					}
					terminals++
					continue
				}
				if observation.Validate() != nil {
					t.Fatalf("invalid native observation %+v", observation)
				}
				loans++
				if row.cancel {
					cancel(io.ErrClosedPipe)
				}
				if row.stop {
					break
				}
			}
			if loans != row.wantLoans || terminals != row.wantTerminals {
				t.Fatalf("native loans/terminals got%d/%d want%d/%d", loans, terminals, row.wantLoans, row.wantTerminals)
			}
		})
	}
}

func TestMapIterationMissingFactsCannotBecomeCleanObservation(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name                                     string
		before, missingSyntax, missingDefinition bool
	}{
		{name: "incomplete analysis", before: true},
		{name: "missing syntax", missingSyntax: true},
		{name: "missing native function", missingDefinition: true},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			analysis := nativeSymbolSourceAnalysis(t, t.TempDir(), "package sample;func F(v map[string]int){for range v{}}")
			terminals := 0
			consume := func(observation gotoolchain.MapIterationObservation, err error) bool {
				if observation != (gotoolchain.MapIterationObservation{}) || !errors.Is(err, core.ErrGoToolchainOutput) {
					t.Fatalf("missing native facts got%+v/%v want zero/output", observation, err)
				}
				terminals++
				return true
			}
			if row.before {
				analysis.Incomplete = true
				analysis.MapIterations(t.Context())(consume)
			} else {
				unit := analysis.Units[0]
				err := analysis.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: unit, Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
					if row.missingSyntax {
						unit.Syntax = nil
					} else {
						function := unit.Syntax[0].Decls[0].(*ast.FuncDecl)
						delete(unit.TypesInfo.Defs, function.Name)
					}
					view.MapIterations(ctx)(consume)
					return nil
				}})
				if err != nil {
					t.Fatal(err)
				}
			}
			if terminals != 1 {
				t.Fatalf("native refusal terminals got%d want1", terminals)
			}
		})
	}
}

func TestMapIterationLiveLoanKeepsFixedMemory(t *testing.T) {
	testserial.Declare(t, core.TestIsolationDeclaration{Hazard: core.TestIsolationHazardRuntimeAllocation, Scope: core.TestIsolationScopePackageProcess})
	for _, population := range []int{64, 4097} {
		t.Run(strconv.Itoa(population), func(t *testing.T) {
			source := "package sample;func F(v map[int]rune){" + strings.Repeat("for range v{};", population) + "}"
			analysis := nativeSymbolSourceAnalysis(t, t.TempDir(), source)
			err := analysis.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: analysis.Units[0], Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
				before, err := hostfacts.ObserveGoAllocationTotal(ctx)
				if err != nil {
					return err
				}
				count := 0
				for observation, err := range view.MapIterations(ctx) {
					if err != nil || observation.Validate() != nil {
						t.Fatalf("native memory observation %+v/%v", observation, err)
					}
					count++
					if count == 1 {
						during, err := hostfacts.ObserveGoAllocationTotal(ctx)
						if err != nil {
							return err
						}
						allocated := during.Uint64() - before.Uint64()
						t.Logf("map iteration population%d first-loan bytes%d", population, allocated)
						if allocated > 32768 {
							t.Fatalf("native iteration allocated%d want at most32768", allocated)
						}
					}
				}
				if count != population {
					t.Fatalf("native iterator conservation got%d want%d", count, population)
				}
				return nil
			}})
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestMapIterationConsumerOwnsNextNativeLoan(t *testing.T) {
	t.Parallel()
	analysis := nativeSymbolSourceAnalysis(t, t.TempDir(), "package sample;func F(v map[int]rune){for range v{};for range v{}}")
	err := analysis.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: analysis.Units[0], Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
		loans, changed := 0, 0
		for observation, err := range view.MapIterations(ctx) {
			if err != nil {
				return err
			}
			loans++
			for _, block := range view.Package.Func("F").Blocks {
				for _, instruction := range block.Instrs {
					if next, ok := instruction.(*ssa.Next); ok {
						if iterator, ok := next.Iter.(*ssa.Range); ok && uint64(view.Source.Fset.PositionFor(iterator.Pos(), false).Offset) > observation.Offset {
							next.IsString = true
							changed++
						}
					}
				}
			}
		}
		if loans != 1 || changed != 1 {
			t.Fatalf("native loan/projection changed got%d/%d want1/1", loans, changed)
		}
		return nil
	}})
	if err != nil {
		t.Fatal(err)
	}
}

func FuzzMapIterationNativeBindingConservation(f *testing.F) {
	f.Add(uint8(0), uint8(1))
	f.Add(uint8(3), uint8(4))
	f.Fuzz(func(t *testing.T, mask, extent uint8) {
		count := int(extent&7) + 1
		outer, inner := "[]int{}", "[]int{}"
		if mask&1 != 0 {
			outer = "map[int]rune{}"
		}
		if mask&2 != 0 {
			inner = "map[int]rune{}"
		}
		source := "package sample;func F(){v:=" + outer + ";" + strings.Repeat("for range v{};", count) + "{v:=" + inner + ";" + strings.Repeat("for range v{};", count) + "};" + strings.Repeat("for range v{};", count) + "}"
		analysis := nativeSymbolSourceAnalysis(t, t.TempDir(), source)
		got := 0
		for observation, err := range analysis.MapIterations(t.Context()) {
			if err != nil || observation.Validate() != nil {
				t.Fatalf("native fuzz observation%+v/%v", observation, err)
			}
			got++
		}
		want := 0
		if mask&1 != 0 {
			want += 2 * count
		}
		if mask&2 != 0 {
			want += count
		}
		if got != want {
			t.Fatalf("native mask%d extent%d iterations%d want%d source%s", mask, extent, got, want, source)
		}
	})
}
