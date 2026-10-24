package gotoolchain_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gotoolchain"
	"github.com/deliri/primitive/v2026/hostfacts"
	"github.com/deliri/primitive/v2026/testserial"
)

func nativeClosureBindingMode(t testing.TB, mode uint8) {
	t.Helper()
	mode %= 24
	mask := mode % 4
	form := (mode / 4) % 3
	prefix := mode / 12
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
	body := `left,right:=` + left + `,` + right + `;f:=func(){_,_=left,right};_=f`
	if form == 1 {
		body = `left,right:=` + left + `,` + right + `;func(){_,_=left,right}()`
	}
	if form == 2 {
		body = `left,right:=` + left + `,` + right + `;sink(func(callback *int){_,_,_=left,right,callback})`
	}
	if prefix != 0 {
		body = `touch(b);touch(b);` + body
	}
	a := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;func touch(*int){};func sink(func(*int)){};func Entry(a,b *int){`+body+`}`)
	err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
		fn := v.Package.Func("Entry")
		closure := nativeSelectedClosure(t, fn)
		count := 0
		for got, queryErr := range v.ClosureBindings(ctx, gotoolchain.SSAClosureBindingsRequest{Closure: closure, Argument: fn.Params[0]}) {
			if queryErr != nil || got.Validate() != nil || got.Function != closure.Fn || got.Local.Parent() != got.Function {
				t.Fatalf("mode=%d lexical loan got=%+v,%v, want exact native free variable,nil", mode, got, queryErr)
			}
			count++
		}
		if count != want {
			t.Fatalf("mode=%d mask=%d form=%d prefix=%d lexical matches=%d, want %d source assignments", mode, mask, form, prefix, count, want)
		}
		return nil
	}})
	if err != nil {
		t.Fatalf("WithSSAUnit got=%v, want nil", err)
	}
}

func TestSSAClosureBindingsFiniteSourceMatrix(t *testing.T) {
	t.Parallel()
	for mode := uint8(0); mode < 24; mode++ {
		t.Run(fmt.Sprint(mode), func(t *testing.T) { t.Parallel(); nativeClosureBindingMode(t, mode) })
	}
}

func FuzzSSAClosureBindingsConservesLexicalIdentity(f *testing.F) {
	for _, mode := range []uint8{0, 1, 2, 3, 7, 11, 15, 23} {
		f.Add(mode)
	}
	f.Fuzz(func(t *testing.T, mode uint8) { nativeClosureBindingMode(t, mode) })
}

func TestSSAClosureBindingsPreservesConsumerStopAndCause(t *testing.T) {
	t.Parallel()
	a := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;func Entry(a *int){left,right:=a,a;f:=func(){_,_=left,right};_=f}`)
	err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
		fn := v.Package.Func("Entry")
		closure := nativeSelectedClosure(t, fn)
		r := gotoolchain.SSAClosureBindingsRequest{Closure: closure, Argument: fn.Params[0]}
		stopped := 0
		v.ClosureBindings(ctx, r)(func(got gotoolchain.SSAArgumentBindingObservation, err error) bool {
			if stopped != 0 || err != nil || got.Local != got.Function.FreeVars[0] {
				t.Fatalf("lexical stop got=%+v,%v count=%d, want first native capture,nil", got, err, stopped)
			}
			stopped++
			return false
		})
		if stopped != 1 {
			t.Fatalf("lexical stop count=%d, want1", stopped)
		}
		query, cancel := context.WithCancelCause(ctx)
		defer cancel(nil)
		count, refusals := 0, 0
		for got, queryErr := range v.ClosureBindings(query, r) {
			if count == 0 {
				if queryErr != nil || got.Local != got.Function.FreeVars[0] {
					t.Fatalf("lexical cancel prelude got=%+v,%v, want first capture,nil", got, queryErr)
				}
				count++
				cancel(io.ErrClosedPipe)
				continue
			}
			if got != (gotoolchain.SSAArgumentBindingObservation{}) || !errors.Is(queryErr, context.Canceled) || !errors.Is(queryErr, io.ErrClosedPipe) {
				t.Fatalf("lexical cancellation got=%+v,%v, want zero and both native causes", got, queryErr)
			}
			refusals++
		}
		if count != 1 || refusals != 1 {
			t.Fatalf("lexical cancel accounting got=%d,%d, want1,1", count, refusals)
		}
		return nil
	}})
	if err != nil {
		t.Fatalf("WithSSAUnit got=%v, want nil", err)
	}
}

func nativeWideClosureBindingSource(width int) string {
	var declarations, reads strings.Builder
	for index := 0; index < width; index++ {
		fmt.Fprintf(&declarations, "x%d:=a;", index)
		fmt.Fprintf(&reads, "_=x%d;", index)
	}
	return "package sample;func Entry(a *int){" + declarations.String() + "f:=func(){" + reads.String() + "};_=f}"
}

func TestSSAClosureBindingsRetainsNoCapturePopulation(t *testing.T) {
	testserial.Declare(t, core.TestIsolationDeclaration{Hazard: core.TestIsolationHazardRuntimeAllocation, Scope: core.TestIsolationScopePackageProcess})
	for _, width := range []int{32, 8193} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			a := nativeSymbolSourceAnalysis(t, t.TempDir(), nativeWideClosureBindingSource(width))
			err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
				fn := v.Package.Func("Entry")
				closure := nativeSelectedClosure(t, fn)
				target := closure.Fn
				if len(closure.Bindings) != width {
					t.Fatalf("lexical fixture width=%d, want%d", len(closure.Bindings), width)
				}
				before, err := hostfacts.ObserveGoAllocationTotal(ctx)
				if err != nil {
					return err
				}
				count := 0
				for got, queryErr := range v.ClosureBindings(ctx, gotoolchain.SSAClosureBindingsRequest{Closure: closure, Argument: fn.Params[0]}) {
					if queryErr != nil || got.Function != target || got.Local != got.Function.FreeVars[count] {
						t.Fatalf("wide lexical binding got=%+v,%v, want ordinal=%d,nil", got, queryErr, count)
					}
					count++
				}
				after, err := hostfacts.ObserveGoAllocationTotal(ctx)
				if err != nil {
					return err
				}
				allocated := after.Uint64() - before.Uint64()
				t.Logf("native capture width=%d query bytes=%d; compiler construction excluded", width, allocated)
				// Native instruction-interface assertion caches are process setup,
				// bounded by native operand kinds rather than capture population.
				if count != width || allocated > 128 {
					t.Fatalf("capture width=%d got matches=%d bytes=%d, want%d and <=128 fixed native cache bytes", width, count, allocated, width)
				}
				return nil
			}})
			if err != nil {
				t.Fatalf("WithSSAUnit got=%v, want nil", err)
			}
		})
	}
}

func BenchmarkSSAClosureBindings(b *testing.B) {
	b.ReportAllocs()
	const width = 257
	a := nativeSymbolSourceAnalysis(b, b.TempDir(), nativeWideClosureBindingSource(width))
	err := a.WithSSAUnit(b.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
		fn := v.Package.Func("Entry")
		closure := nativeSelectedClosure(b, fn)
		if len(closure.Bindings) != width {
			b.Fatalf("lexical benchmark width=%d, want%d", len(closure.Bindings), width)
		}
		for b.Loop() {
			count := 0
			for got, queryErr := range v.ClosureBindings(ctx, gotoolchain.SSAClosureBindingsRequest{Closure: closure, Argument: fn.Params[0]}) {
				if queryErr != nil || got.Local != got.Function.FreeVars[count] {
					b.Fatalf("lexical benchmark got=%+v,%v, want ordinal=%d,nil", got, queryErr, count)
				}
				count++
			}
			if count != width {
				b.Fatalf("lexical benchmark matches=%d, want%d", count, width)
			}
		}
		return nil
	}})
	if err != nil {
		b.Fatalf("WithSSAUnit got=%v, want nil", err)
	}
}
