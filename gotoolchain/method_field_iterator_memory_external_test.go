package gotoolchain_test

import (
	"context"
	"go/types"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gotoolchain"
	"github.com/deliri/primitive/v2026/hostfacts"
	"github.com/deliri/primitive/v2026/testserial"
)

func TestMethodFieldIteratorErrorRetainsNoSyntaxPopulation(t *testing.T) {
	testserial.Declare(t, core.TestIsolationDeclaration{Hazard: core.TestIsolationHazardRuntimeAllocation, Scope: core.TestIsolationScopePackageProcess})
	var small uint64
	for _, extent := range []int{32, 16385} {
		source := `package sample
import "iter"
type refusal uint8
const bad refusal=1
func(refusal)Error()string{return "refusal"}
type Child struct{Count int}
func(Child)Inspect()error{return bad}
type Parent struct{A,B Child}
func(p Parent)items()iter.Seq2[int,error]{return func(yield func(int,error)bool){refuse:=func(err error){yield(0,err)};refuse(p.A.Inspect())}}
func(p Parent)Check()error{` + strings.Repeat(`_=p.B.Count;`, extent) + `for _,err:=range p.items(){if err!=nil{return err}};return nil}`
		analysis := nativeSymbolSourceAnalysis(t, t.TempDir(), source)
		unit := analysis.Units[0]
		method := types.NewMethodSet(unit.Types.Scope().Lookup("Parent").Type()).Lookup(nil, "Check").Obj().(*types.Func)
		target := types.NewMethodSet(unit.Types.Scope().Lookup("Child").Type()).Lookup(nil, "Inspect").Obj().(*types.Func)
		if err := analysis.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: unit, SourceValues: true, Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
			request := gotoolchain.MethodFieldErrorPropagationRequest{Analysis: analysis, Method: method, Target: target}
			if got, err := view.MethodFieldErrorPropagation(ctx, request); err != nil || !got.Propagated {
				t.Fatalf("warm iterator query=%+v/%v,want propagated", got, err)
			}
			before, err := hostfacts.ObserveGoAllocationTotal(ctx)
			if err != nil {
				return err
			}
			got, err := view.MethodFieldErrorPropagation(ctx, request)
			if err != nil || got.Validate() != nil || !got.Propagated {
				t.Fatalf("measured iterator query=%+v/%v,want propagated", got, err)
			}
			after, err := hostfacts.ObserveGoAllocationTotal(ctx)
			if err != nil {
				return err
			}
			allocated := after.Uint64() - before.Uint64()
			if extent == 32 {
				small = allocated
			}
			t.Logf("iterator statements=%d query allocated=%d baseline=%d", extent, allocated, small)
			if allocated > small+(64<<10) {
				t.Fatalf("iterator allocated=%d for statements=%d,want baseline=%d plus64KiB without syntax population", allocated, extent, small)
			}
			return nil
		}}); err != nil {
			t.Fatalf("native SSA scope=%v,want nil", err)
		}
	}
}
