package temporal_test

import (
	"errors"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

func TestCompactUTCExactCalendarAndExtent(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		wantErr                error
		name, input, canonical string
	}{
		{name: "epoch", input: "19700101T000000Z", canonical: "1970-01-01T00:00:00Z", wantErr: nil},
		{name: "second before epoch", input: "19691231T235959Z", canonical: "1969-12-31T23:59:59Z", wantErr: nil},
		{name: "second after epoch", input: "19700101T000001Z", canonical: "1970-01-01T00:00:01Z", wantErr: nil},
		{name: "leap day divisible by four", input: "20240229T000000Z", canonical: "2024-02-29T00:00:00Z", wantErr: nil},
		{name: "century divisible by four hundred", input: "20000229T000000Z", canonical: "2000-02-29T00:00:00Z", wantErr: nil},
		{name: "month end thirty days", input: "20260430T235959Z", canonical: "2026-04-30T23:59:59Z", wantErr: nil},
		{name: "month end thirty one days", input: "20260731T235959Z", canonical: "2026-07-31T23:59:59Z", wantErr: nil},
		{name: "last second of year", input: "20261231T235959Z", canonical: "2026-12-31T23:59:59Z", wantErr: nil},
		{name: "first representable whole second", input: "16770921T001244Z", canonical: "1677-09-21T00:12:44Z", wantErr: nil},
		{name: "second above first representable", input: "16770921T001245Z", canonical: "1677-09-21T00:12:45Z", wantErr: nil},
		{name: "last representable whole second", input: "22620411T234716Z", canonical: "2262-04-11T23:47:16Z", wantErr: nil},
		{name: "second below last representable", input: "22620411T234715Z", canonical: "2262-04-11T23:47:15Z", wantErr: nil},
		{name: "empty", input: "", canonical: "", wantErr: core.ErrTemporalContract},
		{name: "one byte below exact extent", input: "19700101T000000", canonical: "", wantErr: core.ErrTemporalContract},
		{name: "one byte above exact extent", input: "19700101T000000ZZ", canonical: "", wantErr: core.ErrTemporalContract},
		{name: "huge representation", input: "19700101T000000Z" + strings.Repeat("0", temporal.CompactUTCTextBytes), canonical: "", wantErr: core.ErrTemporalContract},
		{name: "one second before extent", input: "16770921T001243Z", canonical: "", wantErr: core.ErrTemporalOverflow},
		{name: "one second after extent", input: "22620411T234717Z", canonical: "", wantErr: core.ErrTemporalOverflow},
		{name: "year zero outside extent", input: "00000101T000000Z", canonical: "", wantErr: core.ErrTemporalOverflow},
		{name: "year maximum outside extent", input: "99991231T235959Z", canonical: "", wantErr: core.ErrTemporalOverflow},
		{name: "non leap February twenty nine", input: "20260229T000000Z", canonical: "", wantErr: core.ErrTemporalContract},
		{name: "century not divisible by four hundred", input: "19000229T000000Z", canonical: "", wantErr: core.ErrTemporalContract},
		{name: "month zero", input: "20260001T000000Z", canonical: "", wantErr: core.ErrTemporalContract},
		{name: "month thirteen", input: "20261301T000000Z", canonical: "", wantErr: core.ErrTemporalContract},
		{name: "day zero", input: "20260100T000000Z", canonical: "", wantErr: core.ErrTemporalContract},
		{name: "day thirty two", input: "20260132T000000Z", canonical: "", wantErr: core.ErrTemporalContract},
		{name: "day after thirty day month", input: "20260431T000000Z", canonical: "", wantErr: core.ErrTemporalContract},
		{name: "hour twenty four", input: "20260101T240000Z", canonical: "", wantErr: core.ErrTemporalContract},
		{name: "minute sixty", input: "20260101T006000Z", canonical: "", wantErr: core.ErrTemporalContract},
		{name: "leap second is not Go calendar time", input: "20260101T000060Z", canonical: "", wantErr: core.ErrTemporalContract},
		{name: "lowercase separator", input: "20260101t000000Z", canonical: "", wantErr: core.ErrTemporalContract},
		{name: "lowercase zone", input: "20260101T000000z", canonical: "", wantErr: core.ErrTemporalContract},
		{name: "numeric UTC offset wrong format", input: "20260101T000000+0000", canonical: "", wantErr: core.ErrTemporalContract},
		{name: "RFC3339 wrong format", input: "2026-01-01T00:00:00Z", canonical: "", wantErr: core.ErrTemporalContract},
		{name: "fractional second cannot be rounded", input: "20260101T000000.1Z", canonical: "", wantErr: core.ErrTemporalContract},
		{name: "leading whitespace", input: " 20260101T000000Z", canonical: "", wantErr: core.ErrTemporalContract},
		{name: "trailing whitespace", input: "20260101T000000Z ", canonical: "", wantErr: core.ErrTemporalContract},
		{name: "embedded zero", input: "20260101T00\x00000Z", canonical: "", wantErr: core.ErrTemporalContract},
		{name: "nondecimal date", input: "2026xx01T000000Z", canonical: "", wantErr: core.ErrTemporalContract},
		{name: "nondecimal clock", input: "20260101Txx0000Z", canonical: "", wantErr: core.ErrTemporalContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := temporal.ParseCompactUTC(tc.input)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("ParseCompactUTC(%q) error = %v, want %v", tc.input, err, tc.wantErr)
			}
			if tc.wantErr != nil {
				if got != (temporal.Instant{}) || !errors.Is(err, core.ErrTemporalContract) {
					t.Fatalf("compact refusal = (%v,%v), want zero and Temporal identity", got, err)
				}
				return
			}
			canonical, canonicalErr := got.RFC3339Nano()
			compact, compactErr := got.CompactUTC()
			if got.Validate() != nil || canonicalErr != nil || compactErr != nil || canonical != tc.canonical || compact != tc.input {
				t.Fatalf("compact projections = (%q,%q,%v,%v), want (%q,%q,nil,nil)", canonical, compact, canonicalErr, compactErr, tc.canonical, tc.input)
			}
		})
	}
}

func TestCompactUTCRefusesFractionalPrecisionLoss(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		wantErr error
		name    string
		want    string
		instant temporal.Instant
	}{
		{name: "unset", instant: temporal.Instant{}, want: "", wantErr: core.ErrTemporalContract},
		{name: "epoch", instant: temporal.InstantFromNanoseconds(0), want: "19700101T000000Z", wantErr: nil},
		{name: "one nanosecond before epoch", instant: temporal.InstantFromNanoseconds(-1), want: "", wantErr: core.ErrTemporalContract},
		{name: "one nanosecond after epoch", instant: temporal.InstantFromNanoseconds(1), want: "", wantErr: core.ErrTemporalContract},
		{name: "minimum instant has fraction", instant: temporal.InstantFromNanoseconds(math.MinInt64), want: "", wantErr: core.ErrTemporalContract},
		{name: "maximum instant has fraction", instant: temporal.InstantFromNanoseconds(math.MaxInt64), want: "", wantErr: core.ErrTemporalContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := tc.instant.CompactUTC()
			if got != tc.want || !errors.Is(err, tc.wantErr) {
				t.Fatalf("CompactUTC = (%q,%v), want (%q,%v)", got, err, tc.want, tc.wantErr)
			}
		})
	}
}

func TestRFC3339UTCPreservesZeroOffsetAndExtent(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		wantErr                error
		name, input, canonical string
	}{
		{name: "Z offset", input: "1970-01-01T00:00:00Z", canonical: "1970-01-01T00:00:00Z", wantErr: nil},
		{name: "positive zero offset", input: "1970-01-01T00:00:00+00:00", canonical: "1970-01-01T00:00:00Z", wantErr: nil},
		{name: "negative zero offset", input: "1970-01-01T00:00:00-00:00", canonical: "1970-01-01T00:00:00Z", wantErr: nil},
		{name: "one nanosecond", input: "1970-01-01T00:00:00.000000001Z", canonical: "1970-01-01T00:00:00.000000001Z", wantErr: nil},
		{name: "trailing fraction zero canonicalized", input: "1970-01-01T00:00:00.1000Z", canonical: "1970-01-01T00:00:00.1Z", wantErr: nil},
		{name: "minimum instant", input: "1677-09-21T00:12:43.145224192Z", canonical: "1677-09-21T00:12:43.145224192Z", wantErr: nil},
		{name: "one above minimum", input: "1677-09-21T00:12:43.145224193Z", canonical: "1677-09-21T00:12:43.145224193Z", wantErr: nil},
		{name: "maximum instant", input: "2262-04-11T23:47:16.854775807Z", canonical: "2262-04-11T23:47:16.854775807Z", wantErr: nil},
		{name: "one below maximum", input: "2262-04-11T23:47:16.854775806Z", canonical: "2262-04-11T23:47:16.854775806Z", wantErr: nil},
		{name: "maximum syntax extent zero offset", input: "1970-01-01T00:00:00.123456789+00:00", canonical: "1970-01-01T00:00:00.123456789Z", wantErr: nil},
		{name: "one below minimum instant", input: "1677-09-21T00:12:43.145224191Z", canonical: "", wantErr: core.ErrTemporalOverflow},
		{name: "one above maximum instant", input: "2262-04-11T23:47:16.854775808Z", canonical: "", wantErr: core.ErrTemporalOverflow},
		{name: "smallest positive offset", input: "1970-01-01T00:00:00+00:01", canonical: "", wantErr: core.ErrTemporalContract},
		{name: "smallest negative offset", input: "1970-01-01T00:00:00-00:01", canonical: "", wantErr: core.ErrTemporalContract},
		{name: "largest positive offset", input: "1970-01-01T00:00:00+23:59", canonical: "", wantErr: core.ErrTemporalContract},
		{name: "largest negative offset", input: "1970-01-01T00:00:00-23:59", canonical: "", wantErr: core.ErrTemporalContract},
		{name: "offset normalized to epoch is still non UTC", input: "1970-01-01T01:00:00+01:00", canonical: "", wantErr: core.ErrTemporalContract},
		{name: "empty", input: "", canonical: "", wantErr: core.ErrTemporalContract},
		{name: "one below minimum text extent", input: "1970-01-01T00:00:00", canonical: "", wantErr: core.ErrTemporalContract},
		{name: "one above maximum fraction digits", input: "1970-01-01T00:00:00.1234567890Z", canonical: "", wantErr: core.ErrTemporalContract},
		{name: "invalid clock despite UTC suffix", input: "1970-01-01T25:00:00Z", canonical: "", wantErr: core.ErrTemporalContract},
		{name: "invalid calendar despite zero suffix", input: "1970-02-30T00:00:00+00:00", canonical: "", wantErr: core.ErrTemporalContract},
		{name: "whitespace after UTC", input: "1970-01-01T00:00:00Z ", canonical: "", wantErr: core.ErrTemporalContract},
		{name: "junk before zero offset", input: "junk+00:00", canonical: "", wantErr: core.ErrTemporalContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := temporal.ParseRFC3339UTC(tc.input)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("ParseRFC3339UTC error = %v, want %v", err, tc.wantErr)
			}
			if tc.wantErr != nil {
				if got != (temporal.Instant{}) || !errors.Is(err, core.ErrTemporalContract) {
					t.Fatalf("UTC refusal = (%v,%v), want zero typed refusal", got, err)
				}
				return
			}
			text, err := got.RFC3339Nano()
			if err != nil || got.Validate() != nil || text != tc.canonical {
				t.Fatalf("UTC canonical = (%q,%v), want %q", text, err, tc.canonical)
			}
		})
	}
}

func FuzzCompactUTCExactTime(f *testing.F) {
	for _, nanos := range []int64{0, -int64(temporal.NanosecondsPerSecond), int64(temporal.NanosecondsPerSecond)} {
		seed, err := temporal.InstantFromNanoseconds(nanos).CompactUTC()
		if err != nil {
			f.Fatalf("compact seed error = %v, want nil", err)
		}
		f.Add(seed)
	}
	f.Add("")
	f.Add("20260229T000000Z")
	f.Add("22620411T234717Z")
	f.Fuzz(func(t *testing.T, value string) {
		if len(value) != temporal.CompactUTCTextBytes {
			got, gotErr := temporal.ParseCompactUTC(value)
			if !errors.Is(gotErr, core.ErrTemporalContract) || got != (temporal.Instant{}) {
				t.Fatalf("compact extent refusal = (%v,%v), want zero typed refusal", got, gotErr)
			}
			return
		}

		// The stdlib layout is obtained from the production formatter's reference
		// instant, so the test does not maintain a second protocol layout literal.
		reference, err := temporal.NewInstant(time.Date(2006, time.January, 2, 15, 4, 5, 0, time.UTC))
		if err != nil {
			t.Fatalf("reference instant error = %v, want nil", err)
		}
		layout, err := reference.CompactUTC()
		if err != nil {
			t.Fatalf("reference layout error = %v, want nil", err)
		}
		parsed, parseErr := time.Parse(layout, value)
		want := temporal.InstantFromNanoseconds(parsed.UnixNano())
		valid := parseErr == nil && time.Unix(0, parsed.UnixNano()).Equal(parsed) && len(value) == temporal.CompactUTCTextBytes && parsed.Format(layout) == value
		got, gotErr := temporal.ParseCompactUTC(value)
		if !valid {
			if !errors.Is(gotErr, core.ErrTemporalContract) || got != (temporal.Instant{}) {
				t.Fatalf("compact refusal = (%v,%v), want zero typed refusal", got, gotErr)
			}
			return
		}
		text, textErr := got.CompactUTC()
		if gotErr != nil || textErr != nil || got != want || got.Validate() != nil || text != value {
			t.Fatalf("compact parse = (%v,%q,%v,%v), want exact stdlib instant and source text", got, text, gotErr, textErr)
		}
	})
}

func FuzzRFC3339UTCExactOffset(f *testing.F) {
	for _, nanos := range []int64{math.MinInt64, 0, math.MaxInt64} {
		seed, err := temporal.InstantFromNanoseconds(nanos).RFC3339Nano()
		if err != nil {
			f.Fatalf("UTC seed error = %v, want nil", err)
		}
		f.Add(seed)
	}
	f.Add("1970-01-01T00:00:00+00:01")
	f.Add("1970-01-01T00:00:00-00:00")
	f.Add("")
	f.Fuzz(func(t *testing.T, value string) {
		if len(value) < temporal.RFC3339MinimumTextBytes || len(value) > temporal.RFC3339MaximumTextBytes {
			got, gotErr := temporal.ParseRFC3339UTC(value)
			if !errors.Is(gotErr, core.ErrTemporalContract) || got != (temporal.Instant{}) {
				t.Fatalf("UTC extent refusal = (%v,%v), want zero typed refusal", got, gotErr)
			}
			return
		}

		raw, rawErr := time.Parse(time.RFC3339Nano, value)
		_, offset := raw.Zone()
		bounded := temporal.InstantFromNanoseconds(raw.UnixNano())
		wantValid := len(value) >= temporal.RFC3339MinimumTextBytes && len(value) <= temporal.RFC3339MaximumTextBytes && rawErr == nil && temporalRFC3339Grammar.MatchString(value) && time.Unix(0, raw.UnixNano()).Equal(raw) && offset == 0
		got, err := temporal.ParseRFC3339UTC(value)
		if !wantValid {
			if !errors.Is(err, core.ErrTemporalContract) || got != (temporal.Instant{}) {
				t.Fatalf("UTC refusal = (%v,%v), want zero typed refusal", got, err)
			}
			return
		}
		if err != nil || got.Validate() != nil || got != bounded {
			t.Fatalf("UTC parse = (%v,%v), want exact bounded stdlib UTC instant", got, err)
		}
	})
}

func BenchmarkParseCompactUTC(b *testing.B) {
	want := temporal.InstantFromNanoseconds(0)
	input, err := want.CompactUTC()
	if err != nil {
		b.Fatalf("compact fixture = %v, want nil", err)
	}
	var got temporal.Instant
	b.ReportAllocs()
	for b.Loop() {
		got, err = temporal.ParseCompactUTC(input)
		if err != nil {
			b.Fatalf("compact parse error = %v, want nil", err)
		}
	}
	if got != want {
		b.Fatalf("compact result = %v, want %v", got, want)
	}
}
func BenchmarkParseRFC3339UTC(b *testing.B) {
	want := temporal.InstantFromNanoseconds(math.MaxInt64)
	input, err := want.RFC3339Nano()
	if err != nil {
		b.Fatalf("UTC fixture = %v, want nil", err)
	}
	var got temporal.Instant
	b.ReportAllocs()
	for b.Loop() {
		got, err = temporal.ParseRFC3339UTC(input)
		if err != nil {
			b.Fatalf("UTC parse error = %v, want nil", err)
		}
	}
	if got != want {
		b.Fatalf("UTC result = %v, want %v", got, want)
	}
}
