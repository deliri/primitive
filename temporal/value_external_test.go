package temporal_test

import (
	"errors"
	"math"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

func TestInstantConstructionAndProjectionHostileBoundaries(t *testing.T) {
	t.Parallel()

	minimum := time.Unix(0, math.MinInt64).UTC()
	maximum := time.Unix(0, math.MaxInt64).UTC()
	cases := []struct {
		value   time.Time
		wantErr error
		name    string
		want    int64
	}{
		{name: "minimum signed Unix nanosecond is exact", value: minimum, want: math.MinInt64},
		{name: "minimum plus one is exact", value: minimum.Add(time.Nanosecond), want: math.MinInt64 + 1},
		{name: "one nanosecond before epoch is exact", value: time.Unix(-1, 999_999_999).UTC(), want: -1},
		{name: "epoch is a set instant", value: time.Unix(0, 0).UTC()},
		{name: "one nanosecond after epoch is exact", value: time.Unix(0, 1).UTC(), want: 1},
		{name: "non UTC location preserves the instant", value: time.Unix(0, 7).In(time.FixedZone("offset", -11*60*60)), want: 7},
		{name: "maximum minus one is exact", value: maximum.Add(-time.Nanosecond), want: math.MaxInt64 - 1},
		{name: "maximum signed Unix nanosecond is exact", value: maximum, want: math.MaxInt64},
		{name: "one below signed range is rejected", value: minimum.Add(-time.Nanosecond), wantErr: core.ErrTemporalOverflow},
		{name: "one above signed range is rejected", value: maximum.Add(time.Nanosecond), wantErr: core.ErrTemporalOverflow},
		{name: "zero time lies outside signed nanoseconds", value: time.Time{}, wantErr: core.ErrTemporalOverflow},
		{name: "far future lies outside signed nanoseconds", value: time.Date(3000, time.January, 1, 0, 0, 0, 0, time.UTC), wantErr: core.ErrTemporalOverflow},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := temporal.NewInstant(tc.value)
			if tc.wantErr != nil {
				if !errors.Is(gotErr, tc.wantErr) ||
					!errors.Is(gotErr, core.ErrTemporalContract) ||
					got.IsSet() {
					t.Fatalf(
						"NewInstant() = (%v, %v, set=%t), want zero/%v/false",
						got,
						gotErr,
						got.IsSet(),
						tc.wantErr,
					)
				}
				return
			}
			gotNanoseconds, gotNanosecondsErr := got.Nanoseconds()
			gotTime, gotTimeErr := got.Time()
			if gotErr != nil || gotNanosecondsErr != nil || gotTimeErr != nil ||
				gotNanoseconds != tc.want || !got.IsSet() {
				t.Fatalf(
					"NewInstant() projections = (ns:%d time:%v errors:%v/%v/%v set:%t), want ns:%d and no errors",
					gotNanoseconds,
					gotTime,
					gotErr,
					gotNanosecondsErr,
					gotTimeErr,
					got.IsSet(),
					tc.want,
				)
			}
			if !gotTime.Equal(time.Unix(0, tc.want).UTC()) || gotTime.Location() != time.UTC {
				t.Fatalf("Instant.Time() = %v in %v, want exact UTC Unix nanoseconds %d", gotTime, gotTime.Location(), tc.want)
			}
		})
	}
}

func TestInstantRFC3339ProjectsCanonicalUTCText(t *testing.T) {
	t.Parallel()

	instant, err := temporal.NewInstant(time.Date(2026, time.July, 31, 12, 34, 56, 987_654_321, time.FixedZone("east", 2*60*60)))
	if err != nil {
		t.Fatal(err)
	}
	got, err := instant.RFC3339()
	if err != nil || got != "2026-07-31T10:34:56Z" {
		t.Fatalf("Instant.RFC3339() = (%q, %v), want (%q, nil)", got, err, "2026-07-31T10:34:56Z")
	}
	if got, gotErr := (temporal.Instant{}).RFC3339(); got != "" || !errors.Is(gotErr, core.ErrTemporalContract) {
		t.Fatalf("zero Instant.RFC3339() = (%q, %v), want (empty, %v)", got, gotErr, core.ErrTemporalContract)
	}
}

func TestInstantArithmeticAttacksSignedEdges(t *testing.T) {
	t.Parallel()

	cases := []struct {
		wantErr     error
		name        string
		start       int64
		duration    int64
		want        int64
		subtraction bool
	}{
		{name: "negative one plus one reaches epoch", start: -1, duration: 1, want: 0},
		{name: "minimum plus zero stays minimum", start: math.MinInt64, want: math.MinInt64},
		{name: "minimum plus maximum reaches negative one", start: math.MinInt64, duration: math.MaxInt64, want: -1},
		{name: "maximum minus maximum reaches epoch", start: math.MaxInt64, duration: math.MaxInt64, want: 0, subtraction: true},
		{name: "minimum plus one is exact", start: math.MinInt64, duration: 1, want: math.MinInt64 + 1},
		{name: "maximum minus one is exact", start: math.MaxInt64, duration: 1, want: math.MaxInt64 - 1, subtraction: true},
		{name: "maximum plus one overflows", start: math.MaxInt64, duration: 1, wantErr: core.ErrTemporalOverflow},
		{name: "minimum minus one overflows", start: math.MinInt64, duration: 1, wantErr: core.ErrTemporalOverflow, subtraction: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			start := temporal.InstantFromNanoseconds(tc.start)
			duration, durationErr := temporal.DurationFromNanoseconds(tc.duration)
			if durationErr != nil {
				t.Fatalf("DurationFromNanoseconds(%d) error = %v, want nil", tc.duration, durationErr)
			}
			var got temporal.Instant
			var gotErr error
			if tc.subtraction {
				got, gotErr = start.Subtract(duration)
			} else {
				got, gotErr = start.Add(duration)
			}
			if tc.wantErr != nil {
				if !errors.Is(gotErr, tc.wantErr) ||
					!errors.Is(gotErr, core.ErrNumericOverflow) ||
					got.IsSet() {
					t.Fatalf("instant arithmetic = (%v, %v), want zero/%v", got, gotErr, tc.wantErr)
				}
				return
			}
			gotNanoseconds, gotNanosecondsErr := got.Nanoseconds()
			if gotErr != nil || gotNanosecondsErr != nil || gotNanoseconds != tc.want {
				t.Fatalf(
					"instant arithmetic = (%d, %v, %v), want (%d, nil, nil)",
					gotNanoseconds,
					gotErr,
					gotNanosecondsErr,
					tc.want,
				)
			}
		})
	}
}

func TestInstantSinceAndTruncatePressurePastAndFuture(t *testing.T) {
	t.Parallel()

	sinceCases := []struct {
		wantErr error
		name    string
		earlier int64
		later   int64
		want    int64
	}{
		{name: "same negative instant is neutral", earlier: -1, later: -1},
		{name: "crossing epoch by two nanoseconds is exact", earlier: -1, later: 1, want: 2},
		{name: "largest duration ending before epoch is exact", earlier: math.MinInt64, later: -1, want: math.MaxInt64},
		{name: "largest duration starting at epoch is exact", earlier: 0, later: math.MaxInt64, want: math.MaxInt64},
		{name: "one beyond duration range is rejected", earlier: math.MinInt64, later: 0, wantErr: core.ErrTemporalOverflow},
		{name: "full signed span is rejected", earlier: math.MinInt64, later: math.MaxInt64, wantErr: core.ErrTemporalOverflow},
		{name: "reversed instants are rejected as contract", earlier: 1, later: 0, wantErr: core.ErrTemporalContract},
	}
	for _, tc := range sinceCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := temporal.InstantFromNanoseconds(tc.later).Since(
				temporal.InstantFromNanoseconds(tc.earlier),
			)
			if tc.wantErr != nil {
				if !errors.Is(gotErr, tc.wantErr) {
					t.Fatalf("Instant.Since() error = %v, want %v", gotErr, tc.wantErr)
				}
				return
			}
			if gotErr != nil || got.Nanoseconds() != tc.want {
				t.Fatalf("Instant.Since() = (%d, %v), want (%d, nil)", got.Nanoseconds(), gotErr, tc.want)
			}
		})
	}

	truncateCases := []struct {
		wantErr   error
		name      string
		value     int64
		want      int64
		precision temporal.Precision
	}{
		{name: "nanosecond precision preserves minimum", value: math.MinInt64, precision: temporal.PrecisionNanosecond, want: math.MinInt64},
		{name: "positive partial microsecond truncates toward past", value: 1_999, precision: temporal.PrecisionMicrosecond, want: 1_000},
		{name: "negative partial microsecond truncates toward past", value: -1, precision: temporal.PrecisionMicrosecond, want: -1_000},
		{name: "negative exact microsecond is unchanged", value: -1_000, precision: temporal.PrecisionMicrosecond, want: -1_000},
		{name: "negative partial millisecond truncates toward past", value: -1_000_001, precision: temporal.PrecisionMillisecond, want: -2_000_000},
		{name: "positive partial second truncates toward past", value: 1_999_999_999, precision: temporal.PrecisionSecond, want: 1_000_000_000},
		{name: "minimum cannot reach preceding microsecond", value: math.MinInt64, precision: temporal.PrecisionMicrosecond, wantErr: core.ErrTemporalOverflow},
		{name: "unknown precision is rejected", value: 1, precision: temporal.PrecisionUnknown, wantErr: core.ErrTemporalContract},
	}
	for _, tc := range truncateCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := temporal.InstantFromNanoseconds(tc.value).Truncate(tc.precision)
			if tc.wantErr != nil {
				if !errors.Is(gotErr, tc.wantErr) {
					t.Fatalf("Instant.Truncate() error = %v, want %v", gotErr, tc.wantErr)
				}
				return
			}
			gotNanoseconds, gotNanosecondsErr := got.Nanoseconds()
			if gotErr != nil || gotNanosecondsErr != nil || gotNanoseconds != tc.want {
				t.Fatalf(
					"Instant.Truncate() = (%d, %v, %v), want (%d, nil, nil)",
					gotNanoseconds,
					gotErr,
					gotNanosecondsErr,
					tc.want,
				)
			}
		})
	}
}

func TestDurationConstructorsAndArithmeticAttackEveryMagnitude(t *testing.T) {
	t.Parallel()

	stdlibCases := []struct {
		wantErr error
		name    string
		value   time.Duration
	}{
		{name: "negative stdlib duration is rejected", value: -1, wantErr: core.ErrTemporalContract},
		{name: "zero stdlib duration is admitted"},
		{name: "one stdlib nanosecond is exact", value: 1},
		{name: "maximum stdlib duration is exact", value: time.Duration(math.MaxInt64)},
	}
	for _, tc := range stdlibCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := temporal.NewDuration(tc.value)
			if tc.wantErr != nil {
				if !errors.Is(gotErr, tc.wantErr) || got != (temporal.Duration{}) {
					t.Fatalf("NewDuration(%d) = (%v, %v), want zero/%v", tc.value, got, gotErr, tc.wantErr)
				}
				return
			}
			gotStdlib, gotStdlibErr := got.Stdlib()
			if gotErr != nil || gotStdlibErr != nil || gotStdlib != tc.value {
				t.Fatalf("NewDuration(%d) = (%d, %v, %v), want exact/nil/nil", tc.value, gotStdlib, gotErr, gotStdlibErr)
			}
		})
	}

	type constructor func(uint64) (temporal.Duration, error)
	cases := []struct {
		constructor constructor
		name        string
		magnitude   uint64
	}{
		{name: "microseconds", magnitude: temporal.NanosecondsPerMicrosecond, constructor: temporal.DurationFromMicroseconds},
		{name: "milliseconds", magnitude: temporal.NanosecondsPerMillisecond, constructor: temporal.DurationFromMilliseconds},
		{name: "seconds", magnitude: temporal.NanosecondsPerSecond, constructor: temporal.DurationFromSeconds},
		{name: "minutes", magnitude: temporal.NanosecondsPerMinute, constructor: temporal.DurationFromMinutes},
		{name: "hours", magnitude: temporal.NanosecondsPerHour, constructor: temporal.DurationFromHours},
		{name: "days", magnitude: temporal.NanosecondsPerDay, constructor: temporal.DurationFromDays},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			maximum := uint64(math.MaxInt64) / tc.magnitude
			boundaries := []struct {
				wantErr error
				name    string
				value   uint64
				want    int64
			}{
				{name: "zero remains neutral"},
				{name: "one unit is exact", value: 1, want: int64(tc.magnitude)},
				{name: "largest whole unit is exact", value: maximum, want: int64(maximum * tc.magnitude)},
				{name: "one above whole-unit maximum overflows", value: maximum + 1, wantErr: core.ErrTemporalOverflow},
				{name: "maximum unsigned input overflows", value: math.MaxUint64, wantErr: core.ErrTemporalOverflow},
			}
			for _, boundary := range boundaries {
				t.Run(boundary.name, func(t *testing.T) {
					t.Parallel()

					got, gotErr := tc.constructor(boundary.value)
					if boundary.wantErr != nil {
						if !errors.Is(gotErr, boundary.wantErr) ||
							!errors.Is(gotErr, core.ErrNumericOverflow) ||
							got != (temporal.Duration{}) {
							t.Fatalf("unit constructor = (%v, %v), want zero/%v", got, gotErr, boundary.wantErr)
						}
						return
					}
					if gotErr != nil || got.Nanoseconds() != boundary.want {
						t.Fatalf("unit constructor = (%d, %v), want (%d, nil)", got.Nanoseconds(), gotErr, boundary.want)
					}
				})
			}
		})
	}
}

func TestParseDurationLayerTriadHostileMatrix(t *testing.T) {
	t.Parallel()

	cases := []struct {
		wantErr error
		name    string
		value   string
		want    int64
	}{
		{name: "neutral bare zero is admitted", value: "0"},
		{name: "neutral zero seconds is admitted", value: "0s"},
		{name: "minimum positive nanosecond is exact", value: "1ns", want: 1},
		{name: "one below a microsecond is exact", value: "999ns", want: 999},
		{name: "ASCII microsecond is exact", value: "1us", want: 1_000},
		{name: "Unicode microsecond is exact", value: "1µs", want: 1_000},
		{name: "one millisecond is exact", value: "1ms", want: 1_000_000},
		{name: "fractional millisecond is exact", value: "1.5ms", want: 1_500_000},
		{name: "one second is exact", value: "1s", want: 1_000_000_000},
		{name: "one minute is exact", value: "1m", want: 60_000_000_000},
		{name: "one hour is exact", value: "1h", want: 3_600_000_000_000},
		{name: "one day as fixed hours is exact", value: "24h", want: 86_400_000_000_000},
		{name: "compound units preserve every nanosecond", value: "1h2m3.004005006s", want: 3_723_004_005_006},
		{name: "explicit positive sign follows Go syntax", value: "+1s", want: 1_000_000_000},
		{name: "maximum bounded duration is exact", value: "2562047h47m16.854775807s", want: math.MaxInt64},
		{name: "one below maximum bounded duration is exact", value: "2562047h47m16.854775806s", want: math.MaxInt64 - 1},

		{name: "empty text is rejected", value: "", wantErr: core.ErrTemporalContract},
		{name: "space-only text is rejected", value: " ", wantErr: core.ErrTemporalContract},
		{name: "tab-only text is rejected", value: "\t", wantErr: core.ErrTemporalContract},
		{name: "one nanosecond below zero is rejected", value: "-1ns", wantErr: core.ErrTemporalContract},
		{name: "negative second is rejected", value: "-1s", wantErr: core.ErrTemporalContract},
		{name: "unitless magnitude is rejected", value: "1", wantErr: core.ErrTemporalContract},
		{name: "unit without magnitude is rejected", value: "ns", wantErr: core.ErrTemporalContract},
		{name: "unknown unit is rejected", value: "1xs", wantErr: core.ErrTemporalContract},
		{name: "space between magnitude and unit is rejected", value: "1 s", wantErr: core.ErrTemporalContract},
		{name: "exponent syntax is rejected", value: "1e3s", wantErr: core.ErrTemporalContract},
		{name: "not-a-number is rejected", value: "NaN", wantErr: core.ErrTemporalContract},
		{name: "infinity is rejected", value: "Inf", wantErr: core.ErrTemporalContract},
		{name: "duplicated decimal point is rejected", value: "1..0s", wantErr: core.ErrTemporalContract},
		{name: "comma decimal is rejected", value: "1,5s", wantErr: core.ErrTemporalContract},
		{name: "uppercase unit is rejected", value: "1S", wantErr: core.ErrTemporalContract},
		{name: "calendar day unit is rejected", value: "1d", wantErr: core.ErrTemporalContract},
		{name: "one above maximum bounded duration is rejected", value: "2562047h47m16.854775808s", wantErr: core.ErrTemporalContract},
		{name: "negative minimum bounded magnitude is rejected by the domain", value: "-2562047h47m16.854775808s", wantErr: core.ErrTemporalContract},
		{name: "one below negative bounded range is rejected", value: "-2562047h47m16.854775809s", wantErr: core.ErrTemporalContract},
		{name: "trailing text is rejected", value: "1s trailing", wantErr: core.ErrTemporalContract},
		{name: "leading text is rejected", value: "x1s", wantErr: core.ErrTemporalContract},
		{name: "full-width digit is rejected", value: "１s", wantErr: core.ErrTemporalContract},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			got, gotErr := temporal.ParseDuration(testCase.value)
			if !errors.Is(gotErr, testCase.wantErr) {
				t.Fatalf("ParseDuration(%q) error = %v, want %v", testCase.value, gotErr, testCase.wantErr)
			}
			if testCase.wantErr != nil {
				if got != (temporal.Duration{}) {
					t.Fatalf("ParseDuration(%q) result = %v, want zero after rejection", testCase.value, got)
				}
				return
			}
			if got.Nanoseconds() != testCase.want {
				t.Fatalf("ParseDuration(%q) nanoseconds = %d, want %d", testCase.value, got.Nanoseconds(), testCase.want)
			}
		})
	}
}

func FuzzParseDurationSemanticClosure(f *testing.F) {
	for _, nanoseconds := range []int64{0, 1, 999, 1_000, 1_000_000_000, math.MaxInt64 - 1, math.MaxInt64} {
		value, err := temporal.DurationFromNanoseconds(nanoseconds)
		if err != nil {
			f.Fatalf("DurationFromNanoseconds(%d) seed error = %v, want nil", nanoseconds, err)
		}
		stdlib, err := value.Stdlib()
		if err != nil {
			f.Fatalf("Duration.Stdlib(%d) seed error = %v, want nil", nanoseconds, err)
		}
		f.Add(stdlib.String())
	}
	for _, malformed := range []string{"", "-1ns", "1", "1xs", "1e3s", "2562047h47m16.854775808s"} {
		f.Add(malformed)
	}

	f.Fuzz(func(t *testing.T, input string) {
		stdlib, stdlibErr := time.ParseDuration(input)
		wantAccepted := stdlibErr == nil && stdlib >= 0

		got, gotErr := temporal.ParseDuration(input)
		if gotErr != nil {
			if wantAccepted || !errors.Is(gotErr, core.ErrTemporalContract) || got != (temporal.Duration{}) {
				t.Fatalf(
					"ParseDuration(%q) = (%v, %v), want zero/typed refusal with standard accepted=%t",
					input,
					got,
					gotErr,
					wantAccepted,
				)
			}
			return
		}

		if !wantAccepted {
			t.Fatalf("ParseDuration(%q) = (%v, nil), want refusal because time.ParseDuration error = %v or value is negative", input, got, stdlibErr)
		}
		if gotErr := got.Validate(); gotErr != nil {
			t.Fatalf("ParseDuration(%q).Validate() error = %v, want nil", input, gotErr)
		}
		gotStdlib, projectionErr := got.Stdlib()
		if projectionErr != nil || gotStdlib != stdlib {
			t.Fatalf("ParseDuration(%q).Stdlib() = (%v, %v), want (%v, nil)", input, gotStdlib, projectionErr, stdlib)
		}
		canonical := gotStdlib.String()
		roundTrip, roundTripErr := temporal.ParseDuration(canonical)
		second, secondErr := roundTrip.Stdlib()
		if roundTripErr != nil || secondErr != nil || roundTrip != got || second.String() != canonical {
			t.Fatalf(
				"ParseDuration(%q) closure = (canonical:%q round:%v second:%v errors:%v/%v), want stable exact round trip",
				input,
				canonical,
				roundTrip,
				second,
				roundTripErr,
				secondErr,
			)
		}
	})
}

func TestPrecisionExhaustsClosedDomainAndTruncationMagnitudes(t *testing.T) {
	t.Parallel()

	cases := []struct {
		wantErr error
		name    string
		value   temporal.Precision
	}{
		{name: "unknown zero is rejected", value: temporal.PrecisionUnknown, wantErr: core.ErrTemporalContract},
		{name: "nanosecond is admitted", value: temporal.PrecisionNanosecond},
		{name: "microsecond is admitted", value: temporal.PrecisionMicrosecond},
		{name: "millisecond is admitted", value: temporal.PrecisionMillisecond},
		{name: "second is admitted", value: temporal.PrecisionSecond},
		{name: "first future precision is rejected", value: temporal.PrecisionSecond + 1, wantErr: core.ErrTemporalContract},
		{name: "maximum backing value is rejected", value: temporal.Precision(math.MaxUint8), wantErr: core.ErrTemporalContract},
	}
	unknownDiagnostic := temporal.PrecisionUnknown.String()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			gotErr := tc.value.Validate()
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("Precision(%d).Validate() error = %v, want %v", tc.value, gotErr, tc.wantErr)
			}
			if got := tc.value.IsValid(); got != (tc.wantErr == nil) {
				t.Fatalf("Precision(%d).IsValid() = %t, want %t", tc.value, got, tc.wantErr == nil)
			}
			gotDiagnostic := tc.value.String()
			if (gotDiagnostic == unknownDiagnostic) != (tc.wantErr != nil) {
				t.Fatalf(
					"Precision(%d).String() = %q, unknown diagnostic %q, want invalid=%t",
					tc.value,
					gotDiagnostic,
					unknownDiagnostic,
					tc.wantErr != nil,
				)
			}
			tc.value.OffWireEnum()
		})
	}
}

func TestDurationArithmeticAndComparisonHostileMatrix(t *testing.T) {
	t.Parallel()

	type durationOperation func(
		temporal.Duration,
		temporal.Duration,
		uint64,
	) (temporal.Duration, error)
	add := durationOperation(func(left, right temporal.Duration, _ uint64) (temporal.Duration, error) {
		return left.Add(right)
	})
	subtract := durationOperation(func(left, right temporal.Duration, _ uint64) (temporal.Duration, error) {
		return left.Subtract(right)
	})
	multiply := durationOperation(func(left, _ temporal.Duration, multiplier uint64) (temporal.Duration, error) {
		return left.Multiply(multiplier)
	})
	cases := []struct {
		wantErr    error
		operation  durationOperation
		name       string
		left       int64
		right      int64
		multiplier uint64
		want       int64
	}{
		{name: "zero plus zero is neutral", operation: add},
		{name: "adjacent values add to maximum", operation: add, left: math.MaxInt64 - 1, right: 1, want: math.MaxInt64},
		{name: "addition crosses maximum", operation: add, left: math.MaxInt64, right: 1, wantErr: core.ErrTemporalOverflow},
		{name: "maximum minus maximum reaches zero", operation: subtract, left: math.MaxInt64, right: math.MaxInt64},
		{name: "zero minus one underflows", operation: subtract, right: 1, wantErr: core.ErrTemporalOverflow},
		{name: "ordinary multiplication is exact", operation: multiply, left: 21, multiplier: 2, want: 42},
		{name: "zero times maximum remains zero", operation: multiply, multiplier: math.MaxUint64},
		{name: "one times maximum unsigned scalar overflows", operation: multiply, left: 1, multiplier: math.MaxUint64, wantErr: core.ErrTemporalOverflow},
		{name: "maximum times two overflows", operation: multiply, left: math.MaxInt64, multiplier: 2, wantErr: core.ErrTemporalOverflow},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			left, leftErr := temporal.DurationFromNanoseconds(tc.left)
			right, rightErr := temporal.DurationFromNanoseconds(tc.right)
			if leftErr != nil || rightErr != nil {
				t.Fatalf("duration construction errors = (%v, %v), want nil", leftErr, rightErr)
			}
			got, gotErr := tc.operation(left, right, tc.multiplier)
			if tc.wantErr != nil {
				if !errors.Is(gotErr, tc.wantErr) ||
					!errors.Is(gotErr, core.ErrNumericOverflow) {
					t.Fatalf("duration arithmetic error = %v, want %v and %v", gotErr, tc.wantErr, core.ErrNumericOverflow)
				}
				return
			}
			if gotErr != nil || got.Nanoseconds() != tc.want {
				t.Fatalf("duration arithmetic = (%d, %v), want (%d, nil)", got.Nanoseconds(), gotErr, tc.want)
			}
		})
	}

	orderCases := []struct {
		name  string
		left  int64
		right int64
		want  core.Comparison
	}{
		{name: "zero is less than maximum", right: math.MaxInt64, want: core.ComparisonLess},
		{name: "maximum equals maximum", left: math.MaxInt64, right: math.MaxInt64, want: core.ComparisonEqual},
		{name: "maximum is greater than adjacent", left: math.MaxInt64, right: math.MaxInt64 - 1, want: core.ComparisonGreater},
	}
	for _, tc := range orderCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			left, _ := temporal.DurationFromNanoseconds(tc.left)
			right, _ := temporal.DurationFromNanoseconds(tc.right)
			got, gotErr := left.Compare(right)
			if gotErr != nil || got != tc.want {
				t.Fatalf("Duration.Compare() = (%v, %v), want (%v, nil)", got, gotErr, tc.want)
			}
		})
	}
}
