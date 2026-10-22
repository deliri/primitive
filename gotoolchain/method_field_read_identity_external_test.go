package gotoolchain_test

import (
	"context"
	"errors"
	"go/types"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gotoolchain"
)

func TestMethodFieldReadRefusesFabricatedIdentityAndWithholdsForeignAuthority(t *testing.T) {
	t.Parallel()
	for _, fabricated := range []bool{false, true} {
		name := "same spelling in a foreign compiler"
		if fabricated {
			name = "fabricated original name and position"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			const source = "package sample;type Record struct{A int};func(v Record)Inspect(){_=v.A}"
			analysis := nativeSymbolSourceAnalysis(t, t.TempDir(), source)
			foreign := nativeSymbolSourceAnalysis(t, t.TempDir(), source)
			unit := analysis.Units[0]
			method := types.NewMethodSet(foreign.Units[0].Types.Scope().Lookup("Record").Type()).Lookup(nil, "Inspect").Obj().(*types.Func)
			if fabricated {
				original := types.NewMethodSet(unit.Types.Scope().Lookup("Record").Type()).Lookup(nil, "Inspect").Obj().(*types.Func)
				method = types.NewFunc(original.Pos(), original.Pkg(), original.Name(), original.Signature())
			}
			err := analysis.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: unit, SourceValues: true, Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
				got, err := view.MethodFieldRead(ctx, gotoolchain.MethodFieldReadRequest{Analysis: analysis, Method: method})
				if fabricated {
					if !errors.Is(err, core.ErrGoToolchainOutput) || got != (gotoolchain.MethodFieldReadObservation{}) {
						t.Fatalf("fabricated method=%+v/%v,want zero observation and native output refusal", got, err)
					}
				} else if err != nil || got.Read || got.Source != gotoolchain.FunctionSourceAbsent || got.Validate() != nil {
					t.Fatalf("foreign method=%+v/%v,want absent source without read authority", got, err)
				}
				return nil
			}})
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestMethodFieldReadCannotGrantOpaqueImportedHelper(t *testing.T) {
	t.Parallel()
	analysis := nativeSymbolSourceAnalysis(t, t.TempDir(), "package sample\nimport \"fmt\"\ntype Record struct{A int}\nfunc(v Record)Inspect(){_=fmt.Sprint(v)}")
	unit := analysis.Units[0]
	method := types.NewMethodSet(unit.Types.Scope().Lookup("Record").Type()).Lookup(nil, "Inspect").Obj().(*types.Func)
	err := analysis.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: unit, SourceValues: true, Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
		got, err := view.MethodFieldRead(ctx, gotoolchain.MethodFieldReadRequest{Analysis: analysis, Method: method})
		if err != nil || got.Read || got.Source != gotoolchain.FunctionSourcePresent {
			t.Fatalf("opaque helper observation=%+v/%v,want known owning method and no invented imported field read", got, err)
		}
		return nil
	}})
	if err != nil {
		t.Fatal(err)
	}
}
