package gotoolchain_test

import (
	"context"
	"go/types"
	"testing"

	"github.com/deliri/primitive/v2026/gotoolchain"
)

func TestMethodFieldReadNamedReturnedIterators(t *testing.T) {
	t.Parallel()
	for _, row := range []struct{ name, declarations string }{
		{"local named iterator", `type Rows func(func(int)bool);func(x Record)iterate()Rows{return func(yield func(int)bool){yield(x.A)}}`},
		{"standard single value iterator", `import "iter";func(x Record)iterate()iter.Seq[int]{return func(yield func(int)bool){yield(x.A)}}`},
		{"standard value and error iterator", `import "iter";func(x Record)iterate()iter.Seq2[int,error]{return func(yield func(int,error)bool){yield(x.A,nil)}}`},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			a := nativeSymbolSourceAnalysis(t, t.TempDir(), "package sample;"+row.declarations+`;type Record struct{A int};func(v Record)Inspect(){for range v.iterate(){}}`)
			unit := a.Units[0]
			method := types.NewMethodSet(unit.Types.Scope().Lookup("Record").Type()).Lookup(nil, "Inspect").Obj().(*types.Func)
			err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: unit, SourceValues: true, Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
				got, err := view.MethodFieldRead(ctx, gotoolchain.MethodFieldReadRequest{Analysis: a, Method: method, Field: 0})
				if err != nil || got.Source != gotoolchain.FunctionSourcePresent || !got.Read {
					t.Fatalf("named iterator read=%+v/%v, want present native read", got, err)
				}
				return nil
			}})
			if err != nil {
				t.Fatalf("named iterator unit error=%v, want nil", err)
			}
		})
	}
}
