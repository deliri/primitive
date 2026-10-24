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
	"golang.org/x/tools/go/ssa"
)

// These are distinct compiler conversion families and numeric representations;
// expectations come from the source parameter identity, not conversion results.
func nativeConversionSource(mode uint8) string {
	representations := [...]struct{ from, to string }{
		{"int", "int8"}, {"int", "int16"}, {"int", "int32"}, {"int", "int64"},
		{"int", "uint"}, {"int", "uint8"}, {"int", "uint16"}, {"int", "uint32"}, {"int", "uint64"},
		{"int", "float32"}, {"int", "float64"}, {"int", "string"}, {"int", "Count"},
		{"float64", "int"}, {"float64", "float32"}, {"float64", "uint64"},
		{"complex128", "complex64"}, {"string", "[]byte"}, {"string", "[]rune"},
		{"[]byte", "string"}, {"[]rune", "string"}, {"[]byte", "Bytes"},
	}
	mode %= 88
	row := representations[mode%22]
	body := "return " + row.to + "(a)"
	if (mode/22)%2 != 0 {
		body = "touch();" + body
	}
	if mode/44 != 0 {
		body = "if false{" + body + "};var zero " + row.to + ";return zero"
	}
	return "package sample;type Count int;type Bytes []byte;func touch(){};func Entry(a " + row.from + ")" + row.to + "{" + body + "}"
}

func nativeConversionSemanticMode(t testing.TB, directory string, mode uint8) {
	t.Helper()
	a := nativeSymbolSourceAnalysis(t, directory, nativeConversionSource(mode))
	err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
		fn := v.Package.Func("Entry")
		value := nativeConversionValue(t, fn)
		got, err := v.ConversionOperand(ctx, gotoolchain.SSAConversionOperandRequest{Function: fn, Conversion: value})
		if err != nil || got.Validate() != nil {
			t.Fatalf("conversion mode=%d got=%+v,%v, want validated native loan,nil", mode, got, err)
		}
		if mode%88/44 != 0 {
			if got.Operand != nil {
				t.Fatalf("untaken mode=%d operand got=%v, want nil", mode, got.Operand)
			}
		} else if got.Operand != fn.Params[0] {
			t.Fatalf("conversion mode=%d operand got=%v, want source parameter=%v", mode, got.Operand, fn.Params[0])
		}
		return nil
	}})
	if err != nil {
		t.Fatalf("WithSSAUnit got=%v, want nil", err)
	}
}

func TestSSAConversionOperandFiniteNativeDomain(t *testing.T) {
	t.Parallel()
	for mode := uint8(0); mode < 88; mode++ {
		t.Run(fmt.Sprint(mode), func(t *testing.T) { t.Parallel(); nativeConversionSemanticMode(t, t.TempDir(), mode) })
	}
}

func FuzzSSAConversionOperandConservesSourceIdentity(f *testing.F) {
	f.Add(uint8(0))
	f.Fuzz(func(t *testing.T, mode uint8) { nativeConversionSemanticMode(t, t.TempDir(), mode) })
}

func BenchmarkSSAConversionOperand(b *testing.B) {
	b.ReportAllocs()
	source := strings.Replace(nativeConversionSource(0), "return int8(a)", strings.Repeat("touch();", 8193)+"return int8(a)", 1)
	a := nativeSymbolSourceAnalysis(b, b.TempDir(), source)
	err := a.WithSSAUnit(b.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
		fn := v.Package.Func("Entry")
		value := nativeConversionValue(b, fn)
		calls := 0
		for _, block := range fn.Blocks {
			for _, instruction := range block.Instrs {
				if _, ok := instruction.(*ssa.Call); ok {
					calls++
				}
			}
		}
		if calls != 8193 {
			b.Fatalf("conversion benchmark prefix got=%d, want8193 actual native calls", calls)
		}
		for b.Loop() {
			got, err := v.ConversionOperand(ctx, gotoolchain.SSAConversionOperandRequest{Function: fn, Conversion: value})
			if err != nil || got.Operand != fn.Params[0] {
				b.Fatalf("conversion got=%+v,%v, want source parameter,nil", got, err)
			}
		}
		return nil
	}})
	if err != nil {
		b.Fatalf("WithSSAUnit got=%v, want nil", err)
	}
}

func TestSSAConversionOperandRetainsNoInstructionPopulation(t *testing.T) {
	testserial.Declare(t, core.TestIsolationDeclaration{Hazard: core.TestIsolationHazardRuntimeAllocation, Scope: core.TestIsolationScopePackageProcess})
	for _, width := range []int{32, 8193} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			source := strings.Replace(nativeConversionSource(0), "return int8(a)", strings.Repeat("touch();", width)+"return int8(a)", 1)
			a := nativeSymbolSourceAnalysis(t, t.TempDir(), source)
			err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
				fn := v.Package.Func("Entry")
				value := nativeConversionValue(t, fn)
				calls := 0
				for _, block := range fn.Blocks {
					for _, instruction := range block.Instrs {
						if _, ok := instruction.(*ssa.Call); ok {
							calls++
						}
					}
				}
				if calls != width {
					t.Fatalf("conversion prefix got=%d, want %d", calls, width)
				}
				before, err := hostfacts.ObserveGoAllocationTotal(ctx)
				if err != nil {
					return err
				}
				got, err := v.ConversionOperand(ctx, gotoolchain.SSAConversionOperandRequest{Function: fn, Conversion: value})
				if err != nil || got.Operand != fn.Params[0] {
					t.Fatalf("conversion got=%+v,%v, want native parameter,nil", got, err)
				}
				after, err := hostfacts.ObserveGoAllocationTotal(ctx)
				if err != nil {
					return err
				}
				allocated := after.Uint64() - before.Uint64()
				if allocated > 128 {
					t.Fatalf("conversion prefix=%d allocated=%d, want <=128 bytes without copied population", width, allocated)
				}
				t.Logf("conversion prefix=%d allocated=%d bytes", width, allocated)
				return nil
			}})
			if err != nil {
				t.Fatalf("WithSSAUnit got=%v, want nil", err)
			}
		})
	}
}
