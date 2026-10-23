package gotoolchain_test

import (
	"context"
	"strconv"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gotoolchain"
	"github.com/deliri/primitive/v2026/hostfacts"
	"github.com/deliri/primitive/v2026/testserial"
)

func TestSSAMethodReceiverRetainsNoOperandPopulation(t *testing.T) {
	testserial.Declare(t, core.TestIsolationDeclaration{Hazard: core.TestIsolationHazardRuntimeAllocation, Scope: core.TestIsolationScopePackageProcess})
	for _, population := range []int{32, 8193} {
		t.Run(strconv.Itoa(population), func(t *testing.T) {
			a := nativeSymbolSourceAnalysis(t, t.TempDir(), nativeMethodReceiverDeclarations+`func Entry(r *sample){`+strings.Repeat(`_=plain(r);`, population)+`_=r.M(r)}`)
			err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
				fn := v.Package.Func("Entry")
				request := gotoolchain.SSAMethodReceiverRequest{Call: nativeReceiverParameterLastCall(ctx, t, v, fn)}
				before, err := hostfacts.ObserveGoAllocationTotal(ctx)
				if err != nil {
					return err
				}
				got, queryErr := v.MethodReceiver(ctx, request)
				after, err := hostfacts.ObserveGoAllocationTotal(ctx)
				if err != nil {
					return err
				}
				allocated := after.Uint64() - before.Uint64()
				t.Logf("preceding native calls=%d query allocation=%d; compiler construction excluded", population, allocated)
				if queryErr != nil || got.Method == nil || got.Receiver != fn.Params[0] || allocated > 256 {
					t.Fatalf("method query=%+v,%v allocation=%d, want actual receiver,nil and at most 256 bytes", got, queryErr, allocated)
				}
				return nil
			}})
			if err != nil {
				t.Fatalf("WithSSAUnit error=%v, want nil", err)
			}
		})
	}
}

func BenchmarkSSAMethodReceiver(b *testing.B) {
	b.ReportAllocs()
	const population = 257
	a := nativeSymbolSourceAnalysis(b, b.TempDir(), nativeMethodReceiverDeclarations+`func Entry(r *sample){`+strings.Repeat(`_=plain(r);`, population)+`_=r.M(r)}`)
	err := a.WithSSAUnit(b.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
		fn := v.Package.Func("Entry")
		request := gotoolchain.SSAMethodReceiverRequest{Call: nativeReceiverParameterLastCall(ctx, b, v, fn)}
		for b.Loop() {
			got, queryErr := v.MethodReceiver(ctx, request)
			if queryErr != nil || got.Method == nil || got.Receiver != fn.Params[0] {
				b.Fatalf("method query=%+v,%v, want actual receiver,nil", got, queryErr)
			}
		}
		b.ReportMetric(population+1, "native_calls/op")
		return nil
	}})
	if err != nil {
		b.Fatalf("WithSSAUnit error=%v, want nil", err)
	}
}
