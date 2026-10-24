package gotoolchain_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gotoolchain"
	"github.com/deliri/primitive/v2026/hostfacts"
	"github.com/deliri/primitive/v2026/testserial"
	"golang.org/x/tools/go/ssa"
)

func nativePhiSource(mode uint8) string {
	row := [...]struct{ result, left, right string }{
		{"int", "a", "b"}, {"int", "1", "2"}, {"int", "produce(a)", "produce(b)"},
		{"func()int", "left", "right"}, {"int", "globalLeft", "globalRight"},
		{"int64", "int64(a)", "int64(b)"}, {"func()int", "func()int{return a}", "func()int{return b}"},
	}[mode%7]
	return `package sample;var globalLeft,globalRight int;func left()int{return 1};func right()int{return 2};func produce(x int)int{return x};func Entry(flag bool,a,b int)` + row.result + `{value:=` + row.left + `;if flag{value=` + row.right + `};return value};func Other(a int){}`
}

func nativePhiSemanticMode(t testing.TB, directory string, mode uint8) {
	t.Helper()
	mode %= 28
	a := nativeSymbolSourceAnalysis(t, directory, nativePhiSource(mode))
	err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
		fn := v.Package.Func("Entry")
		merge := nativeReturnedPhi(t, fn)
		if len(merge.Edges) != 2 {
			t.Fatalf("Phi mode=%d fixture inputs got=%d, want2", mode, len(merge.Edges))
		}
		want := [2]ssa.Value{merge.Edges[0], merge.Edges[1]}
		request := gotoolchain.SSAPhiInputsRequest{Function: fn, Merge: merge}
		var wantErr error
		switch mode / 7 {
		case 0:
		case 1:
			merge.Edges[1] = v.Package.Func("Other").Params[0]
			wantErr = core.ErrGoToolchainOutput
		case 2:
			merge.Edges[1] = nil
			wantErr = core.ErrGoToolchainOutput
		case 3:
			copied := *merge
			request.Merge = &copied
			wantErr = core.ErrGoToolchainContract
		default:
			t.Fatalf("Phi mode got=%d, want finite domain", mode)
		}
		seen, refused := 0, 0
		for got, err := range v.PhiInputs(ctx, request) {
			if wantErr != nil {
				if got != (gotoolchain.SSAPhiInputObservation{}) || !errors.Is(err, wantErr) || wantErr == core.ErrGoToolchainContract && errors.Is(err, core.ErrGoToolchainOutput) {
					t.Fatalf("Phi mode=%d refusal got=%+v,%v, want zero,%v", mode, got, err, wantErr)
				}
				refused++
				continue
			}
			if err != nil || got.Validate() != nil || seen >= len(want) || got.Value != want[seen] || got.Block != merge.Block() {
				t.Fatalf("Phi mode=%d index=%d got=%+v,%v, want original native input,nil", mode, seen, got, err)
			}
			seen++
		}
		wantSeen, wantRefused := 2, 0
		if wantErr != nil {
			wantSeen, wantRefused = 0, 1
		}
		if seen != wantSeen || refused != wantRefused {
			t.Fatalf("Phi mode=%d conservation got=%d,%d, want%d,%d", mode, seen, refused, wantSeen, wantRefused)
		}
		return nil
	}})
	if err != nil {
		t.Fatalf("WithSSAUnit got=%v, want nil", err)
	}
}

func TestSSAPhiInputsFiniteNativeSourceDomain(t *testing.T) {
	t.Parallel()
	for mode := uint8(0); mode < 28; mode++ {
		t.Run(fmt.Sprint(mode), func(t *testing.T) { t.Parallel(); nativePhiSemanticMode(t, t.TempDir(), mode) })
	}
}

func FuzzSSAPhiInputsConservesSourceOrRefusal(f *testing.F) {
	f.Add(uint8(0))
	f.Fuzz(func(t *testing.T, mode uint8) { nativePhiSemanticMode(t, t.TempDir(), mode) })
}

func nativeWidePhiSource(width int) string {
	var source strings.Builder
	source.WriteString(`package sample;func Entry(a,selector int)int{value:=a-1;switch selector{`)
	for index := 0; index < width; index++ {
		fmt.Fprintf(&source, "case %d:value=a+%d;", index, index)
	}
	source.WriteString(`};return value}`)
	return source.String()
}

// Full-rate profiling attributes the fixed iterator frames and caller closure
// to544 bytes; both32 and8193 inputs retain that constant footprint. The768-byte
// ceiling includes native interface-cache variation, not an input population.
func TestSSAPhiInputsRetainsNoIncomingPopulation(t *testing.T) {
	testserial.Declare(t, core.TestIsolationDeclaration{Hazard: core.TestIsolationHazardRuntimeAllocation, Scope: core.TestIsolationScopePackageProcess})
	for _, width := range []int{32, 8193} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			a := nativeSymbolSourceAnalysis(t, t.TempDir(), nativeWidePhiSource(width))
			err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
				fn := v.Package.Func("Entry")
				merge := nativeReturnedPhi(t, fn)
				if len(merge.Edges) != width+1 {
					t.Fatalf("Phi width=%d native inputs got=%d, want%d", width, len(merge.Edges), width+1)
				}
				before, err := hostfacts.ObserveGoAllocationTotal(ctx)
				if err != nil {
					return err
				}
				seen := 0
				for got, err := range v.PhiInputs(ctx, gotoolchain.SSAPhiInputsRequest{Function: fn, Merge: merge}) {
					if err != nil || seen >= len(merge.Edges) || got.Value != merge.Edges[seen] || got.Block != merge.Block() {
						t.Fatalf("wide Phi input=%d got=%+v,%v, want native value,nil", seen, got, err)
					}
					seen++
				}
				after, err := hostfacts.ObserveGoAllocationTotal(ctx)
				if err != nil {
					return err
				}
				allocated := after.Uint64() - before.Uint64()
				if seen != width+1 || allocated > 768 {
					t.Fatalf("Phi width=%d got inputs=%d bytes=%d, want%d inputs and <=768 fixed bytes", width, seen, allocated, width+1)
				}
				t.Logf("Phi width=%d inputs=%d allocated=%d bytes", width, seen, allocated)
				return nil
			}})
			if err != nil {
				t.Fatalf("WithSSAUnit got=%v, want nil", err)
			}
		})
	}
}

func BenchmarkSSAPhiInputs(b *testing.B) {
	b.ReportAllocs()
	const width = 257
	a := nativeSymbolSourceAnalysis(b, b.TempDir(), nativeWidePhiSource(width))
	err := a.WithSSAUnit(b.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
		fn := v.Package.Func("Entry")
		merge := nativeReturnedPhi(b, fn)
		if len(merge.Edges) != width+1 {
			b.Fatalf("benchmark Phi inputs got=%d, want%d", len(merge.Edges), width+1)
		}
		for b.Loop() {
			seen := 0
			for got, err := range v.PhiInputs(ctx, gotoolchain.SSAPhiInputsRequest{Function: fn, Merge: merge}) {
				if err != nil || seen >= len(merge.Edges) || got.Value != merge.Edges[seen] {
					b.Fatalf("benchmark Phi input=%d got=%+v,%v, want native input,nil", seen, got, err)
				}
				seen++
			}
			if seen != width+1 {
				b.Fatalf("benchmark Phi count got=%d, want%d", seen, width+1)
			}
		}
		return nil
	}})
	if err != nil {
		b.Fatalf("WithSSAUnit got=%v, want nil", err)
	}
}
