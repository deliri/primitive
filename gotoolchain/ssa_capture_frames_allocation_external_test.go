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

func TestSSACaptureFramesFirstLoanRetainsNoInstructionPopulation(t *testing.T) {
	testserial.Declare(t, core.TestIsolationDeclaration{Hazard: core.TestIsolationHazardRuntimeAllocation, Scope: core.TestIsolationScopePackageProcess})
	for _, width := range []int{32, 8193} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			a := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;func touch(*int){};func Entry(a *int){`+strings.Repeat(`touch(a);`, width)+`f:=func(){_=a};_=f}`)
			err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
				fn := v.Package.Func("Entry")
				if len(fn.Blocks) != 1 || len(fn.Blocks[0].Instrs) < width+2 || len(fn.AnonFuncs) != 1 {
					t.Fatal("width fixture lost its actual native prefix or closure")
				}
				before, err := hostfacts.ObserveGoAllocationTotal(ctx)
				if err != nil {
					return err
				}
				count := 0
				for got, queryErr := range v.CaptureFrames(ctx, gotoolchain.SSACaptureFramesRequest{Function: fn, Argument: fn.Params[0]}) {
					if queryErr != nil || got.Function != fn || got.Local != fn.Params[0] {
						t.Fatalf("wide capture first loan=%+v,%v, want exact root", got, queryErr)
					}
					count++
					break
				}
				after, err := hostfacts.ObserveGoAllocationTotal(ctx)
				if err != nil {
					return err
				}
				allocated := after.Uint64() - before.Uint64()
				t.Logf("native capture frame width=%d first-loan bytes=%d; compiler construction and arbitrary-depth ancestry excluded", width, allocated)
				if count != 1 || allocated > 512 {
					t.Fatalf("wide capture width%d observations%d bytes%d, want1 and <=512 fixed bytes", width, count, allocated)
				}
				return nil
			}})
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func BenchmarkSSACaptureFrames(b *testing.B) {
	b.ReportAllocs()
	const width = 257
	a := nativeSymbolSourceAnalysis(b, b.TempDir(), `package sample;func touch(*int){};func Entry(a *int){`+strings.Repeat(`touch(a);`, width)+`f:=func(){_=a;g:=func(){_=a};_=g};_=f}`)
	err := a.WithSSAUnit(b.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
		fn := v.Package.Func("Entry")
		if len(fn.Blocks) != 1 || len(fn.Blocks[0].Instrs) < width+2 || len(fn.AnonFuncs) != 1 || len(fn.AnonFuncs[0].AnonFuncs) != 1 {
			b.Fatal("benchmark lost its native prefix or two capture edges")
		}
		for b.Loop() {
			count := 0
			for got, queryErr := range v.CaptureFrames(ctx, gotoolchain.SSACaptureFramesRequest{Function: fn, Argument: fn.Params[0]}) {
				if queryErr != nil || got.Validate() != nil {
					b.Fatalf("capture benchmark loan=%+v,%v, want native frame", got, queryErr)
				}
				count++
			}
			if count != 3 {
				b.Fatalf("capture benchmark frames=%d, want3", count)
			}
		}
		b.ReportMetric(width, "native_prefix_calls/op")
		b.ReportMetric(3, "frames/op")
		return nil
	}})
	if err != nil {
		b.Fatal(err)
	}
}
