package gotoolchain_test

import (
	"context"
	"errors"
	"go/types"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gotoolchain"
)

func TestSSACallTimingHasClosedNativeContract(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		value gotoolchain.SSACallTiming
		want  bool
		text  string
	}{
		{gotoolchain.SSACallTimingUnknown, false, core.UnknownEnumDiagnostic},
		{gotoolchain.SSACallTimingImmediate, true, "immediate"},
		{gotoolchain.SSACallTimingSynchronous, true, "synchronous"},
		{gotoolchain.SSACallTiming(255), false, core.UnknownEnumDiagnostic},
	} {
		got := row.value.Validate()
		if row.value.IsValid() != row.want || (got == nil) != row.want || (!row.want && !errors.Is(got, core.ErrGoToolchainContract)) {
			t.Errorf("timing %d validation = %v/%t, want valid=%t", row.value, got, row.value.IsValid(), row.want)
		}
		if got := row.value.String(); got != row.text {
			t.Errorf("timing %d rendering = %q, want %q", row.value, got, row.text)
		}
	}
}
func TestSSAArgumentCallHonorsExplicitCallTiming(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name, body string
		timing     gotoolchain.SSACallTiming
		want       bool
	}{
		{"immediate call", `r.Invoke()`, gotoolchain.SSACallTimingImmediate, true},
		{"synchronous immediate call", `r.Invoke()`, gotoolchain.SSACallTimingSynchronous, true},
		{"deferred call is not immediate", `defer r.Invoke()`, gotoolchain.SSACallTimingImmediate, false},
		{"deferred call is synchronous", `defer r.Invoke()`, gotoolchain.SSACallTimingSynchronous, true},
		{"deferred literal is not immediate", `defer func(){r.Invoke()}()`, gotoolchain.SSACallTimingImmediate, false},
		{"deferred literal is synchronous", `defer func(){r.Invoke()}()`, gotoolchain.SSACallTimingSynchronous, true},
		{"spawn is not immediate", `go r.Invoke()`, gotoolchain.SSACallTimingImmediate, false},
		{"spawn is not synchronous", `go r.Invoke()`, gotoolchain.SSACallTimingSynchronous, false},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			analysis := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;type Runner struct{};func(*Runner)Invoke(){};func Entry(r *Runner){`+row.body+`}`)
			unit := analysis.Units[0]
			err := analysis.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: unit, Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
				frame := view.Package.Func("Entry")
				target := types.NewMethodSet(frame.Params[0].Type()).Lookup(nil, "Invoke").Obj().(*types.Func)
				got, err := view.ArgumentCall(ctx, gotoolchain.SSAArgumentCallRequest{Function: frame, Argument: frame.Params[0], Target: target, Timing: row.timing})
				if err != nil || got.Proven != row.want {
					t.Fatalf("ArgumentCall(timing %s) = (%+v, %v), want proven=%t", row.timing, got, err, row.want)
				}
				return nil
			}})
			if err != nil {
				t.Fatalf("WithSSAUnit(call timing) error = %v, want nil", err)
			}
		})
	}
}
