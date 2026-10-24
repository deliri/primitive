package gotoolchain_test

import (
	"errors"
	"go/constant"
	"go/types"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gotoolchain"
	"golang.org/x/tools/go/ssa"
)

// Observation Validate checks shape. Native custody belongs to the producing
// query, so these use a genuine Go constant rather than claiming source identity.
func TestSSAScalarObservationRejectsPartialAndTypedNilLoans(t *testing.T) {
	t.Parallel()
	value := ssa.NewConst(constant.MakeInt64(1), types.Typ[types.Int])
	for _, row := range []struct {
		name        string
		left, right ssa.Value
		want        error
	}{
		{"absent pair", nil, nil, nil}, {"complete native pair", value, value, nil},
		{"left only", value, nil, core.ErrGoToolchainOutput}, {"right only", nil, value, core.ErrGoToolchainOutput},
		{"typed nil left", (*ssa.Parameter)(nil), value, core.ErrGoToolchainOutput},
		{"typed nil right", value, (*ssa.Parameter)(nil), core.ErrGoToolchainOutput},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			got := (gotoolchain.SSABinaryOperandsObservation{Left: row.left, Right: row.right}).Validate()
			if !errors.Is(got, row.want) {
				t.Fatalf("binary observation Validate got=%v, want %v", got, row.want)
			}
		})
	}
	for _, row := range []struct {
		name    string
		operand ssa.Value
		want    error
	}{
		{"absent conversion input", nil, nil}, {"native conversion input", value, nil}, {"typed nil conversion input", (*ssa.Parameter)(nil), core.ErrGoToolchainOutput},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			got := (gotoolchain.SSAConversionOperandObservation{Operand: row.operand}).Validate()
			if !errors.Is(got, row.want) {
				t.Fatalf("conversion observation Validate got=%v, want %v", got, row.want)
			}
		})
	}
}
