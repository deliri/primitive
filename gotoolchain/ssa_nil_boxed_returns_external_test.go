package gotoolchain_test

import (
	"context"
	"go/types"
	"testing"

	"github.com/deliri/primitive/v2026/gotoolchain"
)

func TestSSANilBoxedReturnsUseNativeInterfaceRepresentation(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name, source string
		want         int
	}{
		{"nil interface remains nil", `func Entry()error{return nil}`, 0},
		{"nil pointer boxes nonnil interface", `func Entry()error{return (*E)(nil)}`, 1},
		{"allocated pointer is not nil", `func Entry()error{return &E{}}`, 0},
		{"unknown pointer supplies no nil fact", `func Entry(value *E)error{return value}`, 0},
		{"copied box retains representation", `func Entry()error{var value *E;var err error=value;return err}`, 1},
		{"interface conversion retains representation", `type marked interface{error;Mark()};func(*E)Mark(){};func Entry()error{var value *E;var err marked=value;return err}`, 1},
		{"later result slot retains nil fact", `func Entry()(error,error){return nil,(*E)(nil)}`, 1},
		{"one native return is lent once", `func Entry()(error,error){return (*E)(nil),(*E)(nil)}`, 1},
		{"untaken return supplies no fact", `func Entry()error{if false{return (*E)(nil)};return nil}`, 0},
		{"runtime branch returns stay separate", `func Entry(flag bool)error{if flag{return (*E)(nil)};return (*E)(nil)}`, 2},
		{"unselected interface is no target", `func Entry()interface{}{return (*E)(nil)}`, 0},
		{"concrete pointer return is no interface", `func Entry()*E{return nil}`, 0},
		{"uninvoked nested return is outside function", `func Entry()error{_=func()error{return (*E)(nil)};return nil}`, 0},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			a := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;type E struct{};func(*E)Error()string{return "error"};`+row.source)
			err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
				request := gotoolchain.SSANilBoxedReturnRequest{Function: v.Package.Func("Entry"), Interface: types.Universe.Lookup("error").Type().Underlying().(*types.Interface)}
				observed := 0
				for returned, err := range v.NilBoxedReturns(ctx, request) {
					if err != nil || returned == nil || returned.Parent() != request.Function {
						t.Fatalf("nil-boxed return=(%v,%v), want native selected return and nil", returned, err)
					}
					observed++
				}
				if observed != row.want {
					t.Fatalf("nil-boxed return observations=%d, want %d", observed, row.want)
				}
				return nil
			}})
			if err != nil {
				t.Fatalf("WithSSAUnit error=%v, want nil", err)
			}
		})
	}
}
