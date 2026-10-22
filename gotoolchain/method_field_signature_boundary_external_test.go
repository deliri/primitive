package gotoolchain_test

import (
	"context"
	"errors"
	"go/types"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gotoolchain"
)

func TestMethodFieldQueriesRefuseAbsentNativeSignatures(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name         string
		read, target bool
	}{
		{name: "read method signature absent", read: true},
		{name: "propagation method signature absent"},
		{name: "propagation target signature absent", target: true},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			analysis := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;type Child struct{};func(Child)Inspect()error{return nil};type Parent struct{A Child};func(v Parent)Check()error{return v.A.Inspect()}`)
			unit := analysis.Units[0]
			method := types.NewMethodSet(unit.Types.Scope().Lookup("Parent").Type()).Lookup(nil, "Check").Obj().(*types.Func)
			target := types.NewMethodSet(unit.Types.Scope().Lookup("Child").Type()).Lookup(nil, "Inspect").Obj().(*types.Func)
			if row.target {
				target = types.NewFunc(target.Pos(), target.Pkg(), target.Name(), nil)
			} else {
				method = types.NewFunc(method.Pos(), method.Pkg(), method.Name(), nil)
			}
			err := analysis.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: unit, SourceValues: true, Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
				if row.read {
					got, err := view.MethodFieldRead(ctx, gotoolchain.MethodFieldReadRequest{Analysis: analysis, Method: method})
					if !errors.Is(err, core.ErrGoToolchainContract) || got != (gotoolchain.MethodFieldReadObservation{}) {
						t.Fatalf("absent signature read=%+v/%v,want zero observation and native contract refusal", got, err)
					}
				} else {
					got, err := view.MethodFieldErrorPropagation(ctx, gotoolchain.MethodFieldErrorPropagationRequest{Analysis: analysis, Method: method, Target: target})
					if !errors.Is(err, core.ErrGoToolchainContract) || got != (gotoolchain.MethodFieldErrorPropagationObservation{}) {
						t.Fatalf("absent signature propagation=%+v/%v,want zero observation and native contract refusal", got, err)
					}
				}
				return nil
			}})
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}
