package runnercontrol

import (
	"errors"
	"math"
	"reflect"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

// Runtime heap sampling cannot prove that a future field retains an entire line.
// This structural guard pins the compiler's fixed storage shape instead.
func TestGoCoverageCompilerHasNoInputSizedStorage(t *testing.T) {
	t.Parallel()
	shape := reflect.TypeFor[GoCoverageCompiler]()
	for field := range shape.Fields() {
		if field.Name == "failure" && field.Type == reflect.TypeFor[error]() {
			continue
		}
		switch field.Type.Kind() {
		case reflect.Bool, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		case reflect.Array:
			if field.Type.Elem().Kind() != reflect.Uint8 || field.Type.Len() > 12 {
				t.Fatalf("compiler field %s = %v, want a fixed header/rune byte window", field.Name, field.Type)
			}
		default:
			t.Fatalf("compiler field %s = %v, want scalar or fixed byte window", field.Name, field.Type)
		}
	}
}

func TestGoCoverageNativeArithmeticLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name                string
		covered, statements uint64
		want                uint16
	}{
		{name: "maximum covered total remains representable", covered: math.MaxUint64, statements: math.MaxUint64, want: 10000},
		{name: "maximum uncovered total remains zero", statements: math.MaxUint64},
		{name: "half of odd maximum floors below half", covered: math.MaxUint64 / 2, statements: math.MaxUint64, want: 4999},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			compiler := GoCoverageCompiler{mode: CoverageCount, statements: tc.statements, covered: tc.covered}
			got, err := compiler.Seal()
			want := GoCoverageObservation{Mode: CoverageCount, Statements: tc.statements, Covered: tc.covered, BasisPoints: tc.want}
			if err != nil || got != want {
				t.Fatalf("Seal(native arithmetic) = %+v/%v, want %+v/nil", got, err, want)
			}
		})
	}
	t.Run("overflow refuses a record without changing totals", func(t *testing.T) {
		t.Parallel()
		compiler := GoCoverageCompiler{mode: CoverageCount, statements: math.MaxUint64, covered: math.MaxUint64}
		if err := compiler.accumulateCoverage(1, 1); !errors.Is(err, core.ErrNumericOverflow) || compiler.statements != math.MaxUint64 || compiler.covered != math.MaxUint64 {
			t.Fatalf("accumulateCoverage() = %d/%d/%v, want preserved maximum totals/%v", compiler.statements, compiler.covered, err, core.ErrNumericOverflow)
		}
	})
}
