package gotoolchain_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gotoolchain"
	"github.com/deliri/primitive/v2026/hostfacts"
	"github.com/deliri/primitive/v2026/testserial"
)

func nativeInterfaceSource(mode uint8, prefix int) string {
	forms := [...]struct{ declarations, from, to string }{
		{`type Problem struct{};func(*Problem)Error()string{return "detail"};`, "*Problem", "error"},
		{`type Rich interface{Error()string;Extra()};`, "Rich", "error"},
		{"type Count int;", "Count", "any"},
		{"type Rich interface{Extra()};", "Rich", "any"},
	}
	row := forms[mode%4]
	body := strings.Repeat("touch();", prefix) + "return value"
	if mode%8/4 != 0 {
		body = "if false{" + body + "};return nil"
	}
	return "package sample;" + row.declarations + "func touch(){};func Entry(value " + row.from + ")" + row.to + "{" + body + "}"
}

func nativeInterfaceMode(t testing.TB, directory string, mode uint8) {
	t.Helper()
	mode %= 16
	a := nativeSymbolSourceAnalysis(t, directory, nativeInterfaceSource(mode, int(mode/8)))
	err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
		fn := view.Package.Func("Entry")
		value := nativeInterfaceConversion(t, fn)
		got, err := view.ConversionOperand(ctx, gotoolchain.SSAConversionOperandRequest{Function: fn, Conversion: value})
		if err != nil || got.Validate() != nil {
			t.Fatalf("mode=%d got=%+v,%v, want validated loan,nil", mode, got, err)
		}
		if mode%8/4 != 0 {
			if got.Operand != nil {
				t.Fatalf("untaken mode=%d got=%v, want no loan", mode, got.Operand)
			}
		} else if got.Operand != fn.Params[0] {
			t.Fatalf("mode=%d got=%v, want exact source parameter=%v", mode, got.Operand, fn.Params[0])
		}
		return nil
	}})
	if err != nil {
		t.Fatal(err)
	}
}

func TestSSAInterfaceOperandFiniteNativeDomain(t *testing.T) {
	t.Parallel()
	for mode := uint8(0); mode < 16; mode++ {
		t.Run(fmt.Sprint(mode), func(t *testing.T) { t.Parallel(); nativeInterfaceMode(t, t.TempDir(), mode) })
	}
}

func FuzzSSAInterfaceOperandConservesNativeInput(f *testing.F) {
	f.Add(uint8(0))
	f.Fuzz(func(t *testing.T, mode uint8) { nativeInterfaceMode(t, t.TempDir(), mode) })
}

func TestSSAInterfaceOperandRetainsNoInstructionPopulation(t *testing.T) {
	testserial.Declare(t, core.TestIsolationDeclaration{Hazard: core.TestIsolationHazardRuntimeAllocation, Scope: core.TestIsolationScopePackageProcess})
	for _, width := range []int{32, 8193} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			a := nativeSymbolSourceAnalysis(t, t.TempDir(), nativeInterfaceSource(0, width))
			err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
				fn := view.Package.Func("Entry")
				value := nativeInterfaceConversion(t, fn)
				before, err := hostfacts.ObserveGoAllocationTotal(ctx)
				if err != nil {
					return err
				}
				got, err := view.ConversionOperand(ctx, gotoolchain.SSAConversionOperandRequest{Function: fn, Conversion: value})
				if err != nil || got.Operand != fn.Params[0] {
					t.Fatalf("loan got=%+v,%v, want parameter,nil", got, err)
				}
				after, err := hostfacts.ObserveGoAllocationTotal(ctx)
				if err != nil {
					return err
				}
				allocated := after.Uint64() - before.Uint64()
				if allocated > 128 {
					t.Fatalf("prefix=%d allocated=%d, want <=128 bytes", width, allocated)
				}
				t.Logf("native prefix=%d allocated=%d bytes", width, allocated)
				return nil
			}})
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func BenchmarkSSAInterfaceOperand(b *testing.B) {
	b.ReportAllocs()
	a := nativeSymbolSourceAnalysis(b, b.TempDir(), nativeInterfaceSource(0, 8193))
	err := a.WithSSAUnit(b.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
		fn := view.Package.Func("Entry")
		value := nativeInterfaceConversion(b, fn)
		for b.Loop() {
			got, err := view.ConversionOperand(ctx, gotoolchain.SSAConversionOperandRequest{Function: fn, Conversion: value})
			if err != nil || got.Operand != fn.Params[0] {
				b.Fatalf("loan got=%+v,%v, want parameter,nil", got, err)
			}
		}
		return nil
	}})
	if err != nil {
		b.Fatal(err)
	}
}
