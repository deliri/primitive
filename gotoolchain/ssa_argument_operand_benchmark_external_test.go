package gotoolchain_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gotoolchain"
	"github.com/deliri/primitive/v2026/testserial"
	"golang.org/x/tools/go/ssa"
)

func TestSSAArgumentOperandRetainsNoInstructionPopulation(t *testing.T) {
	testserial.Declare(t, core.TestIsolationDeclaration{Hazard: core.TestIsolationHazardRuntimeAllocation, Scope: core.TestIsolationScopePackageProcess})
	for _, width := range []int{32, 8193} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			a := nativeSymbolSourceAnalysis(t, t.TempDir(), nativeArgumentOperandPrelude+`func Entry(r *Runner){`+strings.Repeat("sink(r);", width)+`}`)
			err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
				fn := v.Package.Func("Entry")
				calls := 0
				for _, b := range fn.Blocks {
					for _, instruction := range b.Instrs {
						if _, ok := instruction.(*ssa.Call); ok {
							calls++
						}
					}
				}
				if calls != width {
					t.Fatalf("population got=%d, want %d native calls", calls, width)
				}
				request := gotoolchain.SSAArgumentOperandRequest{Function: fn, Value: nativeArgumentOperandSink(t, fn).Common().Args[0], Argument: fn.Params[0]}
				got, err := v.ArgumentOperand(ctx, request)
				if err != nil || !got.Proven {
					t.Fatalf("query precondition got=%+v,%v, want proven,nil", got, err)
				}
				bytes := testing.AllocsPerRun(10, func() { got, err = v.ArgumentOperand(ctx, request) })
				if err != nil || !got.Proven || bytes != 0 {
					t.Fatalf("width=%d allocation got=%g allocs,%+v,%v, want zero,proven,nil", width, bytes, got, err)
				}
				return nil
			}})
			if err != nil {
				t.Fatalf("WithSSAUnit got=%v, want nil", err)
			}
		})
	}
}

func BenchmarkSSAArgumentOperand(b *testing.B) {
	const width = 257
	a := nativeSymbolSourceAnalysis(b, b.TempDir(), nativeArgumentOperandPrelude+`func Entry(r *Runner){`+strings.Repeat("sink(r);", width)+`}`)
	err := a.WithSSAUnit(b.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
		fn := v.Package.Func("Entry")
		calls := 0
		for _, block := range fn.Blocks {
			for _, instruction := range block.Instrs {
				if _, ok := instruction.(*ssa.Call); ok {
					calls++
				}
			}
		}
		if calls != width {
			b.Fatalf("benchmark population got=%d, want %d", calls, width)
		}
		request := gotoolchain.SSAArgumentOperandRequest{Function: fn, Value: nativeArgumentOperandSink(b, fn).Common().Args[0], Argument: fn.Params[0]}
		got, err := v.ArgumentOperand(ctx, request)
		if err != nil || !got.Proven {
			b.Fatalf("benchmark precondition got=%+v,%v, want proven,nil", got, err)
		}
		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			got, err = v.ArgumentOperand(ctx, request)
		}
		b.StopTimer()
		if err != nil || !got.Proven {
			b.Fatalf("benchmark query got=%+v,%v, want proven,nil", got, err)
		}
		return nil
	}})
	if err != nil {
		b.Fatalf("WithSSAUnit got=%v, want nil", err)
	}
}
