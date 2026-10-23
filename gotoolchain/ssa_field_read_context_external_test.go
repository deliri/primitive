package gotoolchain_test

import (
	"context"
	"errors"
	"go/token"
	"go/types"
	"io"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gotoolchain"
	"golang.org/x/tools/go/ssa"
)

func TestSSAFieldReadSchemaLayerTriad(t *testing.T) {
	t.Parallel()
	a := nativeSymbolSourceAnalysis(t, t.TempDir(), nativeFieldReadSource)
	err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
		read := nativeFieldReadInstruction(t, view.Package.Func("Entry"), true)
		actual, queryErr := view.FieldRead(ctx, gotoolchain.SSAFieldReadRequest{Read: read})
		if queryErr != nil || actual.Base == nil || actual.Field == nil {
			t.Fatalf("schema source got=%+v,%v, want native field/base,nil", actual, queryErr)
		}
		for _, row := range []struct {
			name    string
			value   gotoolchain.SSAFieldReadObservation
			wantErr error
		}{
			{"neutral absent pair", gotoolchain.SSAFieldReadObservation{}, nil},
			{"base without field", gotoolchain.SSAFieldReadObservation{Base: actual.Base}, core.ErrGoToolchainOutput},
			{"field without base", gotoolchain.SSAFieldReadObservation{Field: actual.Field}, core.ErrGoToolchainOutput},
			{"actual field pair", actual, nil},
			{"typed nil base", gotoolchain.SSAFieldReadObservation{Base: (*ssa.Parameter)(nil), Field: actual.Field}, core.ErrGoToolchainOutput},
			{"local variable cannot be a field", gotoolchain.SSAFieldReadObservation{Base: actual.Base, Field: types.NewVar(token.NoPos, nil, "local", types.Typ[types.Int])}, core.ErrGoToolchainOutput},
		} {
			gotErr := row.value.Validate()
			if !errors.Is(gotErr, row.wantErr) {
				t.Fatalf("%s schema got=%v, want %v", row.name, gotErr, row.wantErr)
			}
		}
		for _, row := range []struct {
			name    string
			request gotoolchain.SSAFieldReadRequest
			wantErr error
		}{
			{"absent read", gotoolchain.SSAFieldReadRequest{}, core.ErrGoToolchainContract},
			{"parentless read", gotoolchain.SSAFieldReadRequest{Read: &ssa.UnOp{}}, core.ErrGoToolchainContract},
			{"actual read", gotoolchain.SSAFieldReadRequest{Read: read}, nil},
		} {
			if gotErr := row.request.Validate(); !errors.Is(gotErr, row.wantErr) {
				t.Fatalf("%s request got=%v, want %v", row.name, gotErr, row.wantErr)
			}
		}
		copied := *read
		got, queryErr := view.FieldRead(ctx, gotoolchain.SSAFieldReadRequest{Read: &copied})
		if got != (gotoolchain.SSAFieldReadObservation{}) || !errors.Is(queryErr, core.ErrGoToolchainContract) {
			t.Fatalf("detached root got=%+v,%v, want zero,%v", got, queryErr, core.ErrGoToolchainContract)
		}
		return nil
	}})
	if err != nil {
		t.Fatalf("WithSSAUnit got=%v, want nil", err)
	}
}

func TestSSAFieldReadCancellationAtEveryObservation(t *testing.T) {
	t.Parallel()
	a := nativeSymbolSourceAnalysis(t, t.TempDir(), nativeFieldReadSource)
	err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
		request := gotoolchain.SSAFieldReadRequest{Read: nativeFieldReadInstruction(t, view.Package.Func("Entry"), true)}
		parent, cancel := context.WithCancelCause(ctx)
		defer cancel(nil)
		probe := &nativeConditionalContext{Context: parent, cancel: cancel}
		got, queryErr := view.FieldRead(probe, request)
		if queryErr != nil || got.Field == nil || probe.checks < 3 {
			t.Fatalf("field context calibration got=%+v,%v checks=%d, want native loan and at least three observations", got, queryErr, probe.checks)
		}
		t.Logf("field-read context observations=%d", probe.checks)
		for at := 1; at <= probe.checks; at++ {
			parent, cancel := context.WithCancelCause(ctx)
			controlled := &nativeConditionalContext{Context: parent, cancel: cancel, failAt: at}
			got, queryErr := view.FieldRead(controlled, request)
			cancel(nil)
			if got != (gotoolchain.SSAFieldReadObservation{}) || !errors.Is(queryErr, context.Canceled) || !errors.Is(queryErr, io.ErrClosedPipe) {
				t.Fatalf("field observation=%d got=%+v,%v, want zero,cancelled with native cause", at, got, queryErr)
			}
		}
		deadline, stop := context.WithDeadlineCause(ctx, time.Time{}, io.ErrClosedPipe)
		defer stop()
		for _, row := range []struct {
			name               string
			ctx                context.Context
			wantErr, wantCause error
		}{
			{"nil", nil, core.ErrNilContext, nil}, {"expired", deadline, context.DeadlineExceeded, io.ErrClosedPipe},
		} {
			got, queryErr := view.FieldRead(row.ctx, request)
			if got != (gotoolchain.SSAFieldReadObservation{}) || !errors.Is(queryErr, row.wantErr) || row.wantCause != nil && !errors.Is(queryErr, row.wantCause) {
				t.Fatalf("%s field context got=%+v,%v, want zero,%v with cause %v", row.name, got, queryErr, row.wantErr, row.wantCause)
			}
		}
		return nil
	}})
	if err != nil {
		t.Fatalf("WithSSAUnit got=%v, want nil", err)
	}
}

// The source fixture observes cancellation four times. Keep the final receipt
// schedule independent of calibration, so deleting that check cannot move it.
func TestSSAFieldReadFinalCancellationZerosLoan(t *testing.T) {
	t.Parallel()
	a := nativeSymbolSourceAnalysis(t, t.TempDir(), nativeFieldReadSource)
	err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
		parent, cancel := context.WithCancelCause(ctx)
		defer cancel(nil)
		controlled := &nativeConditionalContext{Context: parent, cancel: cancel, failAt: 4}
		got, queryErr := view.FieldRead(controlled, gotoolchain.SSAFieldReadRequest{Read: nativeFieldReadInstruction(t, view.Package.Func("Entry"), true)})
		if got != (gotoolchain.SSAFieldReadObservation{}) || !errors.Is(queryErr, context.Canceled) || !errors.Is(queryErr, io.ErrClosedPipe) {
			t.Fatalf("final field receipt got=%+v,%v checks=%d, want zero,cancelled with native cause", got, queryErr, controlled.checks)
		}
		return nil
	}})
	if err != nil {
		t.Fatalf("WithSSAUnit got=%v, want nil", err)
	}
}

func TestSSAFieldReadAuthenticatesNeutralRootBeforeSelection(t *testing.T) {
	t.Parallel()
	a := nativeSymbolSourceAnalysis(t, t.TempDir(), nativeFieldReadSource)
	err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
		read := nativeFieldReadInstruction(t, view.Package.Func("Not"), false)
		if read.Op != token.NOT {
			t.Fatalf("neutral root opcode got=%v, want NOT", read.Op)
		}
		got, queryErr := view.FieldRead(ctx, gotoolchain.SSAFieldReadRequest{Read: read})
		if got != (gotoolchain.SSAFieldReadObservation{}) || queryErr != nil {
			t.Fatalf("native neutral root got=%+v,%v, want zero,nil", got, queryErr)
		}
		copied := *read
		got, queryErr = view.FieldRead(ctx, gotoolchain.SSAFieldReadRequest{Read: &copied})
		if got != (gotoolchain.SSAFieldReadObservation{}) || !errors.Is(queryErr, core.ErrGoToolchainContract) {
			t.Fatalf("detached neutral root got=%+v,%v, want zero,%v", got, queryErr, core.ErrGoToolchainContract)
		}
		return nil
	}})
	if err != nil {
		t.Fatalf("WithSSAUnit got=%v, want nil", err)
	}
}
