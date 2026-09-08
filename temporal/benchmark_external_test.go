package temporal_test

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/temporal"
)

// benchmarkTimingBatch prevents very fast operations from hitting Go’s iteration
// ceiling before the requested duration. ns/value reports the per-call cost.
const benchmarkTimingBatch = 64

func BenchmarkTickerChannelBatch(b *testing.B) {
	interval, err := temporal.DurationFromNanoseconds(math.MaxInt64)
	if err != nil {
		b.Fatal(err)
	}
	ticker, err := temporal.OpenTicker(temporal.TickerRequest{Interval: interval})
	if err != nil {
		b.Fatal(err)
	}
	defer ticker.Stop()
	want := ticker.Ticks()
	b.ReportAllocs()
	for b.Loop() {
		for range benchmarkTimingBatch {
			if got := ticker.Ticks(); got == nil || got != want {
				b.Fatalf("channel = %v, want original %v", got, want)
			}
		}
	}
	b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(b.N)/benchmarkTimingBatch, "ns/value")
}

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

	var got temporal.Duration
	for b.Loop() {
		got, err = temporal.ParseDuration(input)
		if err != nil {
			b.Fatal(err)
		}
	}
	if got != value {
		b.Fatalf("duration = %v, want %v", got, value)
	}
}

func BenchmarkParseRFC3339Canonical(b *testing.B) {
	value := temporal.InstantFromNanoseconds(math.MaxInt64)
	input, err := value.RFC3339Nano()
	if err != nil {
		b.Fatalf("Instant.RFC3339Nano(maximum) error = %v, want nil", err)
	}
	b.ReportAllocs()

	var got temporal.Instant
	for b.Loop() {
		got, err = temporal.ParseRFC3339(input)
		if err != nil {
			b.Fatal(err)
		}
	}
	if got != value {
		b.Fatalf("instant = %v, want %v", got, value)
	}
}

func BenchmarkAggregateDurationDecimalMaximum(b *testing.B) {
	value, err := temporal.ParseAggregateDuration(maximumUint128Decimal)
	if err != nil {
		b.Fatalf("ParseAggregateDuration(maximum) error = %v, want nil", err)
	}
	b.ReportAllocs()

	var got string
	for b.Loop() {
		got = value.Decimal()
	}
	if got != maximumUint128Decimal {
		b.Fatalf("decimal = %q, want %q", got, maximumUint128Decimal)
	}
}

// Each JSON benchmark measures one direct persistence decode, including its
// owned validation. Fixture construction and exact result checks are untimed.
func BenchmarkInstantJSONDecodeMaximum(b *testing.B) {
	want := temporal.InstantFromNanoseconds(math.MinInt64)

	wire, err := want.MarshalJSON()
	if err != nil || len(wire) == 0 {
		b.Fatalf("fixture = (%q,%v), want nonempty JSON", wire, err)
	}
	var got temporal.Instant
	b.ReportAllocs()
	for b.Loop() {
		if err := got.UnmarshalJSON(wire); err != nil {
			b.Fatal(err)
		}
	}
	if got != want {
		b.Fatalf("decoded = %v, want %v", got, want)
	}
}
func BenchmarkDurationJSONDecodeMaximum(b *testing.B) {
	want, err := temporal.DurationFromNanoseconds(math.MaxInt64)
	if err != nil {
		b.Fatal(err)
	}
	wire, err := want.MarshalJSON()
	if err != nil || len(wire) == 0 {
		b.Fatalf("fixture = (%q,%v), want nonempty JSON", wire, err)
	}
	var got temporal.Duration
	b.ReportAllocs()
	for b.Loop() {
		if err := got.UnmarshalJSON(wire); err != nil {
			b.Fatal(err)
		}
	}
	if got != want {
		b.Fatalf("decoded = %v, want %v", got, want)
	}
}
func BenchmarkAggregateDurationJSONDecodeMaximum(b *testing.B) {
	want, err := temporal.ParseAggregateDuration(maximumUint128Decimal)
	if err != nil {
		b.Fatal(err)
	}
	wire, err := want.MarshalJSON()
	if err != nil || len(wire) == 0 {
		b.Fatalf("fixture = (%q,%v), want nonempty JSON", wire, err)
	}
	var got temporal.AggregateDuration
	b.ReportAllocs()
	for b.Loop() {
		if err := got.UnmarshalJSON(wire); err != nil {
			b.Fatal(err)
		}
	}
	if got != want {
		b.Fatalf("decoded = %v, want %v", got, want)
	}
}
func BenchmarkNumericInstantJSONDecodeMaximumBatch(b *testing.B) {
	want, err := temporal.NewNumericInstant(temporal.InstantFromNanoseconds(math.MinInt64))
	if err != nil {
		b.Fatal(err)
	}
	wire, err := want.MarshalJSON()
	if err != nil || len(wire) == 0 {
		b.Fatalf("fixture = (%q,%v), want nonempty JSON", wire, err)
	}
	var got temporal.NumericInstant
	b.ReportAllocs()
	for b.Loop() {
		for range benchmarkTimingBatch {
			if err := got.UnmarshalJSON(wire); err != nil {
				b.Fatal(err)
			}

		}
	}
	b.ReportMetric(benchmarkTimingBatch, "values/op")
	b.ReportMetric(float64(b.Elapsed().Nanoseconds())/(float64(b.N)*benchmarkTimingBatch), "ns/value")
	if got != want {
		b.Fatalf("decoded = %v, want %v", got, want)
	}
}

func BenchmarkNumericDurationJSONDecodeMaximumBatch(b *testing.B) {
	duration, err := temporal.DurationFromNanoseconds(math.MaxInt64)
	if err != nil {
		b.Fatal(err)
	}
	want, err := temporal.NewNumericDuration(duration)
	if err != nil {
		b.Fatal(err)
	}
	wire, err := want.MarshalJSON()
	if err != nil || len(wire) == 0 {
		b.Fatalf("fixture = (%q,%v), want nonempty JSON", wire, err)
	}
	var got temporal.NumericDuration
	b.ReportAllocs()
	for b.Loop() {
		for range benchmarkTimingBatch {
			if err := got.UnmarshalJSON(wire); err != nil {
				b.Fatal(err)
			}

		}
	}
	b.ReportMetric(benchmarkTimingBatch, "values/op")
	b.ReportMetric(float64(b.Elapsed().Nanoseconds())/(float64(b.N)*benchmarkTimingBatch), "ns/value")
	if got != want {
		b.Fatalf("decoded = %v, want %v", got, want)
	}
}
func BenchmarkNewInterval(b *testing.B) {
	start, err := temporal.NewObservation(time.Unix(0, -1))
	if err != nil {
		b.Fatal(err)
	}
	finish, err := temporal.NewObservation(time.Unix(0, math.MaxInt64-1))
	if err != nil {
		b.Fatal(err)
	}
	request := temporal.IntervalRequest{Start: start, Finish: finish}
	if err := request.Validate(); err != nil {
		b.Fatal(err)
	}
	var got temporal.Interval
	b.ReportAllocs()
	for b.Loop() {
		got, err = temporal.NewInterval(request)
		if err != nil {
			b.Fatal(err)
		}
	}
	bounds, boundsErr := got.Bounds()
	elapsed, elapsedErr := got.Elapsed()
	if boundsErr != nil || elapsedErr != nil || bounds.Start != temporal.InstantFromNanoseconds(-1) || bounds.End != temporal.InstantFromNanoseconds(math.MaxInt64-1) || elapsed.Nanoseconds() != math.MaxInt64 {
		b.Fatalf("interval = (%v,%v,%v,%v), want exact bounds and maximum elapsed", bounds, elapsed, boundsErr, elapsedErr)
	}
}
func BenchmarkWithTimeoutCancel(b *testing.B) {
	duration, err := temporal.DurationFromHours(1)
	if err != nil {
		b.Fatal(err)
	}
	request := temporal.TimeoutRequest{Parent: context.Background(), Duration: duration}
	if err := request.Validate(); err != nil {
		b.Fatal(err)
	}
	var got context.Context
	b.ReportAllocs()
	for b.Loop() {
		var cancel context.CancelFunc
		got, cancel, err = temporal.WithTimeout(request)
		if err != nil {
			b.Fatal(err)
		}
		cancel()
	}
	if got == nil {
		b.Fatalf("context result = %v, want a constructed context", got)
	}
	if !errors.Is(got.Err(), context.Canceled) {
		b.Fatalf("context error = %v, want cancellation", got.Err())
	}
}
func BenchmarkWaitZeroBatch(b *testing.B) {
	request := temporal.WaitRequest{Context: context.Background()}
	if err := request.Validate(); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	for b.Loop() {
		for range benchmarkTimingBatch {
			if err := temporal.Wait(request); err != nil {
				b.Fatal(err)
			}

		}
	}
	b.ReportMetric(benchmarkTimingBatch, "values/op")
	b.ReportMetric(float64(b.Elapsed().Nanoseconds())/(float64(b.N)*benchmarkTimingBatch), "ns/value")
}
func BenchmarkTickerOpenStop(b *testing.B) {
	duration, err := temporal.DurationFromHours(1)
	if err != nil {
		b.Fatal(err)
	}
	request := temporal.TickerRequest{Interval: duration}
	if err := request.Validate(); err != nil {
		b.Fatal(err)
	}
	var got *temporal.Ticker
	b.ReportAllocs()
	for b.Loop() {
		got, err = temporal.OpenTicker(request)
		if err != nil {
			b.Fatal(err)
		}
		got.Stop()
	}
	if got == nil || got.Ticks() == nil {
		b.Fatal("ticker channel = nil, want caller-owned channel")
	}
}
