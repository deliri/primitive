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

func nativeArgumentBindingLastCall(t testing.TB, fn *ssa.Function) *ssa.Call {
	t.Helper()
	var selected *ssa.Call
	for _, block := range fn.Blocks {
		for _, instruction := range block.Instrs {
			if call, ok := instruction.(*ssa.Call); ok {
				selected = call
			}
		}
	}
	if selected == nil {
		t.Fatalf("native fixture got no call in %v, want selected invocation", fn)
	}
	return selected
}

// The independent oracle counts explicit selected source operands, rather than
// repeating Primitive's SSA representation and capture traversal.
func nativeArgumentBindingMode(t testing.TB, directory string, mode uint8) {
	t.Helper()
	mode %= 48
	mask := mode % 4
	form := (mode / 4) % 3
	untaken := (mode / 12) % 2
	prefix := mode / 24
	left, right := "b", "b"
	want := 0
	if mask&1 != 0 {
		left = "a"
		want++
	}
	if mask&2 != 0 {
		right = "a"
		want++
	}
	body := "helper(" + left + "," + right + ")"
	if form == 1 {
		body = "func(x,y *int){}(" + left + "," + right + ")"
	}
	if form == 2 {
		body = "func(){_,_=" + left + "," + right + "}()"
		if mask == 3 {
			want = 1
		}
	}
	if untaken != 0 {
		body = "if false{" + body + "}"
		want = 0
	}
	if prefix != 0 {
		body = "helper(b,b);" + body
	}
	a := nativeSymbolSourceAnalysis(t, directory, `package sample;func helper(x,y *int){};func Entry(a,b *int){`+body+`}`)
	err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
		fn := v.Package.Func("Entry")
		call := nativeArgumentBindingLastCall(t, fn)
		count := 0
		for got, queryErr := range v.ArgumentBindings(ctx, gotoolchain.SSAArgumentBindingsRequest{Call: call, Argument: fn.Params[0]}) {
			if queryErr != nil || got.Validate() != nil || got.Function != call.Common().StaticCallee() {
				t.Fatalf("mode=%d binding got=%+v,%v, want exact source callee/local,nil", mode, got, queryErr)
			}
			count++
		}
		if count != want {
			t.Fatalf("mode=%d mask=%d form=%d untaken=%d prefix=%d bindings=%d, want %d explicit source identities", mode, mask, form, untaken, prefix, count, want)
		}
		return nil
	}})
	if err != nil {
		t.Fatalf("WithSSAUnit got=%v, want nil", err)
	}
}

func TestSSAArgumentBindingsFiniteSourceMatrix(t *testing.T) {
	t.Parallel()
	for mode := uint8(0); mode < 48; mode++ {
		t.Run(fmt.Sprint(mode), func(t *testing.T) { t.Parallel(); nativeArgumentBindingMode(t, t.TempDir(), mode) })
	}
}

func FuzzSSAArgumentBindingsConservesSelectedSourceIdentity(f *testing.F) {
	for _, seed := range []uint8{0, 1, 3, 5, 9, 11, 13, 35, 47} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, mode uint8) { nativeArgumentBindingMode(t, t.TempDir(), mode) })
}

func nativeWideArgumentBindingSource(width int) string {
	var parameters, arguments strings.Builder
	for index := 0; index < width; index++ {
		if index != 0 {
			parameters.WriteByte(',')
			arguments.WriteByte(',')
		}
		fmt.Fprintf(&parameters, "p%d *int", index)
		arguments.WriteByte('a')
	}
	return "package sample;func helper(" + parameters.String() + "){};func Entry(a *int){helper(" + arguments.String() + ")}"
}

func TestSSAArgumentBindingsRetainsNoBindingPopulation(t *testing.T) {
	testserial.Declare(t, core.TestIsolationDeclaration{Hazard: core.TestIsolationHazardRuntimeAllocation, Scope: core.TestIsolationScopePackageProcess})
	for _, width := range []int{32, 8193} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			a := nativeSymbolSourceAnalysis(t, t.TempDir(), nativeWideArgumentBindingSource(width))
			err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
				fn := v.Package.Func("Entry")
				call := nativeArgumentBindingLastCall(t, fn)
				if len(call.Common().Args) != width {
					t.Fatalf("fixture width got=%d, want %d", len(call.Common().Args), width)
				}
				before, err := hostfacts.ObserveGoAllocationTotal(ctx)
				if err != nil {
					return err
				}
				count := 0
				for got, queryErr := range v.ArgumentBindings(ctx, gotoolchain.SSAArgumentBindingsRequest{Call: call, Argument: fn.Params[0]}) {
					if queryErr != nil || got.Local != got.Function.Params[count] {
						t.Fatalf("wide binding got=%+v,%v, want native ordinal=%d,nil", got, queryErr, count)
					}
					count++
				}
				after, err := hostfacts.ObserveGoAllocationTotal(ctx)
				if err != nil {
					return err
				}
				allocated := after.Uint64() - before.Uint64()
				if count != width || allocated != 0 {
					t.Fatalf("binding width=%d got count=%d bytes=%d, want %d,0", width, count, allocated, width)
				}
				t.Logf("native binding width=%d query bytes=%d; compiler construction excluded", width, allocated)
				return nil
			}})
			if err != nil {
				t.Fatalf("WithSSAUnit got=%v, want nil", err)
			}
		})
	}
}

func BenchmarkSSAArgumentBindings(b *testing.B) {
	b.ReportAllocs()
	const width = 257
	a := nativeSymbolSourceAnalysis(b, b.TempDir(), nativeWideArgumentBindingSource(width))
	err := a.WithSSAUnit(b.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
		fn := v.Package.Func("Entry")
		call := nativeArgumentBindingLastCall(b, fn)
		if len(call.Common().Args) != width {
			b.Fatalf("benchmark width got=%d, want %d", len(call.Common().Args), width)
		}
		for b.Loop() {
			count := 0
			for got, queryErr := range v.ArgumentBindings(ctx, gotoolchain.SSAArgumentBindingsRequest{Call: call, Argument: fn.Params[0]}) {
				if queryErr != nil || got.Local != got.Function.Params[count] {
					b.Fatalf("benchmark binding got=%+v,%v, want native ordinal=%d,nil", got, queryErr, count)
				}
				count++
			}
			if count != width {
				b.Fatalf("benchmark count got=%d, want %d", count, width)
			}
		}
		return nil
	}})
	if err != nil {
		b.Fatalf("WithSSAUnit got=%v, want nil", err)
	}
}
