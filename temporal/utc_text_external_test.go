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
		name, input, canonical string
		wantErr                error
	}{
		{"epoch", "19700101T000000Z", "1970-01-01T00:00:00Z", nil},
		{"second before epoch", "19691231T235959Z", "1969-12-31T23:59:59Z", nil},
		{"second after epoch", "19700101T000001Z", "1970-01-01T00:00:01Z", nil},
		{"leap day divisible by four", "20240229T000000Z", "2024-02-29T00:00:00Z", nil},
		{"century divisible by four hundred", "20000229T000000Z", "2000-02-29T00:00:00Z", nil},
		{"month end thirty days", "20260430T235959Z", "2026-04-30T23:59:59Z", nil},
		{"month end thirty one days", "20260731T235959Z", "2026-07-31T23:59:59Z", nil},
		{"last second of year", "20261231T235959Z", "2026-12-31T23:59:59Z", nil},
		{"first representable whole second", "16770921T001244Z", "1677-09-21T00:12:44Z", nil},
		{"second above first representable", "16770921T001245Z", "1677-09-21T00:12:45Z", nil},
		{"last representable whole second", "22620411T234716Z", "2262-04-11T23:47:16Z", nil},
		{"second below last representable", "22620411T234715Z", "2262-04-11T23:47:15Z", nil},
		{"empty", "", "", core.ErrTemporalContract},
		{"one byte below exact extent", "19700101T000000", "", core.ErrTemporalContract},
		{"one byte above exact extent", "19700101T000000ZZ", "", core.ErrTemporalContract},
		{"huge representation", "19700101T000000Z" + strings.Repeat("0", temporal.CompactUTCTextBytes), "", core.ErrTemporalContract},
		{"one second before extent", "16770921T001243Z", "", core.ErrTemporalOverflow},
		{"one second after extent", "22620411T234717Z", "", core.ErrTemporalOverflow},
		{"year zero outside extent", "00000101T000000Z", "", core.ErrTemporalOverflow},
		{"year maximum outside extent", "99991231T235959Z", "", core.ErrTemporalOverflow},
		{"non leap February twenty nine", "20260229T000000Z", "", core.ErrTemporalContract},
		{"century not divisible by four hundred", "19000229T000000Z", "", core.ErrTemporalContract},
		{"month zero", "20260001T000000Z", "", core.ErrTemporalContract},
		{"month thirteen", "20261301T000000Z", "", core.ErrTemporalContract},
		{"day zero", "20260100T000000Z", "", core.ErrTemporalContract},
		{"day thirty two", "20260132T000000Z", "", core.ErrTemporalContract},
		{"day after thirty day month", "20260431T000000Z", "", core.ErrTemporalContract},
		{"hour twenty four", "20260101T240000Z", "", core.ErrTemporalContract},
		{"minute sixty", "20260101T006000Z", "", core.ErrTemporalContract},
		{"leap second is not Go calendar time", "20260101T000060Z", "", core.ErrTemporalContract},
		{"lowercase separator", "20260101t000000Z", "", core.ErrTemporalContract},
		{"lowercase zone", "20260101T000000z", "", core.ErrTemporalContract},
		{"numeric UTC offset wrong format", "20260101T000000+0000", "", core.ErrTemporalContract},
		{"RFC3339 wrong format", "2026-01-01T00:00:00Z", "", core.ErrTemporalContract},
		{"fractional second cannot be rounded", "20260101T000000.1Z", "", core.ErrTemporalContract},
		{"leading whitespace", " 20260101T000000Z", "", core.ErrTemporalContract},
		{"trailing whitespace", "20260101T000000Z ", "", core.ErrTemporalContract},
		{"embedded zero", "20260101T00\x00000Z", "", core.ErrTemporalContract},
		{"nondecimal date", "2026xx01T000000Z", "", core.ErrTemporalContract},
		{"nondecimal clock", "20260101Txx0000Z", "", core.ErrTemporalContract},
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
		name    string
		instant temporal.Instant
		want    string
		wantErr error
	}{
		{"unset", temporal.Instant{}, "", core.ErrTemporalContract},
		{"epoch", temporal.InstantFromNanoseconds(0), "19700101T000000Z", nil},
		{"one nanosecond before epoch", temporal.InstantFromNanoseconds(-1), "", core.ErrTemporalContract},
		{"one nanosecond after epoch", temporal.InstantFromNanoseconds(1), "", core.ErrTemporalContract},
		{"minimum instant has fraction", temporal.InstantFromNanoseconds(math.MinInt64), "", core.ErrTemporalContract},
		{"maximum instant has fraction", temporal.InstantFromNanoseconds(math.MaxInt64), "", core.ErrTemporalContract},
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
		name, input, canonical string
		wantErr                error
	}{
		{"Z offset", "1970-01-01T00:00:00Z", "1970-01-01T00:00:00Z", nil},
		{"positive zero offset", "1970-01-01T00:00:00+00:00", "1970-01-01T00:00:00Z", nil},
		{"negative zero offset", "1970-01-01T00:00:00-00:00", "1970-01-01T00:00:00Z", nil},
		{"one nanosecond", "1970-01-01T00:00:00.000000001Z", "1970-01-01T00:00:00.000000001Z", nil},
		{"trailing fraction zero canonicalized", "1970-01-01T00:00:00.1000Z", "1970-01-01T00:00:00.1Z", nil},
		{"minimum instant", "1677-09-21T00:12:43.145224192Z", "1677-09-21T00:12:43.145224192Z", nil},
		{"one above minimum", "1677-09-21T00:12:43.145224193Z", "1677-09-21T00:12:43.145224193Z", nil},
		{"maximum instant", "2262-04-11T23:47:16.854775807Z", "2262-04-11T23:47:16.854775807Z", nil},
		{"one below maximum", "2262-04-11T23:47:16.854775806Z", "2262-04-11T23:47:16.854775806Z", nil},
		{"maximum syntax extent zero offset", "1970-01-01T00:00:00.123456789+00:00", "1970-01-01T00:00:00.123456789Z", nil},
		{"one below minimum instant", "1677-09-21T00:12:43.145224191Z", "", core.ErrTemporalOverflow},
		{"one above maximum instant", "2262-04-11T23:47:16.854775808Z", "", core.ErrTemporalOverflow},
		{"smallest positive offset", "1970-01-01T00:00:00+00:01", "", core.ErrTemporalContract},
		{"smallest negative offset", "1970-01-01T00:00:00-00:01", "", core.ErrTemporalContract},
		{"largest positive offset", "1970-01-01T00:00:00+23:59", "", core.ErrTemporalContract},
		{"largest negative offset", "1970-01-01T00:00:00-23:59", "", core.ErrTemporalContract},
		{"offset normalized to epoch is still non UTC", "1970-01-01T01:00:00+01:00", "", core.ErrTemporalContract},
		{"empty", "", "", core.ErrTemporalContract},
		{"one below minimum text extent", "1970-01-01T00:00:00", "", core.ErrTemporalContract},
		{"one above maximum fraction digits", "1970-01-01T00:00:00.1234567890Z", "", core.ErrTemporalContract},
		{"invalid clock despite UTC suffix", "1970-01-01T25:00:00Z", "", core.ErrTemporalContract},
		{"invalid calendar despite zero suffix", "1970-02-30T00:00:00+00:00", "", core.ErrTemporalContract},
		{"whitespace after UTC", "1970-01-01T00:00:00Z ", "", core.ErrTemporalContract},
		{"junk before zero offset", "junk+00:00", "", core.ErrTemporalContract},
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
