package temporal_test

import (
	"errors"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

func TestTimeTextParseLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		layout  temporal.TimeLayout
		text    string
		refused bool
	}{
		{name: "leap date", layout: "2006-01-02", text: "2024-02-29"},
		{name: "ordinary date", layout: "2006-01-02", text: "2023-02-28"},
		{name: "release identifier", layout: "2006_01_02_15_04_05", text: "2026_10_06_23_59_59"},
		{name: "named export month", layout: "2006_Jan_02", text: "2026_Oct_06"},
		{name: "HTTP date", layout: time.RFC1123, text: "Tue, 06 Oct 2026 00:00:00 UTC"},
		{name: "RFC nanoseconds", layout: time.RFC3339Nano, text: "2026-10-06T00:00:00.123456789Z"},
		{name: "epoch date", layout: "2006-01-02", text: "1970-01-01"},
		{name: "pre epoch date", layout: "2006-01-02", text: "1900-01-01"},
		{name: "minimum represented year date", layout: "2006-01-02", text: "1678-01-01"},
		{name: "maximum represented year date", layout: "2006-01-02", text: "2262-01-01"},
		{name: "empty layout", text: "2026", refused: true},
		{name: "empty input", layout: "2006", refused: true},
		{name: "truncated release", layout: "2006_01_02_15_04_05", text: "2026_10_06", refused: true},
		{name: "trailing text", layout: "2006-01-02", text: "2026-10-06junk", refused: true},
		{name: "non leap February", layout: "2006-01-02", text: "2023-02-29", refused: true},
		{name: "April overflow", layout: "2006-01-02", text: "2026-04-31", refused: true},
		{name: "month overflow", layout: "2006-01-02", text: "2026-13-01", refused: true},
		{name: "day zero", layout: "2006-01-02", text: "2026-01-00", refused: true},
		{name: "clock overflow", layout: time.RFC3339, text: "2026-10-06T24:00:00Z", refused: true},
		{name: "noncanonical month case", layout: "2006_Jan_02", text: "2026_oct_06", refused: true},
		{name: "non UTC numeric offset", layout: time.RFC3339, text: "2026-10-06T01:00:00+01:00", refused: true},
		{name: "native lower overflow", layout: "2006-01-02", text: "1600-01-01", refused: true},
		{name: "native upper overflow", layout: "2006-01-02", text: "2300-01-01", refused: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := temporal.ParseTimeUTC(temporal.ParseTimeRequest{Layout: tc.layout, Text: tc.text})
			if tc.refused {
				if got != (temporal.Instant{}) || (!errors.Is(err, core.ErrTemporalContract) && !errors.Is(err, core.ErrTemporalOverflow)) {
					t.Fatalf("ParseTimeUTC() = (%v, %v), want zero and typed refusal", got, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseTimeUTC() error = %v, want nil", err)
			}
			native, nativeErr := time.Parse(string(tc.layout), tc.text)
			projected, projectionErr := got.Time()
			if nativeErr != nil || projectionErr != nil || !projected.Equal(native) {
				t.Fatalf("parsed instant = (%v, %v), want (%v, %v)", projected, projectionErr, native, nativeErr)
			}
		})
	}
}

func TestTimeTextFormatLayerTriad(t *testing.T) {
	t.Parallel()
	instant, err := temporal.ParseRFC3339("2026-10-06T00:00:00.123456789Z")
	if err != nil {
		t.Fatalf("seed error = %v, want nil", err)
	}
	for _, tc := range []struct {
		name    string
		request temporal.FormatTimeRequest
		want    string
		refused bool
	}{
		{name: "UTC nanoseconds", request: temporal.FormatTimeRequest{Instant: instant, Layout: time.RFC3339Nano, Location: temporal.LocationUTC}, want: "2026-10-06T00:00:00.123456789Z"},
		{name: "Toronto previous calendar day", request: temporal.FormatTimeRequest{Instant: instant, Layout: time.RFC3339, Location: "America/Toronto"}, want: "2026-10-05T20:00:00-04:00"},
		{name: "Tokyo next clock", request: temporal.FormatTimeRequest{Instant: instant, Layout: time.RFC3339, Location: "Asia/Tokyo"}, want: "2026-10-06T09:00:00+09:00"},
		{name: "UTC release format", request: temporal.FormatTimeRequest{Instant: instant, Layout: "2006_01_02_15_04_05", Location: temporal.LocationUTC}, want: "2026_10_06_00_00_00"},
		{name: "unset instant", request: temporal.FormatTimeRequest{Layout: time.RFC3339, Location: temporal.LocationUTC}, refused: true},
		{name: "empty layout", request: temporal.FormatTimeRequest{Instant: instant, Location: temporal.LocationUTC}, refused: true},
		{name: "empty location", request: temporal.FormatTimeRequest{Instant: instant, Layout: time.RFC3339}, refused: true},
		{name: "ambient location refused", request: temporal.FormatTimeRequest{Instant: instant, Layout: time.RFC3339, Location: "Local"}, refused: true},
		{name: "unknown location", request: temporal.FormatTimeRequest{Instant: instant, Layout: time.RFC3339, Location: "Not/A_Zone"}, refused: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := temporal.FormatTime(tc.request)
			if tc.refused {
				if got != "" || !errors.Is(err, core.ErrTemporalContract) {
					t.Fatalf("FormatTime() = (%q, %v), want empty and typed refusal", got, err)
				}
				return
			}
			if err != nil || got != tc.want {
				t.Fatalf("FormatTime() = (%q, %v), want (%q, nil)", got, err, tc.want)
			}
		})
	}
}

func FuzzParseTimeUTCSemanticClosure(f *testing.F) {
	seed, err := temporal.ParseRFC3339("2026-10-06T00:00:00.123456789Z")
	if err != nil {
		f.Fatalf("seed error = %v, want nil", err)
	}
	layout := temporal.TimeLayout(time.RFC3339Nano)
	text, err := temporal.FormatTime(temporal.FormatTimeRequest{Instant: seed, Layout: layout, Location: temporal.LocationUTC})
	if err != nil {
		f.Fatalf("seed formatting error = %v, want nil", err)
	}
	f.Add(string(layout), text)
	f.Add("", "")
	f.Fuzz(func(t *testing.T, layout, text string) {
		got, err := temporal.ParseTimeUTC(temporal.ParseTimeRequest{Layout: temporal.TimeLayout(layout), Text: text})
		native, nativeErr := time.Parse(layout, text)
		want, extentErr := temporal.NewInstant(native)
		accepted := layout != "" && text != "" && nativeErr == nil && extentErr == nil && native.UTC().Format(layout) == text
		if !accepted {
			if got != (temporal.Instant{}) || (!errors.Is(err, core.ErrTemporalContract) && !errors.Is(err, core.ErrTemporalOverflow)) {
				t.Fatalf("parse refusal = (%v, %v), want zero and typed refusal", got, err)
			}
			return
		}
		if err != nil || got != want {
			t.Fatalf("parsed value = (%v, %v), want (%v, nil)", got, err, want)
		}
		encoded, err := temporal.FormatTime(temporal.FormatTimeRequest{Instant: got, Layout: temporal.TimeLayout(layout), Location: temporal.LocationUTC})
		if err != nil || encoded != text {
			t.Fatalf("canonical text = (%q, %v), want (%q, nil)", encoded, err, text)
		}
	})
}

func FuzzFormatTimeSemanticClosure(f *testing.F) {
	seed, err := temporal.InstantFromUnixSeconds(1_791_244_800)
	if err != nil {
		f.Fatalf("seed error = %v, want nil", err)
	}
	nanoseconds, err := seed.Nanoseconds()
	if err != nil {
		f.Fatalf("seed projection error = %v, want nil", err)
	}
	f.Add(nanoseconds, time.RFC3339Nano, string(temporal.LocationUTC))
	f.Add(int64(0), "", "Local")
	f.Fuzz(func(t *testing.T, nanoseconds int64, layout, location string) {
		instant := temporal.InstantFromNanoseconds(nanoseconds)
		got, err := temporal.FormatTime(temporal.FormatTimeRequest{Instant: instant, Layout: temporal.TimeLayout(layout), Location: temporal.LocationName(location)})
		zone, zoneErr := time.LoadLocation(location)
		if layout == "" || location == "" || location == "Local" || zoneErr != nil {
			if got != "" || !errors.Is(err, core.ErrTemporalContract) {
				t.Fatalf("format refusal = (%q, %v), want empty and typed refusal", got, err)
			}
			return
		}
		want := time.Unix(0, nanoseconds).In(zone).Format(layout)
		if err != nil || got != want {
			t.Fatalf("formatted value = (%q, %v), want (%q, nil)", got, err, want)
		}
	})
}

func TestDurationDisplayProjectionLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name      string
		value     time.Duration
		precision temporal.Precision
	}{
		{name: "zero nanoseconds", precision: temporal.PrecisionNanosecond},
		{name: "exact nanoseconds", value: 1234567890, precision: temporal.PrecisionNanosecond},
		{name: "microsecond remainder", value: 1234567890, precision: temporal.PrecisionMicrosecond},
		{name: "millisecond remainder", value: 1234567890, precision: temporal.PrecisionMillisecond},
		{name: "second remainder", value: 1234567890, precision: temporal.PrecisionSecond},
		{name: "native maximum", value: time.Duration(temporal.DurationMaximumNanoseconds), precision: temporal.PrecisionSecond},
		{name: "unknown precision", value: time.Second},
		{name: "future precision", value: time.Second, precision: temporal.Precision(255)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			duration, err := temporal.NewDuration(tc.value)
			if err != nil {
				t.Fatalf("duration error = %v, want nil", err)
			}
			got, err := duration.Truncate(tc.precision)
			if tc.precision.Validate() != nil {
				if got != (temporal.Duration{}) || !errors.Is(err, core.ErrTemporalContract) {
					t.Fatalf("truncation = (%v, %v), want zero and typed refusal", got, err)
				}
				return
			}
			unit := [...]time.Duration{0, time.Nanosecond, time.Microsecond, time.Millisecond, time.Second}[tc.precision]
			want := tc.value.Truncate(unit)
			if err != nil || got.Nanoseconds() != int64(want) {
				t.Fatalf("truncation = (%v, %v), want (%v, nil)", got, err, want)
			}
			text, err := got.Text()
			if err != nil || text != want.String() {
				t.Fatalf("duration text = (%q, %v), want (%q, nil)", text, err, want.String())
			}
		})
	}
}
