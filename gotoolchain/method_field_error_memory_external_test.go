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

func TestMethodFieldErrorPropagationAllocatesNoSyntaxPopulation(t *testing.T) {
	testserial.Declare(t, core.TestIsolationDeclaration{Hazard: core.TestIsolationHazardRuntimeAllocation, Scope: core.TestIsolationScopePackageProcess})
	for _, shape := range [...]struct{ name, body string }{
		{"constructed argument", `return (Bridge{Other:v.A,Selected:v.A}).Check()}`},
		{"returned record", `return v.project().Check()}`},
		{"stored returned record", `returned:=v.project();if returned.Selected.Count<0{return returned.Check()};return returned.Check()}`},
	} {
		var small uint64
		for _, extent := range []int{32, 16385} {
			source := `package sample;type refusal uint8;const bad refusal=1;func(refusal)Error()string{return "refusal"};type Child struct{Count int};func(Child)Inspect()error{return bad};type Parent struct{A,B Child};type Bridge struct{Other,Selected Child};func(v Bridge)Check()error{return v.Selected.Inspect()};func(v Parent)project()Bridge{return Bridge{Selected:v.A}};func(v Parent)Check()error{` + strings.Repeat(`_=v.B.Count;`, extent) + shape.body
			analysis := nativeSymbolSourceAnalysis(t, t.TempDir(), source)
			unit := analysis.Units[0]
			method := types.NewMethodSet(unit.Types.Scope().Lookup("Parent").Type()).Lookup(nil, "Check").Obj().(*types.Func)
			target := types.NewMethodSet(unit.Types.Scope().Lookup("Child").Type()).Lookup(nil, "Inspect").Obj().(*types.Func)
			err := analysis.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: unit, SourceValues: true, Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
				request := gotoolchain.MethodFieldErrorPropagationRequest{Analysis: analysis, Method: method, Target: target}
				if got, err := view.MethodFieldErrorPropagation(ctx, request); err != nil || !got.Propagated {
					t.Fatalf("warm field error query=%+v/%v,want propagated target", got, err)
				}
				before, err := hostfacts.ObserveGoAllocationTotal(ctx)
				if err != nil {
					return err
				}
				got, err := view.MethodFieldErrorPropagation(ctx, request)
				if err != nil || !got.Propagated || got.Source != gotoolchain.FunctionSourcePresent {
					t.Fatalf("measured field error query=%+v/%v,want propagated target", got, err)
				}
				after, err := hostfacts.ObserveGoAllocationTotal(ctx)
				if err != nil {
					return err
				}
				allocated := after.Uint64() - before.Uint64()
				if extent == 32 {
					small = allocated
				}
				t.Logf("projection=%s native syntax statements=%d query allocated bytes=%d baseline=%d", shape.name, extent, allocated, small)
				if allocated > small+(64<<10) {
					t.Fatalf("query allocated %d bytes for %d statements,want baseline %d plus at most64KiB without syntax population", allocated, extent, small)
				}
				return nil
			}})
			if err != nil {
				t.Fatal(err)
			}
		}
	}
}
