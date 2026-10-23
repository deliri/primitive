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

func TestSSAReceiverParameterRetainsNoInstructionPopulation(t *testing.T) {
	testserial.Declare(t, core.TestIsolationDeclaration{Hazard: core.TestIsolationHazardRuntimeAllocation, Scope: core.TestIsolationScopePackageProcess})
	for _, population := range []int{32, 8193} {
		t.Run(fmt.Sprint(population), func(t *testing.T) {
			a := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;type Cell struct{};func(*Cell)Invoke(int){};func consume(){};func Entry(r *Cell){`+strings.Repeat(`consume();`, population)+`r.Invoke(1)}`)
			err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
				fn := view.Package.Func("Entry")
				request := gotoolchain.SSAReceiverParameterRequest{Call: nativeReceiverParameterLastCall(ctx, t, view, fn), Parameter: fn.Params[0]}
				before, err := hostfacts.ObserveGoAllocationTotal(ctx)
				if err != nil {
					return err
				}
				got, queryErr := view.ReceiverParameter(ctx, request)
				after, err := hostfacts.ObserveGoAllocationTotal(ctx)
				if err != nil {
					return err
				}
				allocated := after.Uint64() - before.Uint64()
				if queryErr != nil || !got.Proven || allocated > 4<<10 {
					t.Fatalf("receiver proof/query storage=%+v/%d,%v, want proven and at most4096 bytes,nil", got, allocated, queryErr)
				}
				t.Logf("receiver query storage=%d at %d native prefix calls; compiler construction excluded", allocated, population)
				return nil
			}})
			if err != nil {
				t.Fatalf("WithSSAUnit error=%v, want nil", err)
			}
		})
	}
}

func BenchmarkSSAReceiverParameter(b *testing.B) {
	b.ReportAllocs()
	const population = 257
	for _, row := range []struct {
		name, declarations, body string
		want                     bool
	}{
		{"direct receiver", `func(*Owner)Invoke(int){}`, `r.Invoke(1)`, true},
		{"hidden embedded method", `func(*Owner)Invoke(int){}`, `r.Cell.Invoke(1)`, false},
		{"promoted receiver", "", `r.Invoke(1)`, true},
	} {
		b.Run(row.name, func(b *testing.B) {
			b.ReportAllocs()
			a := nativeSymbolSourceAnalysis(b, b.TempDir(), `package sample;type Cell struct{};func(*Cell)Invoke(int){};type Owner struct{Cell};`+row.declarations+`
func consume(){};func Entry(r *Owner){`+strings.Repeat(`consume();`, population)+row.body+`}`)
			queryErr := a.WithSSAUnit(b.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
				fn := view.Package.Func("Entry")
				request := gotoolchain.SSAReceiverParameterRequest{Call: nativeReceiverParameterLastCall(ctx, b, view, fn), Parameter: fn.Params[0]}
				calls := 0
				for instruction, err := range view.Instructions(ctx, gotoolchain.SSAInstructionRequest{Function: fn, Extent: gotoolchain.SSAInstructionExtentFunction}) {
					if err != nil {
						return err
					}
					if _, call := instruction.(*ssa.Call); call {
						calls++
					}
				}
				if calls != population+1 {
					b.Fatalf("native calls=%d, want %d", calls, population+1)
				}
				for b.Loop() {
					got, err := view.ReceiverParameter(ctx, request)
					if err != nil || got.Proven != row.want {
						b.Fatalf("receiver proof=%+v,%v, want proven=%t,nil", got, err, row.want)
					}
				}
				b.ReportMetric(float64(calls), "native_calls/op")
				return nil
			}})
			if queryErr != nil {
				b.Fatalf("WithSSAUnit error=%v, want nil", queryErr)
			}
		})
	}
}
