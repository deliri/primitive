package gotoolchain_test

import (
	"context"
	"errors"
	"go/types"
	"io"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gotoolchain"
	"golang.org/x/tools/go/ssa"
)

func TestSSAArgumentCallRefusesInvalidNativeLoans(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name   string
		change func(*gotoolchain.SSAArgumentCallRequest, *gotoolchain.SSAUnitView)
	}{
		{"absent frame", func(r *gotoolchain.SSAArgumentCallRequest, _ *gotoolchain.SSAUnitView) { r.Function = nil }},
		{"absent argument", func(r *gotoolchain.SSAArgumentCallRequest, _ *gotoolchain.SSAUnitView) { r.Argument = nil }},
		{"typed nil argument", func(r *gotoolchain.SSAArgumentCallRequest, _ *gotoolchain.SSAUnitView) {
			r.Argument = (*ssa.Parameter)(nil)
		}},
		{"absent target", func(r *gotoolchain.SSAArgumentCallRequest, _ *gotoolchain.SSAUnitView) { r.Target = nil }},
		{"out of range slot", func(r *gotoolchain.SSAArgumentCallRequest, _ *gotoolchain.SSAUnitView) { r.Index = 1 }},
		{"maximum slot", func(r *gotoolchain.SSAArgumentCallRequest, _ *gotoolchain.SSAUnitView) { r.Index = ^uint64(0) }},
		{"absent native view", func(_ *gotoolchain.SSAArgumentCallRequest, v *gotoolchain.SSAUnitView) {
			*v = gotoolchain.SSAUnitView{}
		}},
		{"fabricated same spelled target", func(r *gotoolchain.SSAArgumentCallRequest, _ *gotoolchain.SSAUnitView) {
			r.Target = types.NewFunc(r.Target.Pos(), r.Target.Pkg(), r.Target.Name(), r.Target.Signature())
		}},
		{"argument belongs to another frame", func(r *gotoolchain.SSAArgumentCallRequest, v *gotoolchain.SSAUnitView) {
			r.Argument = v.Package.Func("Other").Params[0]
		}},
		{"unowned native frame", func(r *gotoolchain.SSAArgumentCallRequest, _ *gotoolchain.SSAUnitView) { r.Function = &ssa.Function{} }},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			analysis := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;type Runner struct{};func(*Runner)Invoke(){};func Entry(r *Runner){r.Invoke()};func Other(r *Runner){}`)
			unit := analysis.Units[0]
			err := analysis.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: unit, Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
				frame := view.Package.Func("Entry")
				target := types.NewMethodSet(frame.Params[0].Type()).Lookup(nil, "Invoke").Obj().(*types.Func)
				request := gotoolchain.SSAArgumentCallRequest{Function: frame, Argument: frame.Params[0], Target: target}
				row.change(&request, &view)
				got, err := view.ArgumentCall(ctx, request)
				if !errors.Is(err, core.ErrGoToolchainContract) || got.Proven {
					t.Fatalf("ArgumentCall(invalid loan) = (%+v, %v), want zero observation and %v", got, err, core.ErrGoToolchainContract)
				}
				return nil
			}})
			if err != nil {
				t.Fatalf("WithSSAUnit(loan refusals) error = %v, want nil", err)
			}
		})
	}
}

func TestSSAArgumentCallConservesCancellationCause(t *testing.T) {
	t.Parallel()
	analysis := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;type Runner struct{};func(*Runner)Invoke(){};func Entry(r *Runner){r.Invoke()}`)
	unit := analysis.Units[0]
	err := analysis.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: unit, Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
		frame := view.Package.Func("Entry")
		target := types.NewMethodSet(frame.Params[0].Type()).Lookup(nil, "Invoke").Obj().(*types.Func)
		request := gotoolchain.SSAArgumentCallRequest{Function: frame, Argument: frame.Params[0], Target: target}
		cancelled, cancel := context.WithCancelCause(ctx)
		cancel(io.ErrClosedPipe)
		for _, row := range []struct {
			ctx  context.Context
			want error
		}{{nil, core.ErrNilContext}, {cancelled, io.ErrClosedPipe}} {
			got, err := view.ArgumentCall(row.ctx, request)
			if !errors.Is(err, row.want) || got.Proven {
				t.Fatalf("ArgumentCall(cancelled) = (%+v, %v), want zero observation and %v", got, err, row.want)
			}
		}
		return nil
	}})
	if err != nil {
		t.Fatalf("WithSSAUnit(cancellation) error = %v, want nil", err)
	}
}

func TestSSAArgumentCallUsesFunctionArgumentSlot(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name, body string
		slot       uint64
		want       bool
	}{
		{"first target slot", `accept(r,&Runner{})`, 0, true},
		{"second target slot cannot borrow first", `accept(r,&Runner{})`, 1, false},
		{"second selected slot", `accept(&Runner{},r)`, 1, true},
		{"first slot cannot borrow second", `accept(&Runner{},r)`, 0, false},
		{"literal capture retains slot", `func(){accept(&Runner{},r)}()`, 1, true},
		{"actual replacement target", `f:=accept;f=reject;f(r,r)`, 0, false},
		{"copied function retains target", `f:=accept;f(r,r)`, 1, true},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			analysis := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;type Runner struct{};func accept(a,b *Runner){};func reject(a,b *Runner){};func Entry(r *Runner){`+row.body+`}`)
			unit := analysis.Units[0]
			err := analysis.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: unit, Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
				frame := view.Package.Func("Entry")
				target := unit.Types.Scope().Lookup("accept").(*types.Func)
				got, err := view.ArgumentCall(ctx, gotoolchain.SSAArgumentCallRequest{Function: frame, Argument: frame.Params[0], Target: target, Index: row.slot})
				if err != nil || got.Proven != row.want {
					t.Fatalf("ArgumentCall(function slot %d) = (%+v, %v), want proven=%t", row.slot, got, err, row.want)
				}
				return nil
			}})
			if err != nil {
				t.Fatalf("WithSSAUnit(function slots) error = %v, want nil", err)
			}
		})
	}
}

func TestSSAArgumentCallRefusesEscapedCaptureIdentity(t *testing.T) {
	t.Parallel()
	analysis := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;type Runner struct{};func(*Runner)Invoke(){};func replace(p **Runner){*p=&Runner{}};func Entry(r *Runner){f:=func(){r.Invoke()};replace(&r);f()}`)
	unit := analysis.Units[0]
	err := analysis.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: unit, Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
		frame := view.Package.Func("Entry")
		target := types.NewMethodSet(frame.Params[0].Type()).Lookup(nil, "Invoke").Obj().(*types.Func)
		got, err := view.ArgumentCall(ctx, gotoolchain.SSAArgumentCallRequest{Function: frame, Argument: frame.Params[0], Target: target})
		if err != nil || got.Proven {
			t.Fatalf("ArgumentCall(escaped capture) = (%+v, %v), want false and nil", got, err)
		}
		return nil
	}})
	if err != nil {
		t.Fatalf("WithSSAUnit(escaped capture) error = %v, want nil", err)
	}
}

func TestSSAArgumentCallBorrowsImportedTargetObjects(t *testing.T) {
	t.Parallel()
	analysis := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;import "testing";func Entry(r *testing.M){r.Run()}`)
	unit := analysis.Units[0]
	err := analysis.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: unit, Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
		frame := view.Package.Func("Entry")
		target := types.NewMethodSet(frame.Params[0].Type()).Lookup(nil, "Run").Obj().(*types.Func)
		got, err := view.ArgumentCall(ctx, gotoolchain.SSAArgumentCallRequest{Function: frame, Argument: frame.Params[0], Target: target})
		if err != nil || !got.Proven {
			t.Fatalf("ArgumentCall(imported method) = (%+v, %v), want true and nil", got, err)
		}
		return nil
	}})
	if err != nil {
		t.Fatalf("WithSSAUnit(imported target) error = %v, want nil", err)
	}
}
