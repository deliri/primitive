package gotoolchain_test

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gotoolchain"
	"golang.org/x/tools/go/ssa"
)

func TestSSAReceiverParameterConservesActualCallIdentity(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name, declarations, parameters, body string
		want                                 bool
	}{
		{"direct receiver", "", "r *Cell, other *Cell", `r.Invoke(1)`, true},
		{"copied receiver", "", "r *Cell, other *Cell", `copy:=r;copy.Invoke(1)`, true},
		{"foreign receiver", "", "r *Cell, other *Cell", `other.Invoke(1)`, false},
		{"replacement receiver", "", "r *Cell, other *Cell", `r=other;r.Invoke(1)`, false},
		{"opaque replacement", "func replace(p **Cell){*p=new(Cell)}", "r *Cell, other *Cell", `replace(&r);r.Invoke(1)`, false},
		{"read only capture", "", "r *Cell, other *Cell", `unused:=func(){_=r};_=unused;r.Invoke(1)`, true},
		{"bound receiver with explicit argument", "", "r *Cell, other *Cell", `bound:=r.Invoke;bound(1)`, true},
		{"direct method expression", "", "r *Cell, other *Cell", `(*Cell).Invoke(r,1)`, true},
		{"promoted method expression", "type Owner struct{*Cell}", "r *Owner, other *Owner", `(*Owner).Invoke(r,1)`, true},
		{"foreign promoted method expression", "type Owner struct{*Cell}", "r *Owner, other *Owner", `(*Owner).Invoke(other,1)`, false},
		{"explicit promoted base expression", "type Owner struct{*Cell;Other *Cell}", "r *Owner, other *Owner", `(*Cell).Invoke(r.Cell,1)`, true},
		{"expression neighbor receiver", "type Owner struct{*Cell;Other *Cell}", "r *Owner, other *Owner", `(*Cell).Invoke(r.Other,1)`, false},
		{"foreign bound receiver", "", "r *Cell, other *Cell", `bound:=other.Invoke;bound(1)`, false},
		{"interface receiver", "", "r Runner, other Runner", `r.Invoke(1)`, true},
		{"foreign interface receiver", "", "r Runner, other Runner", `other.Invoke(1)`, false},
		{"embedded pointer receiver", "type Owner struct{*Cell;Other *Cell}", "r *Owner, other *Owner", `r.Invoke(1)`, true},
		{"embedded neighbor receiver", "type Owner struct{*Cell;Other *Cell}", "r *Owner, other *Owner", `r.Other.Invoke(1)`, false},
		{"embedded interface receiver", "type Owner struct{Runner}", "r *Owner, other *Owner", `r.Invoke(1)`, true},
		{"embedded value parameter", "type Owner struct{*Cell}", "r Owner, other Owner", `r.Invoke(1)`, true},
		{"ordinary function neutral", "func consume(*Cell){}", "r *Cell, other *Cell", `consume(r)`, false},
		{"global receiver remains neutral", "var global *Cell", "r *Cell, other *Cell", `global.Invoke(1)`, false},
		{"nil pointer receiver remains neutral", "", "r *Cell, other *Cell", `r=nil;r.Invoke(1)`, false},
		{"actual other call cannot lend authority", "", "r *Cell, other *Cell", `r.Invoke(1);other.Invoke(2)`, false},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			a := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;type Cell struct{};func(*Cell)Invoke(int){};type Runner interface{Invoke(int)};`+row.declarations+`
func Entry(`+row.parameters+`){`+row.body+`}`)
			err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], SourceValues: true, Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
				fn := view.Package.Func("Entry")
				call := nativeReceiverParameterLastCall(ctx, t, view, fn)
				got, queryErr := view.ReceiverParameter(ctx, gotoolchain.SSAReceiverParameterRequest{Call: call, Parameter: fn.Params[0]})
				if queryErr != nil || got.Proven != row.want {
					t.Fatalf("receiver parameter=%+v,%v, want proven=%t,nil", got, queryErr, row.want)
				}
				return nil
			}})
			if err != nil {
				t.Fatalf("WithSSAUnit error=%v, want nil", err)
			}
		})
	}
}

func nativeReceiverParameterLastCall(ctx context.Context, t testing.TB, view gotoolchain.SSAUnitView, fn *ssa.Function) *ssa.Call {
	t.Helper()
	var last *ssa.Call
	for instruction, queryErr := range view.Instructions(ctx, gotoolchain.SSAInstructionRequest{Function: fn, Extent: gotoolchain.SSAInstructionExtentFunction}) {
		if queryErr != nil {
			t.Fatalf("receiver fixture refusal=%v, want nil", queryErr)
		}
		if call, ok := instruction.(*ssa.Call); ok {
			last = call
		}
	}
	if last == nil {
		t.Fatal("receiver fixture calls=0, want at least one native call")
	}
	return last
}

func TestSSAReceiverParameterDistinguishesBoundReceiverFromMatchingArgument(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name, body string
		want       bool
	}{
		{"bound original receiver", `bound:=r.Invoke;bound(other)`, true},
		{"bound foreign receiver", `bound:=other.Invoke;bound(r)`, false},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			a := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;type Cell struct{};type Owner struct{*Cell};func(*Cell)Invoke(*Owner){};func Entry(r,other *Owner){`+row.body+`}`)
			err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
				fn := view.Package.Func("Entry")
				got, queryErr := view.ReceiverParameter(ctx, gotoolchain.SSAReceiverParameterRequest{Call: nativeReceiverParameterLastCall(ctx, t, view, fn), Parameter: fn.Params[0]})
				if queryErr != nil || got.Proven != row.want {
					t.Fatalf("bound receiver=%+v,%v, want proven=%t,nil", got, queryErr, row.want)
				}
				return nil
			}})
			if err != nil {
				t.Fatalf("WithSSAUnit error=%v, want nil", err)
			}
		})
	}
}

func TestSSAReceiverParameterPreservesCustodyAndCause(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name string
		mode uint8
		want error
	}{
		{"missing call", 0, core.ErrGoToolchainContract},
		{"missing parameter", 1, core.ErrGoToolchainContract},
		{"foreign parameter", 2, core.ErrGoToolchainContract},
		{"detached parameter", 3, core.ErrGoToolchainContract},
		{"malformed call ordinal", 4, core.ErrGoToolchainOutput},
		{"missing loan", 5, core.ErrGoToolchainContract},
		{"native cancellation cause", 6, io.ErrClosedPipe},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			a := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;type Cell struct{};func(*Cell)Invoke(int){};func Entry(r *Cell){r.Invoke(1)};func Other(r *Cell){}`)
			err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
				fn := view.Package.Func("Entry")
				r := gotoolchain.SSAReceiverParameterRequest{Call: nativeReceiverParameterLastCall(ctx, t, view, fn), Parameter: fn.Params[0]}
				switch row.mode {
				case 0:
					r.Call = nil
				case 1:
					r.Parameter = nil
				case 2:
					r.Parameter = view.Package.Func("Other").Params[0]
				case 3:
					copied := *r.Parameter
					r.Parameter = &copied
				case 4:
					r.Call.Block().Index += 100
				case 5:
					view = gotoolchain.SSAUnitView{}
				case 6:
					cancelled, cancel := context.WithCancelCause(ctx)
					cancel(io.ErrClosedPipe)
					ctx = cancelled
				default:
					t.Fatalf("receiver boundary mode=%d, want declared case", row.mode)
				}
				got, queryErr := view.ReceiverParameter(ctx, r)
				if got != (gotoolchain.SSARelationObservation{}) || !errors.Is(queryErr, row.want) {
					t.Fatalf("receiver custody refusal=%+v,%v, want zero,%v", got, queryErr, row.want)
				}
				return nil
			}})
			if err != nil {
				t.Fatalf("WithSSAUnit error=%v, want nil", err)
			}
		})
	}
}
