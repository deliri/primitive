package temporal

import (
	"errors"
	"math"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

// TestEveryAdmittedPrecisionHasACompleteFact walks the closed domain by its own
// limit rather than by a hand-written list, so a Precision constant added before
// precisionLimit without a precisionFacts row fails here instead of validating
// successfully and then truncating by a zero magnitude.
func TestEveryAdmittedPrecisionHasACompleteFact(t *testing.T) {
	t.Parallel()

	var diagnostics [precisionLimit]string
	previousMagnitude := int64(0)
	for value := PrecisionNanosecond; value < precisionLimit; value++ {
		if gotErr := value.Validate(); gotErr != nil {
			t.Fatalf("Precision(%d).Validate() error = %v, want nil for an admitted member", value, gotErr)
		}
		diagnostic := value.String()
		if diagnostic == core.UnknownEnumDiagnostic || diagnostic == "" {
			t.Fatalf(
				"Precision(%d).String() = %q, want a distinct diagnostic; its precisionFacts row is missing",
				value,
				diagnostic,
			)
		}
		for prior := PrecisionNanosecond; prior < value; prior++ {
			if diagnostic == diagnostics[prior] {
				t.Fatalf(
					"Precision(%d).String() = %q, which duplicates Precision(%d)",
					value,
					diagnostic,
					prior,
				)
			}
		}
		diagnostics[value] = diagnostic

		magnitude, gotErr := value.nanoseconds()
		if gotErr != nil {
			t.Fatalf("Precision(%d).nanoseconds() error = %v, want nil for an admitted member", value, gotErr)
		}
		// Precision is ordered coarsest-last, so each admitted member must
		// truncate to a strictly larger boundary than the one before it. A row
		// copied from its neighbour fails here rather than silently truncating
		// two precisions to the same instant.
		if magnitude <= previousMagnitude {
			t.Fatalf(
				"Precision(%d).nanoseconds() = %d, want strictly greater than Precision(%d) magnitude %d",
				value,
				magnitude,
				value-1,
				previousMagnitude,
			)
		}
		previousMagnitude = magnitude
	}
}

// TestPrecisionMagnitudesMatchTheDeclaredPackageConstants pins each row to the
// exported magnitude constant that names it, so the table cannot drift away from
// the package's published nanosecond vocabulary.
func TestPrecisionMagnitudesMatchTheDeclaredPackageConstants(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		value Precision
		want  int64
	}{
		{name: "nanosecond truncates by one", value: PrecisionNanosecond, want: 1},
		{name: "microsecond truncates by the microsecond constant", value: PrecisionMicrosecond, want: int64(NanosecondsPerMicrosecond)},
		{name: "millisecond truncates by the millisecond constant", value: PrecisionMillisecond, want: int64(NanosecondsPerMillisecond)},
		{name: "second truncates by the second constant", value: PrecisionSecond, want: int64(NanosecondsPerSecond)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := tc.value.nanoseconds()
			if gotErr != nil || got != tc.want {
				t.Fatalf("Precision(%d).nanoseconds() = (%d, %v), want (%d, nil)", tc.value, got, gotErr, tc.want)
			}
		})
	}
}

// TestPrecisionAdmissionAgreesAcrossItsWholeBackingType sweeps every uint8 and
// requires Validate, IsValid, String, and nanoseconds to agree on membership.
// Before Precision projected from one fact table these were three independent
// domain expressions, and a member admitted by the range check could still be
// refused by the magnitude switch.
func TestPrecisionAdmissionAgreesAcrossItsWholeBackingType(t *testing.T) {
	t.Parallel()

	for backing := 0; backing <= math.MaxUint8; backing++ {
		value := Precision(backing)
		admitted := value.Validate() == nil
		if got := value.IsValid(); got != admitted {
			t.Fatalf("Precision(%d).IsValid() = %t, want %t to agree with Validate", value, got, admitted)
		}
		_, magnitudeErr := value.nanoseconds()
		if (magnitudeErr == nil) != admitted {
			t.Fatalf(
				"Precision(%d).nanoseconds() error = %v, want admission %t to agree with Validate",
				value,
				magnitudeErr,
				admitted,
			)
		}
		if got := value.String() != core.UnknownEnumDiagnostic; got != admitted {
			t.Fatalf(
				"Precision(%d).String() = %q, want its non-unknown status %t to agree with Validate %t",
				value,
				value.String(),
				got,
				admitted,
			)
		}
		if !admitted && !errors.Is(value.Validate(), core.ErrTemporalContract) {
			t.Fatalf(
				"Precision(%d).Validate() error = %v, want %v",
				value,
				value.Validate(),
				core.ErrTemporalContract,
			)
		}
	}
}

// TestNegativeDurationRefusesAggregateWidening reaches past the constructors to
// build the one Duration the exported surface cannot produce. Widening used to
// map it to the zero aggregate, which turned negative time into no time at the
// boundary whose whole job is exact preservation.
func TestNegativeDurationRefusesAggregateWidening(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		nanoseconds int64
	}{
		{name: "minus one nanosecond", nanoseconds: -1},
		{name: "minimum signed nanoseconds", nanoseconds: math.MinInt64},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			negative := Duration{nanoseconds: tc.nanoseconds}
			got, gotErr := AggregateDurationFromDuration(negative)
			if !errors.Is(gotErr, core.ErrTemporalContract) || !got.IsZero() {
				t.Fatalf(
					"AggregateDurationFromDuration(%d) = (%q, %v), want (zero, %v)",
					tc.nanoseconds,
					got.Decimal(),
					gotErr,
					core.ErrTemporalContract,
				)
			}

			viaMethod, viaMethodErr := negative.Aggregate()
			if !errors.Is(viaMethodErr, core.ErrTemporalContract) || !viaMethod.IsZero() {
				t.Fatalf(
					"Duration(%d).Aggregate() = (%q, %v), want (zero, %v)",
					tc.nanoseconds,
					viaMethod.Decimal(),
					viaMethodErr,
					core.ErrTemporalContract,
				)
			}

			accumulated, accumulatedErr := AggregateDurationFromNanoseconds(7).AddDuration(negative)
			if !errors.Is(accumulatedErr, core.ErrTemporalContract) || !accumulated.IsZero() {
				t.Fatalf(
					"AddDuration(%d) = (%q, %v), want (zero, %v)",
					tc.nanoseconds,
					accumulated.Decimal(),
					accumulatedErr,
					core.ErrTemporalContract,
				)
			}
		})
	}
}
