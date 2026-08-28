package temporal_test

import (
	"math"
	"testing"

	"github.com/deliri/primitive/v2026/temporal"
)

func BenchmarkParseDurationCanonical(b *testing.B) {
	value, err := temporal.DurationFromNanoseconds(math.MaxInt64)
	if err != nil {
		b.Fatalf("DurationFromNanoseconds(maximum) error = %v, want nil", err)
	}
	stdlib, err := value.Stdlib()
	if err != nil {
		b.Fatalf("Duration.Stdlib(maximum) error = %v, want nil", err)
	}
	input := stdlib.String()
	b.ReportAllocs()

	for b.Loop() {
		_, _ = temporal.ParseDuration(input)
	}
}

func BenchmarkParseRFC3339Canonical(b *testing.B) {
	value := temporal.InstantFromNanoseconds(math.MaxInt64)
	input, err := value.RFC3339Nano()
	if err != nil {
		b.Fatalf("Instant.RFC3339Nano(maximum) error = %v, want nil", err)
	}
	b.ReportAllocs()

	for b.Loop() {
		_, _ = temporal.ParseRFC3339(input)
	}
}

func BenchmarkAggregateDurationDecimalMaximum(b *testing.B) {
	value, err := temporal.ParseAggregateDuration(maximumUint128Decimal)
	if err != nil {
		b.Fatalf("ParseAggregateDuration(maximum) error = %v, want nil", err)
	}
	b.ReportAllocs()

	for b.Loop() {
		_ = value.Decimal()
	}
}
