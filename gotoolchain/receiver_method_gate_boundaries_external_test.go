package gotoolchain_test

import (
	"context"
	"errors"
	"go/types"
	"io"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gotoolchain"
)

func TestReceiverMethodGateRefusesInvalidNativeAuthority(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name   string
		mutate func(*gotoolchain.ReceiverMethodGateRequest, *gotoolchain.SSAUnitView)
		want   error
	}{
		{name: "absent method", mutate: func(r *gotoolchain.ReceiverMethodGateRequest, _ *gotoolchain.SSAUnitView) { r.Method = nil }, want: core.ErrGoToolchainContract},
		{name: "absent target", mutate: func(r *gotoolchain.ReceiverMethodGateRequest, _ *gotoolchain.SSAUnitView) { r.Target = nil }, want: core.ErrGoToolchainContract},
		{name: "absent analysis", mutate: func(r *gotoolchain.ReceiverMethodGateRequest, _ *gotoolchain.SSAUnitView) {
			r.Analysis = gotoolchain.PackageAnalysis{}
		}, want: core.ErrGoToolchainContract},
		{name: "incomplete analysis", mutate: func(r *gotoolchain.ReceiverMethodGateRequest, _ *gotoolchain.SSAUnitView) {
			r.Analysis.Incomplete = true
		}, want: core.ErrGoToolchainOutput},
		{name: "absent native view", mutate: func(_ *gotoolchain.ReceiverMethodGateRequest, v *gotoolchain.SSAUnitView) {
			*v = gotoolchain.SSAUnitView{}
		}, want: core.ErrGoToolchainContract},
		{name: "fabricated source method", mutate: func(r *gotoolchain.ReceiverMethodGateRequest, _ *gotoolchain.SSAUnitView) {
			r.Method = types.NewFunc(r.Method.Pos(), r.Method.Pkg(), r.Method.Name(), r.Method.Signature())
		}, want: core.ErrGoToolchainOutput},
		{name: "fabricated target method", mutate: func(r *gotoolchain.ReceiverMethodGateRequest, _ *gotoolchain.SSAUnitView) {
			r.Target = types.NewFunc(r.Target.Pos(), r.Target.Pkg(), r.Target.Name(), r.Target.Signature())
		}, want: core.ErrGoToolchainContract},
		{name: "method without receiver signature", mutate: func(r *gotoolchain.ReceiverMethodGateRequest, _ *gotoolchain.SSAUnitView) {
			r.Method = types.NewFunc(r.Method.Pos(), r.Method.Pkg(), r.Method.Name(), nil)
		}, want: core.ErrGoToolchainContract},
		{name: "target without receiver signature", mutate: func(r *gotoolchain.ReceiverMethodGateRequest, _ *gotoolchain.SSAUnitView) {
			r.Target = types.NewFunc(r.Target.Pos(), r.Target.Pkg(), r.Target.Name(), nil)
		}, want: core.ErrGoToolchainContract},
		{name: "another record cannot grant target", mutate: func(r *gotoolchain.ReceiverMethodGateRequest, _ *gotoolchain.SSAUnitView) {
			r.Target = types.NewMethodSet(r.Analysis.Units[0].Types.Scope().Lookup("Other").Type()).Lookup(nil, "Inspect").Obj().(*types.Func)
		}, want: core.ErrGoToolchainContract},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			analysis := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;type Record struct{};func(Record)Inspect()error{return nil};func(v *Record)Apply()error{return v.Inspect()};type Other struct{};func(Other)Inspect()error{return nil}`)
			unit := analysis.Units[0]
			record := unit.Types.Scope().Lookup("Record").Type()
			method := types.NewMethodSet(types.NewPointer(record)).Lookup(nil, "Apply").Obj().(*types.Func)
			target := types.NewMethodSet(record).Lookup(nil, "Inspect").Obj().(*types.Func)
			err := analysis.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: unit, SourceValues: true, Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
				request := gotoolchain.ReceiverMethodGateRequest{Analysis: analysis, Method: method, Target: target}
				row.mutate(&request, &view)
				got, err := view.ReceiverMethodGate(ctx, request)
				if !errors.Is(err, row.want) || got != (gotoolchain.ReceiverMethodGateObservation{}) {
					t.Fatalf("native authority refusal=%+v/%v,want zero observation and %v", got, err, row.want)
				}
				return nil
			}})
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestReceiverMethodGateKeepsCancellationCauseAndForeignAbsence(t *testing.T) {
	t.Parallel()
	const source = `package sample;type Record struct{};func(Record)Inspect()error{return nil};func(v *Record)Apply()error{return v.Inspect()}`
	analysis := nativeSymbolSourceAnalysis(t, t.TempDir(), source)
	foreign := nativeSymbolSourceAnalysis(t, t.TempDir(), source)
	unit := analysis.Units[0]
	record := unit.Types.Scope().Lookup("Record").Type()
	method := types.NewMethodSet(types.NewPointer(record)).Lookup(nil, "Apply").Obj().(*types.Func)
	target := types.NewMethodSet(record).Lookup(nil, "Inspect").Obj().(*types.Func)
	err := analysis.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: unit, SourceValues: true, Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
		request := gotoolchain.ReceiverMethodGateRequest{Analysis: analysis, Method: method, Target: target}
		cancelled, cancel := context.WithCancelCause(ctx)
		cancel(io.ErrClosedPipe)
		for _, current := range []context.Context{nil, cancelled} {
			want := error(core.ErrNilContext)
			if current != nil {
				want = io.ErrClosedPipe
			}
			got, err := view.ReceiverMethodGate(current, request)
			if !errors.Is(err, want) || got != (gotoolchain.ReceiverMethodGateObservation{}) {
				t.Errorf("native context refusal=%+v/%v,want zero observation and %v", got, err, want)
			}
		}
		other := foreign.Units[0].Types.Scope().Lookup("Record").Type()
		request.Method = types.NewMethodSet(types.NewPointer(other)).Lookup(nil, "Apply").Obj().(*types.Func)
		request.Target = types.NewMethodSet(other).Lookup(nil, "Inspect").Obj().(*types.Func)
		got, err := view.ReceiverMethodGate(ctx, request)
		if err != nil || got.Validate() != nil || got.Source != gotoolchain.FunctionSourceAbsent || got.Preserved {
			t.Fatalf("foreign source gate=%+v/%v,want absent source without authority", got, err)
		}
		return nil
	}})
	if err != nil {
		t.Fatal(err)
	}
}
