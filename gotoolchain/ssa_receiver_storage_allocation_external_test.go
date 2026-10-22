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

func TestSSAReceiverStorageRetainsNoCallPopulation(t *testing.T) {
	testserial.Declare(t, core.TestIsolationDeclaration{Hazard: core.TestIsolationHazardRuntimeAllocation, Scope: core.TestIsolationScopePackageProcess})
	for _, population := range []int{32, 8193} {
		t.Run(fmt.Sprint(population), func(t *testing.T) {
			a := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;type Cell struct{};func(*Cell)First(){};func(*Cell)Second(){};type Box struct{Cell};func Entry(a *Box){first:=a.First;first();`+strings.Repeat(`a.Second();`, population-1)+`}`)
			err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], SourceValues: true, Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
				var first, last *ssa.Call
				count := 0
				for _, block := range v.Package.Func("Entry").Blocks {
					for _, instruction := range block.Instrs {
						if c, ok := instruction.(*ssa.Call); ok {
							if first == nil {
								first = c
							}
							last = c
							count++
						}
					}
				}
				if count != population {
					t.Fatalf("native fixture calls=%d, want %d", count, population)
				}
				before, err := hostfacts.ObserveGoAllocationTotal(ctx)
				if err != nil {
					return err
				}
				got, queryErr := v.SameReceiverStorage(ctx, gotoolchain.SSACallReceiverStorageRequest{Left: first, Right: last})
				after, err := hostfacts.ObserveGoAllocationTotal(ctx)
				if err != nil {
					return err
				}
				allocated := after.Uint64() - before.Uint64()
				if queryErr != nil || !got.Proven || allocated > 4096 {
					t.Fatalf("receiver query=(%+v,%v) scratch=%d at %d calls, want proved,nil and <=4096 bytes", got, queryErr, allocated, population)
				}
				t.Logf("receiver query scratch=%d at %d calls; compiler construction excluded", allocated, population)
				return nil
			}})
			if err != nil {
				t.Fatalf("WithSSAUnit error=%v, want nil", err)
			}
		})
	}
}
