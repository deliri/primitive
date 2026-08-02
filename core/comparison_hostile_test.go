package core

import (
	"errors"
	"math"
	"testing"
)

func TestComparisonExhaustsClosedDomain(t *testing.T) {
	t.Parallel()

	cases := []struct {
		wantErr        error
		name           string
		wantDiagnostic string
		value          Comparison
	}{
		{name: "unknown zero is rejected", value: ComparisonUnknown, wantDiagnostic: UnknownEnumDiagnostic, wantErr: ErrPrimitiveContract},
		{name: "less is admitted", value: ComparisonLess, wantDiagnostic: comparisonLessDiagnostic},
		{name: "equal is admitted", value: ComparisonEqual, wantDiagnostic: comparisonEqualDiagnostic},
		{name: "greater is admitted", value: ComparisonGreater, wantDiagnostic: comparisonGreaterDiagnostic},
		{name: "first future value is rejected", value: comparisonLimit, wantDiagnostic: UnknownEnumDiagnostic, wantErr: ErrPrimitiveContract},
		{name: "maximum backing value is rejected", value: Comparison(math.MaxUint8), wantDiagnostic: UnknownEnumDiagnostic, wantErr: ErrPrimitiveContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			gotErr := tc.value.Validate()
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("Comparison(%d).Validate() error = %v, want %v", tc.value, gotErr, tc.wantErr)
			}
			if got := tc.value.String(); got != tc.wantDiagnostic {
				t.Fatalf("Comparison(%d).String() = %q, want %q", tc.value, got, tc.wantDiagnostic)
			}
			if got := tc.value.IsValid(); got != (tc.wantErr == nil) {
				t.Fatalf("Comparison(%d).IsValid() = %t, want %t", tc.value, got, tc.wantErr == nil)
			}
		})
	}
}

// TestEveryAdmittedComparisonHasADistinctDiagnostic walks the closed domain by
// its own limit rather than by a hand-written list, so a Comparison constant
// added before comparisonLimit without a String case fails here instead of
// silently projecting as "unknown". TestComparisonExhaustsClosedDomain pins the
// members that exist today; this pins the members that arrive tomorrow. It
// restores, at the domain's new home, the closure ratchet that currency's
// retired Order enum carried.
func TestEveryAdmittedComparisonHasADistinctDiagnostic(t *testing.T) {
	t.Parallel()

	var diagnostics [comparisonLimit]string
	for value := ComparisonLess; value < comparisonLimit; value++ {
		if gotErr := value.Validate(); gotErr != nil {
			t.Fatalf("Comparison(%d).Validate() error = %v, want nil for an admitted member", value, gotErr)
		}
		diagnostic := value.String()
		if diagnostic == UnknownEnumDiagnostic || diagnostic == "" {
			t.Fatalf(
				"Comparison(%d).String() = %q, want a distinct diagnostic; its String case is missing",
				value,
				diagnostic,
			)
		}
		for prior := ComparisonLess; prior < value; prior++ {
			if diagnostic == diagnostics[prior] {
				t.Fatalf(
					"Comparison(%d).String() = %q, which duplicates Comparison(%d)",
					value,
					diagnostic,
					prior,
				)
			}
		}
		diagnostics[value] = diagnostic
	}
}
