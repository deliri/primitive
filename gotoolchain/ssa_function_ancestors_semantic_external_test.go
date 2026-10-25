package gotoolchain_test

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gotoolchain"
	"github.com/deliri/primitive/v2026/hostfacts"
	"github.com/deliri/primitive/v2026/testserial"
	"golang.org/x/tools/go/ssa"
)

func FuzzSSAFunctionAncestorsConserveIdentityAndCause(f *testing.F) {
	f.Add(uint8(2), uint8(0))
	f.Fuzz(func(t *testing.T, position, damage uint8) {
		a := nativeSymbolSourceAnalysis(t, t.TempDir(), functionAncestorsSource)
		err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
			root := v.Package.Func("Entry")
			if len(root.AnonFuncs) != 1 || len(root.AnonFuncs[0].AnonFuncs) != 1 {
				t.Fatal("fuzz fixture lost its distinct native frames")
			}
			chain := [3]*ssa.Function{root, root.AnonFuncs[0], root.AnonFuncs[0].AnonFuncs[0]}
			depth := int(position % 3)
			selected := chain[depth]
			ctx, cancel := context.WithCancelCause(ctx)
			defer cancel(nil)
			mode := damage % 4
			switch mode {
			case 1:
				clone := *selected
				selected = &clone
				if selected == chain[depth] || selected.Syntax() != chain[depth].Syntax() {
					t.Fatal("copy mutation did not isolate native frame identity")
				}
			case 2:
				if selected.Signature == nil {
					t.Fatal("missing signature precondition")
				}
				selected.Signature = nil
			case 3:
				cancel(io.ErrClosedPipe)
			}
			loans, refusals := 0, 0
			for fn, queryErr := range v.FunctionAncestors(ctx, gotoolchain.SSAFunctionAncestorsRequest{Function: selected}) {
				if mode != 0 {
					want := error(core.ErrGoToolchainOutput)
					if mode == 3 {
						want = io.ErrClosedPipe
					}
					if fn != nil || !errors.Is(queryErr, want) || mode == 3 && !errors.Is(queryErr, context.Canceled) {
						t.Fatalf("fuzz damaged native loan=%v,%v, want nil,%v", fn, queryErr, want)
					}
					refusals++
					continue
				}
				if queryErr != nil || loans > depth || fn != chain[depth-loans] {
					t.Fatalf("fuzz ancestor%d=%v,%v, want exact native source chain", loans, fn, queryErr)
				}
				loans++
			}
			if mode == 0 {
				if loans != depth+1 || refusals != 0 {
					t.Fatalf("fuzz native loans/refusals=%d/%d, want%d/0", loans, refusals, depth+1)
				}
			} else if loans != 0 || refusals != 1 {
				t.Fatalf("fuzz refused loans/refusals=%d/%d, want0/1", loans, refusals)
			}
			return nil
		}})
		if err != nil {
			t.Fatal(err)
		}
	})
}

func TestSSAFunctionAncestorsRetainNoNativePopulation(t *testing.T) {
	testserial.Declare(t, core.TestIsolationDeclaration{Hazard: core.TestIsolationHazardRuntimeAllocation, Scope: core.TestIsolationScopePackageProcess})
	for _, row := range []struct {
		name         string
		width, depth int
	}{
		{"width32", 32, 1}, {"width8193", 8193, 1}, {"depth8", 1, 8}, {"depth257", 1, 257},
	} {
		t.Run(row.name, func(t *testing.T) {
			source := "package sample;func Entry(){" + strings.Repeat("_=func(){};", row.width-1) + strings.Repeat("_=func(){", row.depth) + strings.Repeat("};", row.depth) + "}"
			a := nativeSymbolSourceAnalysis(t, t.TempDir(), source)
			err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
				root := v.Package.Func("Entry")
				if len(root.AnonFuncs) != row.width {
					t.Fatalf("native width=%d want%d", len(root.AnonFuncs), row.width)
				}
				selected := root.AnonFuncs[row.width-1]
				for level := 1; level < row.depth; level++ {
					if len(selected.AnonFuncs) != 1 {
						t.Fatal("native depth lost intermediate frame")
					}
					selected = selected.AnonFuncs[0]
				}
				before, err := hostfacts.ObserveGoAllocationTotal(ctx)
				if err != nil {
					return err
				}
				loans := 0
				for fn, queryErr := range v.FunctionAncestors(ctx, gotoolchain.SSAFunctionAncestorsRequest{Function: selected}) {
					if queryErr != nil || fn != selected {
						t.Fatalf("first ancestor=%v,%v", fn, queryErr)
					}
					loans++
					break
				}
				after, err := hostfacts.ObserveGoAllocationTotal(ctx)
				if err != nil {
					return err
				}
				allocated := after.Uint64() - before.Uint64()
				t.Logf("native ancestors width%d depth%d first-loan bytes%d; compiler construction excluded", row.width, row.depth, allocated)
				if loans != 1 || allocated > 512 {
					t.Fatalf("first-loan population retained: loans%d bytes%d", loans, allocated)
				}
				return nil
			}})
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func BenchmarkSSAFunctionAncestors(b *testing.B) {
	b.ReportAllocs()
	const depth = 8
	source := "package sample;func Entry(){" + strings.Repeat("_=func(){", depth) + strings.Repeat("};", depth) + "}"
	a := nativeSymbolSourceAnalysis(b, b.TempDir(), source)
	err := a.WithSSAUnit(b.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
		root := v.Package.Func("Entry")
		selected := root
		for level := 0; level < depth; level++ {
			if len(selected.AnonFuncs) != 1 {
				b.Fatal("benchmark lost fixed native depth")
			}
			selected = selected.AnonFuncs[0]
		}
		for b.Loop() {
			loans := 0
			var last *ssa.Function
			for fn, queryErr := range v.FunctionAncestors(ctx, gotoolchain.SSAFunctionAncestorsRequest{Function: selected}) {
				if queryErr != nil || fn == nil || loans == 0 && fn != selected {
					b.Fatalf("ancestor benchmark=%v,%v", fn, queryErr)
				}
				last = fn
				loans++
			}
			if loans != depth+1 || last != root {
				b.Fatalf("ancestor workload loans%d root%v, want%d/%v", loans, last, depth+1, root)
			}
		}
		b.ReportMetric(depth+1, "native_frames/op")
		return nil
	}})
	if err != nil {
		b.Fatal(err)
	}
}
