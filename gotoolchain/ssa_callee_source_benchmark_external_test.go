package gotoolchain_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gotoolchain"
	"github.com/deliri/primitive/v2026/testserial"
)

func TestSSACalleeSourceRetainsNoArgumentPopulation(t *testing.T) {
	testserial.Declare(t, core.TestIsolationDeclaration{Hazard: core.TestIsolationHazardRuntimeAllocation, Scope: core.TestIsolationScopePackageProcess})
	for _, width := range []int{32, 8193} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			a := nativeSymbolSourceAnalysis(t, t.TempDir(), nativeWideFunctionArgumentSource(width))
			err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
				call := nativeFunctionArgumentCall(t, v.Package.Func("Entry"))
				if len(call.Common().Args) != width+1 {
					t.Fatalf("argument population got=%d, want %d", len(call.Common().Args), width+1)
				}
				request := gotoolchain.SSACalleeSourceRequest{Call: call}
				got, err := v.CalleeSource(ctx, request)
				if err != nil || got.Function != v.Package.Func("consume") || got.Value != call.Common().Value {
					t.Fatalf("wide argument precondition got=%+v,%v, want exact callee source loan,nil", got, err)
				}
				allocations := testing.AllocsPerRun(5, func() { got, err = v.CalleeSource(ctx, request) })
				if err != nil || got.Function != v.Package.Func("consume") || allocations != 0 {
					t.Fatalf("width=%d retained query got=%g allocations,%+v,%v, want zero,source loan,nil", width, allocations, got, err)
				}
				return nil
			}})
			if err != nil {
				t.Fatalf("WithSSAUnit got=%v, want nil", err)
			}
		})
	}
}

func BenchmarkSSACalleeSource(b *testing.B) {
	const width = 257
	a := nativeSymbolSourceAnalysis(b, b.TempDir(), nativeWideFunctionArgumentSource(width))
	err := a.WithSSAUnit(b.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
		call := nativeFunctionArgumentCall(b, v.Package.Func("Entry"))
		if len(call.Common().Args) != width+1 {
			b.Fatalf("argument benchmark population got=%d, want %d", len(call.Common().Args), width+1)
		}
		request := gotoolchain.SSACalleeSourceRequest{Call: call}
		got, err := v.CalleeSource(ctx, request)
		if err != nil || got.Function != v.Package.Func("consume") || got.Value != call.Common().Value {
			b.Fatalf("argument benchmark precondition got=%+v,%v, want exact callee source loan,nil", got, err)
		}
		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			got, err = v.CalleeSource(ctx, request)
		}
		b.StopTimer()
		if err != nil || got.Function != v.Package.Func("consume") || got.Value != call.Common().Value {
			b.Fatalf("argument benchmark got=%+v,%v, want exact callee source loan,nil", got, err)
		}
		return nil
	}})
	if err != nil {
		b.Fatalf("WithSSAUnit got=%v, want nil", err)
	}
}
