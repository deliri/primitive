package gotoolchain_test

import (
	"context"
	"go/types"
	"testing"

	"github.com/deliri/primitive/v2026/gotoolchain"
)

func TestSSAArgumentCallFollowsNativePromotedReceiverPath(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name, body string
		want       bool
	}{
		{"selected promoted receiver", `r.Invoke()`, true},
		{"explicit native field", `r.Cell.Invoke()`, true},
		{"neighbor field cannot lend selected path", `r.Other.Invoke()`, false},
		{"foreign owner cannot lend selected path", `(&Owner{}).Invoke()`, false},
		{"bound promoted receiver", `f:=r.Invoke;f()`, true},
		{"literal retains promoted root", `func(){r.Invoke()}()`, true},
		{"literal receives promoted field", `func(c *Cell){c.Invoke()}(&r.Cell)`, true},
		{"literal receives other field", `func(c *Cell){c.Invoke()}(&r.Other)`, false},
		{"replaced root loses selected path", `r=&Owner{};r.Invoke()`, false},
		{"uninvoked promoted method", `f:=r.Invoke;_=f`, false},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			analysis := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;type Cell struct{Count int};func(*Cell)Invoke(){};type Owner struct{Cell;Other Cell};func Entry(r *Owner){`+row.body+`}`)
			unit := analysis.Units[0]
			err := analysis.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: unit, Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
				frame := view.Package.Func("Entry")
				target := types.NewMethodSet(frame.Params[0].Type()).Lookup(nil, "Invoke").Obj().(*types.Func)
				got, err := view.ArgumentCall(ctx, gotoolchain.SSAArgumentCallRequest{Timing: gotoolchain.SSACallTimingSynchronous, Function: frame, Argument: frame.Params[0], Target: target})
				if err != nil || got.Proven != row.want {
					t.Fatalf("ArgumentCall(promoted) = (%+v, %v), want proven=%t", got, err, row.want)
				}
				return nil
			}})
			if err != nil {
				t.Fatalf("WithSSAUnit(promoted calls) error = %v, want nil", err)
			}
		})
	}
}

func TestSSAArgumentCallUsesActualPromotedPathShape(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name, declarations, body string
		want                     bool
	}{
		{"nested value fields", `type Mid struct{Cell};type Owner struct{Mid;Other Cell}`, `r.Invoke()`, true},
		{"nested pointer fields", `type Mid struct{*Cell};type Owner struct{*Mid;Other Cell}`, `r.Invoke()`, true},
		{"nested neighbor is refused", `type Mid struct{*Cell};type Owner struct{*Mid;Other Cell}`, `r.Other.Invoke()`, false},
		{"last declared embedded field", `type Owner struct{Other Cell;Cell}`, `r.Invoke()`, true},
		{"last embedded field cannot borrow first", `type Owner struct{Other Cell;Cell}`, `r.Other.Invoke()`, false},
		{"native alias retains path", `type Root struct{Cell};type Owner=Root`, `r.Invoke()`, true},
		{"method expression keeps field", `type Owner struct{Cell}`, `(*Cell).Invoke(&r.Cell)`, true},
		{"copied value loses native field storage", `type Owner struct{Cell}`, `copy:=r.Cell;copy.Invoke()`, false},
		{"native representation conversion retains path", `type Owner struct{Cell};type OtherOwner Owner`, `converted:=(*OtherOwner)(r);converted.Cell.Invoke()`, true},
		{"declared method hides embedded target", `type Owner struct{Cell};func(*Owner)Invoke(){}`, `r.Cell.Invoke()`, false},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			analysis := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;type Cell struct{Count int};func(*Cell)Invoke(){};`+row.declarations+`;func Entry(r *Owner){`+row.body+`}`)
			unit := analysis.Units[0]
			err := analysis.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: unit, Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
				frame := view.Package.Func("Entry")
				target := types.NewMethodSet(frame.Params[0].Type()).Lookup(nil, "Invoke").Obj().(*types.Func)
				got, err := view.ArgumentCall(ctx, gotoolchain.SSAArgumentCallRequest{Function: frame, Argument: frame.Params[0], Target: target, Timing: gotoolchain.SSACallTimingSynchronous})
				if err != nil || got.Proven != row.want {
					t.Fatalf("ArgumentCall(promoted shape) = (%+v, %v), want proven=%t", got, err, row.want)
				}
				return nil
			}})
			if err != nil {
				t.Fatalf("WithSSAUnit(promoted shape) error = %v, want nil", err)
			}
		})
	}
}
