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

func nativeBinaryOperandsMode(t testing.TB, directory string, mode uint8) {
	t.Helper()
	operators := [...]string{"+", "-", "*", "/", "%", "&", "|", "^", "<<", ">>", "&^", "==", "!=", "<", "<=", ">", ">="}
	mode %= 68
	index := mode % 17
	swapped := (mode/17)%2 != 0
	prefix := mode/34 != 0
	result := "uint"
	if index >= 11 {
		result = "bool"
	}
	left, right := "a", "b"
	if swapped {
		left, right = right, left
	}
	body := "return " + left + operators[index] + right
	if prefix {
		body = "touch(a);" + body
	}
	a := nativeSymbolSourceAnalysis(t, directory, `package sample;func touch(uint){};func Entry(a,b uint)`+result+`{`+body+`}`)
	err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
		fn := v.Package.Func("Entry")
		var op *ssa.BinOp
		for _, block := range fn.Blocks {
			for _, instruction := range block.Instrs {
				if binary, ok := instruction.(*ssa.BinOp); ok {
					if op != nil {
						t.Fatalf("binary mode=%d got operations=%v,%v, want one", mode, op, binary)
					}
					op = binary
				}
			}
		}
		if op == nil {
			t.Fatalf("binary mode=%d got no operation, want %s", mode, operators[index])
		}
		got, queryErr := v.BinaryOperands(ctx, gotoolchain.SSABinaryOperandsRequest{Function: fn, Operation: op})
		wantLeft, wantRight := fn.Params[0], fn.Params[1]
		if swapped {
			wantLeft, wantRight = wantRight, wantLeft
		}
		if queryErr != nil || got.Left != wantLeft || got.Right != wantRight || got.Validate() != nil {
			t.Fatalf("mode=%d operator=%s binary got=%+v,%v, want native inputs=%v,%v,nil", mode, operators[index], got, queryErr, wantLeft, wantRight)
		}
		return nil
	}})
	if err != nil {
		t.Fatalf("WithSSAUnit got=%v, want nil", err)
	}
}

func TestSSABinaryOperandsFiniteSourceMatrix(t *testing.T) {
	t.Parallel()
	for mode := uint8(0); mode < 68; mode++ {
		t.Run(fmt.Sprint(mode), func(t *testing.T) { t.Parallel(); nativeBinaryOperandsMode(t, t.TempDir(), mode) })
	}
}

func FuzzSSABinaryOperandsConservesNativeInputOrder(f *testing.F) {
	for _, mode := range []uint8{0, 8, 11, 16, 17, 33, 34, 67} {
		f.Add(mode)
	}
	f.Fuzz(func(t *testing.T, mode uint8) { nativeBinaryOperandsMode(t, t.TempDir(), mode) })
}

func nativeWideBinaryOperandsSource(width int) string {
	return `package sample;func touch(){};func Entry(a,b int)int{` + strings.Repeat("touch();", width) + `return a+b}`
}

func TestSSABinaryOperandsRetainsNoInstructionPopulation(t *testing.T) {
	testserial.Declare(t, core.TestIsolationDeclaration{Hazard: core.TestIsolationHazardRuntimeAllocation, Scope: core.TestIsolationScopePackageProcess})
	for _, width := range []int{32, 8193} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			a := nativeSymbolSourceAnalysis(t, t.TempDir(), nativeWideBinaryOperandsSource(width))
			err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
				fn := v.Package.Func("Entry")
				op := nativeBinaryOperandsAdd(t, fn)
				calls := 0
				for _, block := range fn.Blocks {
					for _, instruction := range block.Instrs {
						if _, ok := instruction.(*ssa.Call); ok {
							calls++
						}
					}
				}
				if calls != width {
					t.Fatalf("binary prefix got=%d, want%d", calls, width)
				}
				before, err := hostfacts.ObserveGoAllocationTotal(ctx)
				if err != nil {
					return err
				}
				got, queryErr := v.BinaryOperands(ctx, gotoolchain.SSABinaryOperandsRequest{Function: fn, Operation: op})
				after, err := hostfacts.ObserveGoAllocationTotal(ctx)
				if err != nil {
					return err
				}
				allocated := after.Uint64() - before.Uint64()
				t.Logf("binary native prefix=%d query bytes=%d; compiler construction excluded", width, allocated)
				if queryErr != nil || got.Left != fn.Params[0] || got.Right != fn.Params[1] || allocated > 128 {
					t.Fatalf("binary prefix=%d got=%+v,%v bytes=%d, want exact native inputs,nil and <=128 fixed bytes", width, got, queryErr, allocated)
				}
				return nil
			}})
			if err != nil {
				t.Fatalf("WithSSAUnit got=%v, want nil", err)
			}
		})
	}
}

func BenchmarkSSABinaryOperands(b *testing.B) {
	b.ReportAllocs()
	const width = 8193
	a := nativeSymbolSourceAnalysis(b, b.TempDir(), nativeWideBinaryOperandsSource(width))
	err := a.WithSSAUnit(b.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
		fn := v.Package.Func("Entry")
		op := nativeBinaryOperandsAdd(b, fn)
		calls := 0
		for _, block := range fn.Blocks {
			for _, instruction := range block.Instrs {
				if _, ok := instruction.(*ssa.Call); ok {
					calls++
				}
			}
		}
		if calls != width {
			b.Fatalf("binary benchmark prefix=%d, want%d", calls, width)
		}
		for b.Loop() {
			got, queryErr := v.BinaryOperands(ctx, gotoolchain.SSABinaryOperandsRequest{Function: fn, Operation: op})
			if queryErr != nil || got.Left != fn.Params[0] || got.Right != fn.Params[1] {
				b.Fatalf("binary benchmark got=%+v,%v, want exact native inputs,nil", got, queryErr)
			}
		}
		return nil
	}})
	if err != nil {
		b.Fatalf("WithSSAUnit got=%v, want nil", err)
	}
}
