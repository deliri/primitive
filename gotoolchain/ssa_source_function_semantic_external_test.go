package gotoolchain_test

import (
	"context"
	"errors"
	"fmt"
	"go/ast"
	"strconv"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gotoolchain"
	"github.com/deliri/primitive/v2026/hostfacts"
	"github.com/deliri/primitive/v2026/testserial"
	"golang.org/x/tools/go/ssa"
)

func nativeSourceFunctionMode(t testing.TB, directory string, mode uint8, copied bool) {
	t.Helper()
	mode %= 12
	var declarations strings.Builder
	for index := 0; index < 12; index++ {
		fmt.Fprintf(&declarations, "func Entry%d()int{return %d};", index, index)
	}
	a := nativeSymbolSourceAnalysis(t, directory, "package sample;"+declarations.String())
	err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
		actual := v.Source.Syntax[0].Decls[int(mode)].(*ast.FuncDecl)
		selected := actual
		if copied {
			clone := *actual
			selected = &clone
			if selected == actual || selected.Name != actual.Name {
				t.Fatal("declaration identity mutation changed the wrong native fact")
			}
		}
		got, queryErr := v.SourceFunction(ctx, gotoolchain.SSASourceFunctionRequest{Kind: gotoolchain.SSASourceFunctionDeclaration, Declaration: selected})
		if copied {
			if got != (gotoolchain.SSASourceFunctionObservation{}) || !errors.Is(queryErr, core.ErrGoToolchainContract) {
				t.Fatalf("source mode%d copied declaration=%+v,%v, want zero and contract refusal", mode, got, queryErr)
			}
			return nil
		}
		if queryErr != nil || got.Validate() != nil || got.Function.Syntax() != actual || got.Function.Name() != "Entry"+strconv.Itoa(int(mode)) {
			t.Fatalf("source mode%d native function=%+v,%v, want exact source-selected function", mode, got, queryErr)
		}
		returned := 0
		for instruction, instructionErr := range v.Instructions(ctx, gotoolchain.SSAInstructionRequest{Function: got.Function, Extent: gotoolchain.SSAInstructionExtentFunction}) {
			if instructionErr != nil {
				t.Fatal(instructionErr)
			}
			result, ok := instruction.(*ssa.Return)
			if !ok {
				continue
			}
			if len(result.Results) != 1 {
				t.Fatal("native source return has wrong width")
			}
			value, ok := result.Results[0].(*ssa.Const)
			if !ok || value.Value == nil || value.Value.ExactString() != strconv.Itoa(int(mode)) {
				t.Fatalf("source mode%d borrowed another native return: %v", mode, result.Results[0])
			}
			returned++
		}
		if returned != 1 {
			t.Fatalf("source mode%d returned observations=%d, want1", mode, returned)
		}
		return nil
	}})
	if err != nil {
		t.Fatal(err)
	}
}

func TestSSASourceFunctionFiniteNativeSelectionDomain(t *testing.T) {
	t.Parallel()
	for mode := uint8(0); mode < 12; mode++ {
		for _, copied := range []bool{false, true} {
			t.Run(fmt.Sprintf("%d/copied=%t", mode, copied), func(t *testing.T) { t.Parallel(); nativeSourceFunctionMode(t, t.TempDir(), mode, copied) })
		}
	}
}

func FuzzSSASourceFunctionConservesDeclarationIdentityAndReturn(f *testing.F) {
	f.Add(uint8(0), false)
	f.Fuzz(func(t *testing.T, mode uint8, copied bool) { nativeSourceFunctionMode(t, t.TempDir(), mode, copied) })
}

func TestSSASourceFunctionLookupRetainsNoDeclarationPopulation(t *testing.T) {
	testserial.Declare(t, core.TestIsolationDeclaration{Hazard: core.TestIsolationHazardRuntimeAllocation, Scope: core.TestIsolationScopePackageProcess})
	for _, width := range []int{32, 8193} {
		t.Run(strconv.Itoa(width), func(t *testing.T) {
			var declarations strings.Builder
			for index := 0; index < width; index++ {
				fmt.Fprintf(&declarations, "func Entry%d(){};", index)
			}
			a := nativeSymbolSourceAnalysis(t, t.TempDir(), "package sample;"+declarations.String())
			err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
				if len(v.Source.Syntax[0].Decls) != width {
					t.Fatal("wide declaration fixture lost native source width")
				}
				decl := v.Source.Syntax[0].Decls[width-1].(*ast.FuncDecl)
				request := gotoolchain.SSASourceFunctionRequest{Kind: gotoolchain.SSASourceFunctionDeclaration, Declaration: decl}
				before, err := hostfacts.ObserveGoAllocationTotal(ctx)
				if err != nil {
					return err
				}
				got, queryErr := v.SourceFunction(ctx, request)
				after, err := hostfacts.ObserveGoAllocationTotal(ctx)
				if err != nil {
					return err
				}
				allocated := after.Uint64() - before.Uint64()
				t.Logf("source declaration width=%d lookup bytes=%d; native compiler construction excluded", width, allocated)
				if queryErr != nil || got.Function == nil || got.Function.Syntax() != decl || allocated > 512 {
					t.Fatalf("source declaration width%d loan=%+v,%v bytes%d, want exact last declaration and <=512 fixed bytes", width, got, queryErr, allocated)
				}
				return nil
			}})
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func BenchmarkSSASourceFunction(b *testing.B) {
	b.ReportAllocs()
	const width = 257
	var declarations strings.Builder
	for index := 0; index < width; index++ {
		fmt.Fprintf(&declarations, "func Entry%d(){};", index)
	}
	a := nativeSymbolSourceAnalysis(b, b.TempDir(), "package sample;"+declarations.String())
	err := a.WithSSAUnit(b.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
		if len(v.Source.Syntax[0].Decls) != width {
			b.Fatal("benchmark lost native declaration width")
		}
		decl := v.Source.Syntax[0].Decls[width-1].(*ast.FuncDecl)
		request := gotoolchain.SSASourceFunctionRequest{Kind: gotoolchain.SSASourceFunctionDeclaration, Declaration: decl}
		for b.Loop() {
			got, queryErr := v.SourceFunction(ctx, request)
			if queryErr != nil || got.Function == nil || got.Function.Syntax() != decl {
				b.Fatalf("source function benchmark=%+v,%v, want exact last declaration", got, queryErr)
			}
		}
		b.ReportMetric(width, "native_declarations/op")
		return nil
	}})
	if err != nil {
		b.Fatal(err)
	}
}
