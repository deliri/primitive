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

func TestMethodFieldErrorPropagationRefusesInvalidNativeCapabilities(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name   string
		mutate func(*gotoolchain.MethodFieldErrorPropagationRequest, *gotoolchain.SSAUnitView)
		want   error
	}{
		{name: "missing method", mutate: func(r *gotoolchain.MethodFieldErrorPropagationRequest, _ *gotoolchain.SSAUnitView) { r.Method = nil }, want: core.ErrGoToolchainContract},
		{name: "missing target", mutate: func(r *gotoolchain.MethodFieldErrorPropagationRequest, _ *gotoolchain.SSAUnitView) { r.Target = nil }, want: core.ErrGoToolchainContract},
		{name: "ordinal beyond record", mutate: func(r *gotoolchain.MethodFieldErrorPropagationRequest, _ *gotoolchain.SSAUnitView) { r.Field = 1 }, want: core.ErrGoToolchainContract},
		{name: "ordinal maximum cannot truncate", mutate: func(r *gotoolchain.MethodFieldErrorPropagationRequest, _ *gotoolchain.SSAUnitView) {
			r.Field = ^uint64(0)
		}, want: core.ErrGoToolchainContract},
		{name: "missing analysis", mutate: func(r *gotoolchain.MethodFieldErrorPropagationRequest, _ *gotoolchain.SSAUnitView) {
			r.Analysis = gotoolchain.PackageAnalysis{}
		}, want: core.ErrGoToolchainContract},
		{name: "incomplete analysis", mutate: func(r *gotoolchain.MethodFieldErrorPropagationRequest, _ *gotoolchain.SSAUnitView) {
			r.Analysis.Incomplete = true
		}, want: core.ErrGoToolchainOutput},
		{name: "missing view", mutate: func(_ *gotoolchain.MethodFieldErrorPropagationRequest, v *gotoolchain.SSAUnitView) {
			*v = gotoolchain.SSAUnitView{}
		}, want: core.ErrGoToolchainContract},
		{name: "fabricated target name and position", mutate: func(r *gotoolchain.MethodFieldErrorPropagationRequest, _ *gotoolchain.SSAUnitView) {
			r.Target = types.NewFunc(r.Target.Pos(), r.Target.Pkg(), r.Target.Name(), r.Target.Signature())
		}, want: core.ErrGoToolchainContract},
		{name: "fabricated method name and position", mutate: func(r *gotoolchain.MethodFieldErrorPropagationRequest, _ *gotoolchain.SSAUnitView) {
			r.Method = types.NewFunc(r.Method.Pos(), r.Method.Pkg(), r.Method.Name(), r.Method.Signature())
		}, want: core.ErrGoToolchainOutput},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			analysis := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;type Child struct{};func(Child)Inspect()error{return nil};type Parent struct{A Child};func(v Parent)Check()error{return v.A.Inspect()}`)
			unit := analysis.Units[0]
			method := types.NewMethodSet(unit.Types.Scope().Lookup("Parent").Type()).Lookup(nil, "Check").Obj().(*types.Func)
			target := types.NewMethodSet(unit.Types.Scope().Lookup("Child").Type()).Lookup(nil, "Inspect").Obj().(*types.Func)
			err := analysis.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: unit, SourceValues: true, Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
				request := gotoolchain.MethodFieldErrorPropagationRequest{Analysis: analysis, Method: method, Target: target}
				row.mutate(&request, &view)
				got, err := view.MethodFieldErrorPropagation(ctx, request)
				if !errors.Is(err, row.want) || got != (gotoolchain.MethodFieldErrorPropagationObservation{}) {
					t.Fatalf("native refusal=%+v/%v,want zero observation and %v", got, err, row.want)
				}
				return nil
			}})
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestMethodFieldErrorPropagationPreservesCancellationAndForeignAbsence(t *testing.T) {
	t.Parallel()
	const source = `package sample;type Child struct{};func(Child)Inspect()error{return nil};type Parent struct{A Child};func(v Parent)Check()error{return v.A.Inspect()}`
	analysis := nativeSymbolSourceAnalysis(t, t.TempDir(), source)
	foreign := nativeSymbolSourceAnalysis(t, t.TempDir(), source)
	unit := analysis.Units[0]
	method := types.NewMethodSet(unit.Types.Scope().Lookup("Parent").Type()).Lookup(nil, "Check").Obj().(*types.Func)
	target := types.NewMethodSet(unit.Types.Scope().Lookup("Child").Type()).Lookup(nil, "Inspect").Obj().(*types.Func)
	err := analysis.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: unit, SourceValues: true, Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
		request := gotoolchain.MethodFieldErrorPropagationRequest{Analysis: analysis, Method: method, Target: target}
		cancelled, cancel := context.WithCancelCause(ctx)
		cancel(io.ErrClosedPipe)
		for _, current := range []context.Context{nil, cancelled} {
			want := error(core.ErrNilContext)
			if current != nil {
				want = io.ErrClosedPipe
			}
			got, err := view.MethodFieldErrorPropagation(current, request)
			if !errors.Is(err, want) || got != (gotoolchain.MethodFieldErrorPropagationObservation{}) {
				t.Errorf("context refusal=%+v/%v,want zero observation and %v", got, err, want)
			}
		}
		request.Method = types.NewMethodSet(foreign.Units[0].Types.Scope().Lookup("Parent").Type()).Lookup(nil, "Check").Obj().(*types.Func)
		request.Target = types.NewMethodSet(foreign.Units[0].Types.Scope().Lookup("Child").Type()).Lookup(nil, "Inspect").Obj().(*types.Func)
		got, err := view.MethodFieldErrorPropagation(ctx, request)
		if err != nil || got.Validate() != nil || got.Source != gotoolchain.FunctionSourceAbsent || got.Propagated {
			t.Fatalf("foreign method observation=%+v/%v,want absent source without propagation authority", got, err)
		}
		return nil
	}})
	if err != nil {
		t.Fatal(err)
	}
}

func TestMethodFieldErrorObservationHasClosedSourceAuthority(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		observation gotoolchain.MethodFieldErrorPropagationObservation
		valid       bool
	}{
		{observation: gotoolchain.MethodFieldErrorPropagationObservation{Source: gotoolchain.FunctionSourcePresent}, valid: true},
		{observation: gotoolchain.MethodFieldErrorPropagationObservation{Source: gotoolchain.FunctionSourcePresent, Propagated: true}, valid: true},
		{observation: gotoolchain.MethodFieldErrorPropagationObservation{Source: gotoolchain.FunctionSourceAbsent}, valid: true},
		{observation: gotoolchain.MethodFieldErrorPropagationObservation{Source: gotoolchain.FunctionSourceAbsent, Propagated: true}},
		{observation: gotoolchain.MethodFieldErrorPropagationObservation{}},
		{observation: gotoolchain.MethodFieldErrorPropagationObservation{Source: gotoolchain.FunctionSourcePresence(255)}},
	} {
		err := row.observation.Validate()
		if row.valid && err != nil || !row.valid && !errors.Is(err, core.ErrGoToolchainContract) {
			t.Errorf("source authority=%+v validation=%v,want valid=%t", row.observation, err, row.valid)
		}
	}
}

func TestMethodFieldErrorPropagationRejectsNonRecordAndForeignTargetContracts(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name, fieldType, parent, target string
	}{
		{name: "scalar receiver", parent: "Scalar"},
		{name: "free function receiver", parent: "Plain"},
		{name: "scalar field has no target record", fieldType: "int"},
		{name: "recursive container has no terminal record", fieldType: "Loop", target: "Loop"},
		{name: "other record method cannot grant target", target: "Other"},
		{name: "free function cannot grant method target", target: "Plain"},
		{name: "target without owning package", target: "unowned"},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			fieldType := row.fieldType
			if fieldType == "" {
				fieldType = "Child"
			}
			analysis := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;type Child struct{};func(Child)Inspect()error{return nil};type Other struct{};func(Other)Inspect()error{return nil};type Loop []Loop;func(Loop)Inspect()error{return nil};type Scalar int;func(Scalar)Check()error{return nil};func Plain()error{return nil};type Parent struct{A `+fieldType+`};func(Parent)Check()error{return nil}`)
			unit := analysis.Units[0]
			parent := row.parent
			if parent == "" {
				parent = "Parent"
			}
			var method *types.Func
			if parent == "Plain" {
				method = unit.Types.Scope().Lookup(parent).(*types.Func)
			} else {
				method = types.NewMethodSet(unit.Types.Scope().Lookup(parent).Type()).Lookup(nil, "Check").Obj().(*types.Func)
			}
			target := row.target
			if target == "" || target == "unowned" {
				target = "Child"
			}
			var selected *types.Func
			if target == "Plain" {
				selected = unit.Types.Scope().Lookup(target).(*types.Func)
			} else {
				selected = types.NewMethodSet(unit.Types.Scope().Lookup(target).Type()).Lookup(nil, "Inspect").Obj().(*types.Func)
			}
			if row.target == "unowned" {
				selected = types.NewFunc(selected.Pos(), nil, selected.Name(), selected.Signature())
			}
			err := analysis.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: unit, SourceValues: true, Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
				got, err := view.MethodFieldErrorPropagation(ctx, gotoolchain.MethodFieldErrorPropagationRequest{Analysis: analysis, Method: method, Target: selected})
				if !errors.Is(err, core.ErrGoToolchainContract) || got != (gotoolchain.MethodFieldErrorPropagationObservation{}) {
					t.Fatalf("invalid method/field/target=%s/%s/%s observation=%+v/%v,want zero observation and native contract refusal", parent, fieldType, target, got, err)
				}
				return nil
			}})
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}
