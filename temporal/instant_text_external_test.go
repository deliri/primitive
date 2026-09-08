package temporal_test

import (
	"errors"
	"math"
	"regexp"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

func TestRFC3339InstantTextLayerTriadHostileMatrix(t *testing.T) {
	t.Parallel()

	cases := []struct {
		wantErr        error
		name           string
		value          string
		wantNanosecond int64
		wantParseError bool
	}{
		{name: "epoch UTC is neutral exact second", value: "1970-01-01T00:00:00Z"},
		{name: "one nanosecond after epoch is exact", value: "1970-01-01T00:00:00.000000001Z", wantNanosecond: 1},
		{name: "one nanosecond before epoch is exact", value: "1969-12-31T23:59:59.999999999Z", wantNanosecond: -1},
		{name: "positive offset projects to the same instant", value: "2026-08-20T16:37:52+02:00", wantNanosecond: 1_787_236_672_000_000_000},
		{name: "negative offset projects to the same instant", value: "2026-08-20T09:37:52-05:00", wantNanosecond: 1_787_236_672_000_000_000},
		{name: "one fractional digit is accepted", value: "2026-08-20T14:37:52.1Z", wantNanosecond: 1_787_236_672_100_000_000},
		{name: "three fractional digits are accepted", value: "2026-08-20T14:37:52.123Z", wantNanosecond: 1_787_236_672_123_000_000},
		{name: "six fractional digits are accepted", value: "2026-08-20T14:37:52.123456Z", wantNanosecond: 1_787_236_672_123_456_000},
		{name: "nine fractional digits are accepted", value: "2026-08-20T14:37:52.123456789Z", wantNanosecond: 1_787_236_672_123_456_789},
		{name: "maximum text extent is accepted", value: "2026-08-20T14:37:52.123456789+00:00", wantNanosecond: 1_787_236_672_123_456_789},
		{name: "leap day is accepted", value: "2024-02-29T23:59:59Z", wantNanosecond: 1_709_251_199_000_000_000},

		{name: "empty text is rejected before parsing", value: "", wantErr: core.ErrTemporalContract},
		{name: "space-only text is rejected before parsing", value: " ", wantErr: core.ErrTemporalContract},
		{name: "date without time is rejected before parsing", value: "2026-08-20", wantErr: core.ErrTemporalContract},
		{name: "missing zone is rejected before parsing", value: "2026-08-20T14:37:52", wantErr: core.ErrTemporalContract},
		{name: "lowercase zone is rejected", value: "2026-08-20T14:37:52z", wantErr: core.ErrTemporalContract, wantParseError: true},
		{name: "space separator is rejected", value: "2026-08-20 14:37:52Z", wantErr: core.ErrTemporalContract, wantParseError: true},
		{name: "comma fraction is rejected", value: "2026-08-20T14:37:52,1Z", wantErr: core.ErrTemporalContract},
		{name: "ten fractional digits are rejected", value: "2026-08-20T14:37:52.1234567890Z", wantErr: core.ErrTemporalContract},
		{name: "above maximum text extent is rejected before parsing", value: "2026-08-20T14:37:52.1234567890+00:00", wantErr: core.ErrTemporalContract},
		{name: "trailing text is rejected", value: "2026-08-20T14:37:52Zx", wantErr: core.ErrTemporalContract, wantParseError: true},
		{name: "leading text is rejected", value: "x2026-08-20T14:37:52Z", wantErr: core.ErrTemporalContract, wantParseError: true},

		{name: "minimum signed instant is accepted", value: "1677-09-21T00:12:43.145224192Z", wantNanosecond: math.MinInt64},
		{name: "minimum plus one nanosecond is accepted", value: "1677-09-21T00:12:43.145224193Z", wantNanosecond: math.MinInt64 + 1},
		{name: "below minimum signed instant is rejected", value: "1677-09-21T00:12:43.145224191Z", wantErr: core.ErrTemporalOverflow},
		{name: "maximum minus one nanosecond is accepted", value: "2262-04-11T23:47:16.854775806Z", wantNanosecond: math.MaxInt64 - 1},
		{name: "maximum signed instant is accepted", value: "2262-04-11T23:47:16.854775807Z", wantNanosecond: math.MaxInt64},
		{name: "above maximum signed instant is rejected", value: "2262-04-11T23:47:16.854775808Z", wantErr: core.ErrTemporalOverflow},
		{name: "month zero is rejected", value: "2026-00-20T14:37:52Z", wantErr: core.ErrTemporalContract, wantParseError: true},
		{name: "month thirteen is rejected", value: "2026-13-20T14:37:52Z", wantErr: core.ErrTemporalContract, wantParseError: true},
		{name: "day zero is rejected", value: "2026-08-00T14:37:52Z", wantErr: core.ErrTemporalContract, wantParseError: true},
		{name: "day above month range is rejected", value: "2026-04-31T14:37:52Z", wantErr: core.ErrTemporalContract, wantParseError: true},
		{name: "non leap February day is rejected", value: "2025-02-29T14:37:52Z", wantErr: core.ErrTemporalContract, wantParseError: true},
		{name: "hour twenty four is rejected", value: "2026-08-20T24:00:00Z", wantErr: core.ErrTemporalContract, wantParseError: true},
		{name: "minute sixty is rejected", value: "2026-08-20T14:60:00Z", wantErr: core.ErrTemporalContract, wantParseError: true},
		{name: "leap second is rejected", value: "2026-08-20T14:37:60Z", wantErr: core.ErrTemporalContract, wantParseError: true},
		{name: "offset hour above range is rejected", value: "2026-08-20T14:37:52+25:00", wantErr: core.ErrTemporalContract, wantParseError: true},
		{name: "offset minute above range is rejected", value: "2026-08-20T14:37:52+02:60", wantErr: core.ErrTemporalContract},
		{name: "offset without colon is rejected", value: "2026-08-20T14:37:52+0200", wantErr: core.ErrTemporalContract, wantParseError: true},
		{name: "short year is rejected before parsing", value: "026-08-20T14:37:52Z", wantErr: core.ErrTemporalContract},
		{name: "signed year is rejected", value: "+2026-08-20T14:37:52Z", wantErr: core.ErrTemporalContract, wantParseError: true},
		{name: "non ASCII digit is rejected", value: "٢٠٢٦-08-20T14:37:52Z", wantErr: core.ErrTemporalContract, wantParseError: true},
		{name: "one digit hour accepted by stdlib is rejected", value: "0000-01-10T0:00:00Z", wantErr: core.ErrTemporalContract},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := temporal.ParseRFC3339(tc.value)
			if tc.wantErr != nil {
				if !errors.Is(gotErr, tc.wantErr) || !errors.Is(gotErr, core.ErrTemporalContract) || got.IsSet() {
					t.Fatalf("ParseRFC3339(%q) = (%v, %v), want (zero, %v)", tc.value, got, gotErr, tc.wantErr)
				}
				var parseErr *time.ParseError
				if got := errors.As(gotErr, &parseErr); got != tc.wantParseError {
					t.Fatalf("ParseRFC3339(%q) exposes *time.ParseError = %t, want %t", tc.value, got, tc.wantParseError)
				}
				return
			}

			gotNanoseconds, nanosecondsErr := got.Nanoseconds()
			gotText, textErr := got.RFC3339Nano()
			if gotErr != nil || nanosecondsErr != nil || textErr != nil || gotNanoseconds != tc.wantNanosecond {
				t.Fatalf("ParseRFC3339(%q) = (ns:%d text:%q errors:%v/%v/%v), want (ns:%d, no errors)", tc.value, gotNanoseconds, gotText, gotErr, nanosecondsErr, textErr, tc.wantNanosecond)
			}
			roundTrip, roundTripErr := temporal.ParseRFC3339(gotText)
			if roundTripErr != nil || roundTrip != got {
				t.Fatalf("ParseRFC3339(RFC3339Nano()) = (%v, %v), want (%v, nil)", roundTrip, roundTripErr, got)
			}
		})
	}
}

// The regexp is an independent statement of the admitted grammar. Go owns
// calendar arithmetic; the oracle never asks Temporal whether refusal is valid.
var temporalRFC3339Grammar = regexp.MustCompile(`^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}(\.[0-9]{1,9})?(Z|[+-]([01][0-9]|2[0-3]):[0-5][0-9])$`)

func FuzzParseRFC3339SemanticClosure(f *testing.F) {
	for _, nanos := range []int64{math.MinInt64, -1, 0, 1, math.MaxInt64} {
		seed := temporal.InstantFromNanoseconds(nanos)
		if err := seed.Validate(); err != nil {
			f.Fatal(err)
		}
		wire, err := seed.RFC3339Nano()
		if err != nil {
			f.Fatal(err)
		}
		f.Add(wire)
	}
	for _, malformed := range []string{"", "2026-08-20T4:37:52Z", "2026-08-20T14:37:52,1Z", "2026-08-20T14:37:52.1234567890Z", "2026-08-20T14:37:52+24:00", "1677-09-21T00:12:43.145224191Z", "2262-04-11T23:47:16.854775808Z"} {
		f.Add(malformed)
	}
	f.Fuzz(func(t *testing.T, value string) {
		got, gotErr := temporal.ParseRFC3339(value)
		if len(value) < temporal.RFC3339MinimumTextBytes || len(value) > temporal.RFC3339MaximumTextBytes {
			if !errors.Is(gotErr, core.ErrTemporalContract) || got != (temporal.Instant{}) {
				t.Fatalf("extent refusal = (%v,%v), want zero typed refusal", got, gotErr)
			}
			return
		}
		parsed, parseErr := time.Parse(time.RFC3339Nano, value)
		syntax := parseErr == nil && temporalRFC3339Grammar.MatchString(value)
		bounded := time.Unix(0, parsed.UnixNano()).Equal(parsed)
		wantAccepted := syntax && bounded
		if !wantAccepted {
			if !errors.Is(gotErr, core.ErrTemporalContract) || got != (temporal.Instant{}) {
				t.Fatalf("parse = (%v,%v), want zero refusal for syntax=%t bounded=%t", got, gotErr, syntax, bounded)
			}
			if syntax && !errors.Is(gotErr, core.ErrTemporalOverflow) {
				t.Fatalf("range refusal = %v, want %v", gotErr, core.ErrTemporalOverflow)
			}
			if parseErr != nil {
				if _, ok := errors.AsType[*time.ParseError](gotErr); !ok {
					t.Fatalf("parse cause = %v, want *time.ParseError", gotErr)
				}
			}
			return
		}
		nanos, nanosErr := got.Nanoseconds()
		canonical, canonicalErr := got.RFC3339Nano()
		if gotErr != nil || nanosErr != nil || canonicalErr != nil || got.Validate() != nil || nanos != parsed.UnixNano() || canonical != parsed.UTC().Format(time.RFC3339Nano) {
			t.Fatalf("parse facts = (%v,%v,%v,%v,%q), want exact admitted Go time %v", nanos, gotErr, nanosErr, canonicalErr, canonical, parsed)
		}
		roundTrip, roundErr := temporal.ParseRFC3339(canonical)
		second, secondErr := roundTrip.RFC3339Nano()
		if roundErr != nil || secondErr != nil || roundTrip != got || second != canonical {
			t.Fatalf("closure = (%v,%v,%q,%v), want %v and %q", roundTrip, roundErr, second, secondErr, got, canonical)
		}
	})
}
