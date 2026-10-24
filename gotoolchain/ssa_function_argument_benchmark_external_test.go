package gotoolchain_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gotoolchain"
	"github.com/deliri/primitive/v2026/testserial"
)

func nativeWideFunctionArgumentSource(width int) string {
	return `package sample;func named(int){};func consume(` + strings.Repeat("_ int,", width) + `callback func(int)){};func Entry(){consume(` + strings.Repeat("0,", width) + `named)}`
}

func TestSSALastFunctionArgumentRetainsNoArgumentPopulation(t *testing.T) {
	testserial.Declare(t, core.TestIsolationDeclaration{Hazard: core.TestIsolationHazardRuntimeAllocation, Scope: core.TestIsolationScopePackageProcess})
	for _, width := range []int{32, 8193} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			a := nativeSymbolSourceAnalysis(t, t.TempDir(), nativeWideFunctionArgumentSource(width))
			err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
				call := nativeFunctionArgumentCall(t, v.Package.Func("Entry"))
				if len(call.Common().Args) != width+1 {
					t.Fatalf("argument population got=%d, want %d", len(call.Common().Args), width+1)
				}
				request := gotoolchain.SSAFunctionArgumentRequest{Call: call}
				got, err := v.LastFunctionArgument(ctx, request)
				if err != nil || got.Function != v.Package.Func("named") || got.Value != call.Common().Args[width] {
					t.Fatalf("wide argument precondition got=%+v,%v, want exact final source loan,nil", got, err)
				}
				allocations := testing.AllocsPerRun(5, func() { got, err = v.LastFunctionArgument(ctx, request) })
				if err != nil || got.Function != v.Package.Func("named") || allocations != 0 {
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

func BenchmarkSSALastFunctionArgument(b *testing.B) {
	const width = 257
	a := nativeSymbolSourceAnalysis(b, b.TempDir(), nativeWideFunctionArgumentSource(width))
	err := a.WithSSAUnit(b.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
		call := nativeFunctionArgumentCall(b, v.Package.Func("Entry"))
		if len(call.Common().Args) != width+1 {
			b.Fatalf("argument benchmark population got=%d, want %d", len(call.Common().Args), width+1)
		}
		request := gotoolchain.SSAFunctionArgumentRequest{Call: call}
		got, err := v.LastFunctionArgument(ctx, request)
		if err != nil || got.Function != v.Package.Func("named") || got.Value != call.Common().Args[width] {
			b.Fatalf("argument benchmark precondition got=%+v,%v, want exact source loan,nil", got, err)
		}
		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			got, err = v.LastFunctionArgument(ctx, request)
		}
		b.StopTimer()
		if err != nil || got.Function != v.Package.Func("named") || got.Value != call.Common().Args[width] {
			b.Fatalf("argument benchmark got=%+v,%v, want exact source loan,nil", got, err)
		}
		return nil
	}})
	if err != nil {
		b.Fatalf("WithSSAUnit got=%v, want nil", err)
	}
}
