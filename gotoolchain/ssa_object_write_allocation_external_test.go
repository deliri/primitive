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

func TestSSAObjectWrittenRetainsOnlyOneStore(t *testing.T) {
	testserial.Declare(t, core.TestIsolationDeclaration{Hazard: core.TestIsolationHazardRuntimeAllocation, Scope: core.TestIsolationScopePackageProcess})
	for _, population := range []int{32, 16385} {
		t.Run(fmt.Sprint(population), func(t *testing.T) {
			a := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;func consume(int){};func Entry()(result error){defer func(){`+strings.Repeat("consume(1);", population)+`result=nil}();return}`)
			err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], SourceValues: true, Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
				fn := v.Package.Func("Entry")
				r := gotoolchain.SSAObjectWriteRequest{Function: fn.AnonFuncs[0], Object: fn.Signature.Results().At(0)}
				before, err := hostfacts.ObserveGoAllocationTotal(ctx)
				if err != nil {
					return err
				}
				observation, queryErr := v.ObjectWritten(ctx, r)
				after, err := hostfacts.ObserveGoAllocationTotal(ctx)
				if err != nil {
					return err
				}
				allocated := after.Uint64() - before.Uint64()
				if queryErr != nil || !observation.Proven || allocated > 4<<10 {
					t.Fatalf("object write=(%+v,%v), scratch=%d at %d instructions, want proven and <=4096 bytes", observation, queryErr, allocated, population)
				}
				t.Logf("object-write scratch=%d at %d instructions; compiler construction excluded", allocated, population)
				return nil
			}})
			if err != nil {
				t.Fatalf("WithSSAUnit error=%v, want nil", err)
			}
		})
	}
}
