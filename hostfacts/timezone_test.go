package hostfacts

import (
	"errors"
	"strings"
	"testing"
	"time"
	_ "time/tzdata"

	"github.com/deliri/primitive/v2026/core"
)

func TestTimeZoneObservationLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		wantErr    error
		at         time.Time
		name, zone string
		wantOffset int
	}{
		{name: "explicit UTC has no offset", zone: "UTC", at: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), wantOffset: 0, wantErr: nil},
		{name: "Toronto winter keeps standard time", zone: "America/Toronto", at: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), wantOffset: -5 * 3600, wantErr: nil},
		{name: "Toronto summer keeps daylight time", zone: "America/Toronto", at: time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC), wantOffset: -4 * 3600, wantErr: nil},
		{name: "Toronto before spring transition", zone: "America/Toronto", at: time.Date(2026, 3, 8, 6, 59, 59, 0, time.UTC), wantOffset: -5 * 3600, wantErr: nil},
		{name: "Toronto at spring transition", zone: "America/Toronto", at: time.Date(2026, 3, 8, 7, 0, 0, 0, time.UTC), wantOffset: -4 * 3600, wantErr: nil},
		{name: "Toronto before autumn transition", zone: "America/Toronto", at: time.Date(2026, 11, 1, 5, 59, 59, 0, time.UTC), wantOffset: -4 * 3600, wantErr: nil},
		{name: "Toronto at autumn transition", zone: "America/Toronto", at: time.Date(2026, 11, 1, 6, 0, 0, 0, time.UTC), wantOffset: -5 * 3600, wantErr: nil},
		{name: "fractional-hour offset is retained", zone: "Asia/Kathmandu", at: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), wantOffset: 20700, wantErr: nil},
		{name: "Etc sign follows IANA convention", zone: "Etc/GMT+12", at: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), wantOffset: -12 * 3600, wantErr: nil},
		{name: "positive date-line offset is retained", zone: "Pacific/Kiritimati", at: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), wantOffset: 14 * 3600, wantErr: nil},
		{name: "empty name cannot infer UTC", zone: "", at: time.Time{}, wantOffset: 0, wantErr: core.ErrHostFactsContract},
		{name: "Local cannot infer server timezone", zone: "Local", at: time.Time{}, wantOffset: 0, wantErr: core.ErrHostFactsContract},
		{name: "absolute path cannot load host file", zone: "/etc/passwd", at: time.Time{}, wantOffset: 0, wantErr: core.ErrHostFactsContract},
		{name: "parent traversal is refused", zone: "America/../UTC", at: time.Time{}, wantOffset: 0, wantErr: core.ErrHostFactsContract},
		{name: "empty component is refused", zone: "America//Toronto", at: time.Time{}, wantOffset: 0, wantErr: core.ErrHostFactsContract},
		{name: "trailing directory is refused", zone: "America/", at: time.Time{}, wantOffset: 0, wantErr: core.ErrHostFactsContract},
		{name: "embedded NUL is refused", zone: "UTC\x00", at: time.Time{}, wantOffset: 0, wantErr: core.ErrHostFactsContract},
		{name: "backslash path is refused", zone: `America\Toronto`, at: time.Time{}, wantOffset: 0, wantErr: core.ErrHostFactsContract},
		{name: "space does not silently trim", zone: " UTC", at: time.Time{}, wantOffset: 0, wantErr: core.ErrHostFactsContract},
		{name: "unknown named zone retains observation error", zone: "Primitive/UnknownZone", at: time.Time{}, wantOffset: 0, wantErr: core.ErrHostFactsObservation},
		{name: "one below byte ceiling reaches lookup", zone: strings.Repeat("Z", core.TimeZoneNameMaximumBytes-1), at: time.Time{}, wantOffset: 0, wantErr: core.ErrHostFactsObservation},
		{name: "exact byte ceiling reaches lookup", zone: strings.Repeat("Z", core.TimeZoneNameMaximumBytes), at: time.Time{}, wantOffset: 0, wantErr: core.ErrHostFactsObservation},
		{name: "one above byte ceiling is refused before lookup", zone: strings.Repeat("Z", core.TimeZoneNameMaximumBytes+1), at: time.Time{}, wantOffset: 0, wantErr: core.ErrHostFactsContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := ObserveTimeZone(TimeZoneRequest{Name: tc.zone})
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("ObserveTimeZone() error = %v, want %v", err, tc.wantErr)
			}
			if tc.wantErr != nil {
				failure, ok := errors.AsType[Failure](err)
				if got != nil || !ok || failure.Operation != OperationTimeZone {
					t.Fatalf("refusal = (%v,%v), want nil location and typed time-zone failure", got, err)
				}
				return
			}
			if got == nil || got.String() != tc.zone {
				t.Fatalf("location = %v, want %s", got, tc.zone)
			}
			_, offset := tc.at.In(got).Zone()
			if offset != tc.wantOffset {
				t.Fatalf("UTC offset = %d, want %d", offset, tc.wantOffset)
			}
		})
	}
}

func FuzzTimeZoneObservationSemanticClosure(f *testing.F) {
	for _, seed := range []TimeZoneRequest{{Name: "UTC"}, {Name: "America/Toronto"}, {Name: "Etc/GMT+12"}} {
		if err := seed.Validate(); err != nil {
			f.Fatalf("seed error = %v, want nil", err)
		}
		f.Add(seed.Name)
	}
	f.Add("")
	f.Add("../UTC")
	f.Add(strings.Repeat("x", core.TimeZoneNameMaximumBytes+1))
	f.Fuzz(func(t *testing.T, name string) {
		request := TimeZoneRequest{Name: name}
		got, err := ObserveTimeZone(request)
		if err != nil {
			failure, typed := errors.AsType[Failure](err)
			if got != nil || !typed || failure.Operation != OperationTimeZone || (!errors.Is(err, core.ErrHostFactsContract) && !errors.Is(err, core.ErrHostFactsObservation)) {
				t.Fatalf("refusal = (%v,%v), want nil and typed contract/observation error", got, err)
			}
			if request.Validate() != nil && !errors.Is(err, core.ErrHostFactsContract) {
				t.Fatalf("invalid request error = %v, want contract refusal", err)
			}
			if request.Validate() == nil {
				if !errors.Is(err, core.ErrHostFactsObservation) {
					t.Fatalf("lookup error = %v, want typed observation failure", err)
				}
				if want, nativeErr := time.LoadLocation(name); nativeErr == nil {
					t.Fatalf("refused native location = %v, want admitted explicit zone", want)
				}
			}
			return
		}
		if request.Validate() != nil || got == nil || got.String() != name {
			t.Fatalf("accepted location = %v, want validated exact name %q", got, name)
		}
		want, nativeErr := time.LoadLocation(name)
		if nativeErr != nil {
			t.Fatalf("native location error = %v, want nil", nativeErr)
		}
		for _, month := range []time.Month{time.January, time.July} {
			at := time.Date(2026, month, 1, 0, 0, 0, 0, time.UTC)
			gotName, gotOffset := at.In(got).Zone()
			wantName, wantOffset := at.In(want).Zone()
			if gotName != wantName || gotOffset != wantOffset || !at.In(got).Equal(at) {
				t.Fatalf("zone = (%s,%d), want native (%s,%d) and preserved instant", gotName, gotOffset, wantName, wantOffset)
			}
		}
	})
}
