package gotoolchain_test

import (
	"context"
	"go/types"
	"testing"

	"github.com/deliri/primitive/v2026/gotoolchain"
)

func FuzzSSANilBoxedReturnsConserveNativeRepresentation(f *testing.F) {
	for _, mode := range []uint8{0, 1, 2, 4, 8, 16, 32, 63} {
		f.Add(mode)
	}
	f.Fuzz(func(t *testing.T, mode uint8) {
		value := "(*E)(nil)"
		want := 1
		if mode&1 != 0 {
			value = "new(E)"
			want = 0
		}
		if mode&2 != 0 {
			value = "nil"
			want = 0
		}
		result := "error"
		if mode&4 != 0 {
			result = "*E"
			want = 0
		}
		if mode&8 != 0 {
			result = "interface{}"
			want = 0
		}
		body := "return " + value + ";"
		if mode&16 != 0 {
			body = "if false{" + body + "};return nil;"
			want = 0
		}
		if mode&32 != 0 {
			body = "_=func()" + result + "{" + body + "};return nil;"
			want = 0
		}
		a := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;type E struct{};func(*E)Error()string{return "error"};func Entry()`+result+`{`+body+`}`)
		err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
			r := gotoolchain.SSANilBoxedReturnRequest{Function: v.Package.Func("Entry"), Interface: types.Universe.Lookup("error").Type().Underlying().(*types.Interface)}
			observed := 0
			for returned, err := range v.NilBoxedReturns(ctx, r) {
				if err != nil || returned == nil || returned.Parent() != r.Function {
					t.Fatalf("mode %d return=(%v,%v), want selected native return and nil", mode, returned, err)
				}
				observed++
			}
			if observed != want {
				t.Fatalf("mode %d nil-boxed returns=%d, want %d", mode, observed, want)
			}
			return nil
		}})
		if err != nil {
			t.Fatalf("WithSSAUnit error=%v, want nil", err)
		}
	})
}
