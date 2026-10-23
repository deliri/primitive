package gotoolchain_test

import (
	"context"
	"errors"
	"go/types"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gotoolchain"
	"golang.org/x/tools/go/ssa"
)

func TestSSATupleElementSchemaLayerTriad(t *testing.T) {
	t.Parallel()
	a := nativeSymbolSourceAnalysis(t, t.TempDir(), nativeTupleSource)
	err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
		var element *ssa.Extract
		for _, instruction := range v.Package.Func("Entry").Blocks[0].Instrs {
			if current, ok := instruction.(*ssa.Extract); ok {
				element = current
			}
		}
		if element == nil {
			t.Fatalf("schema source got=%v, want actual native extraction", element)
		}
		for _, row := range []struct {
			name    string
			in      gotoolchain.SSATupleElementObservation
			wantErr error
		}{
			{"first result", gotoolchain.SSATupleElementObservation{Tuple: element.Tuple, Index: 0}, nil},
			{"last result", gotoolchain.SSATupleElementObservation{Tuple: element.Tuple, Index: 1}, nil},
			{"empty projection", gotoolchain.SSATupleElementObservation{}, nil},
			{"negative ordinal", gotoolchain.SSATupleElementObservation{Tuple: element.Tuple, Index: -1}, core.ErrGoToolchainOutput},
			{"ordinal at width", gotoolchain.SSATupleElementObservation{Tuple: element.Tuple, Index: 2}, core.ErrGoToolchainOutput},
			{"ordinal above width", gotoolchain.SSATupleElementObservation{Tuple: element.Tuple, Index: 3}, core.ErrGoToolchainOutput},
			{"ordinal without tuple", gotoolchain.SSATupleElementObservation{Index: 1}, core.ErrGoToolchainOutput},
			{"typed nil tuple", gotoolchain.SSATupleElementObservation{Tuple: (*ssa.Call)(nil)}, core.ErrGoToolchainOutput},
			{"scalar interface tuple", gotoolchain.SSATupleElementObservation{Tuple: foreignConditionalScalar{}}, core.ErrGoToolchainOutput},
			{"nontuple type", gotoolchain.SSATupleElementObservation{Tuple: ssa.NewConst(nil, types.Typ[types.Bool])}, core.ErrGoToolchainOutput},
		} {
			queryErr := row.in.Validate()
			if row.wantErr == nil && queryErr != nil || row.wantErr != nil && !errors.Is(queryErr, row.wantErr) {
				t.Fatalf("%s schema got=%+v,%v, want %v", row.name, row.in, queryErr, row.wantErr)
			}
		}
		for _, row := range []struct {
			name    string
			in      gotoolchain.SSATupleElementRequest
			wantErr error
		}{
			{"actual extraction", gotoolchain.SSATupleElementRequest{Element: element}, nil},
			{"absent extraction", gotoolchain.SSATupleElementRequest{}, core.ErrGoToolchainContract},
			{"empty instruction", gotoolchain.SSATupleElementRequest{Element: &ssa.Extract{}}, core.ErrGoToolchainContract},
		} {
			queryErr := row.in.Validate()
			if row.wantErr == nil && queryErr != nil || row.wantErr != nil && !errors.Is(queryErr, row.wantErr) {
				t.Fatalf("%s request got=%+v,%v, want %v", row.name, row.in, queryErr, row.wantErr)
			}
		}
		return nil
	}})
	if err != nil {
		t.Fatalf("WithSSAUnit got=%v, want nil", err)
	}
}

func TestSSATupleElementPreservesAbsentAndExpiredContexts(t *testing.T) {
	t.Parallel()
	a := nativeSymbolSourceAnalysis(t, t.TempDir(), nativeTupleSource)
	err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
		var element *ssa.Extract
		for _, instruction := range v.Package.Func("Entry").Blocks[0].Instrs {
			if current, ok := instruction.(*ssa.Extract); ok {
				element = current
			}
		}
		if element == nil {
			t.Fatalf("context fixture got=%v, want actual extraction", element)
		}
		deadline, stop := context.WithDeadlineCause(ctx, time.Time{}, errors.ErrUnsupported)
		defer stop()
		for _, row := range []struct {
			name                string
			ctx                 context.Context
			wantErr, errorCause error
		}{
			{"absent", nil, core.ErrNilContext, nil},
			{"expired", deadline, context.DeadlineExceeded, errors.ErrUnsupported},
		} {
			got, queryErr := v.TupleElement(row.ctx, gotoolchain.SSATupleElementRequest{Element: element})
			if got != (gotoolchain.SSATupleElementObservation{}) || !errors.Is(queryErr, row.wantErr) || row.errorCause != nil && !errors.Is(queryErr, row.errorCause) {
				t.Fatalf("%s context got=%+v,%v, want zero,%v with cause %v", row.name, got, queryErr, row.wantErr, row.errorCause)
			}
		}
		return nil
	}})
	if err != nil {
		t.Fatalf("WithSSAUnit got=%v, want nil", err)
	}
}
