package temporal

import (
	"errors"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
)

func TestCivilDayBoundsExactCompletePreimage(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name             string
		zone             LocationName
		date, start, end string
	}{
		{"ordinary UTC", LocationUTC, "2026-11-02", "2026-11-02T00:00:00Z", "2026-11-03T00:00:00Z"},
		{"Toronto ordinary", "America/Toronto", "2026-11-02", "2026-11-02T05:00:00Z", "2026-11-03T05:00:00Z"},
		{"Toronto short spring day", "America/Toronto", "2026-03-08", "2026-03-08T05:00:00Z", "2026-03-09T04:00:00Z"},
		{"Toronto long fall day", "America/Toronto", "2026-11-01", "2026-11-01T04:00:00Z", "2026-11-02T05:00:00Z"},
		{"Lord Howe half hour short", "Australia/Lord_Howe", "2026-10-04", "2026-10-03T13:30:00Z", "2026-10-04T13:00:00Z"},
		{"Lord Howe half hour long", "Australia/Lord_Howe", "2026-04-05", "2026-04-04T13:00:00Z", "2026-04-05T13:30:00Z"},
		{"fractional offset", "Asia/Kathmandu", "2026-11-02", "2026-11-01T18:15:00Z", "2026-11-02T18:15:00Z"},
		{"positive dateline", "Pacific/Kiritimati", "2026-11-02", "2026-11-01T10:00:00Z", "2026-11-02T10:00:00Z"},
		{"negative dateline", "Pacific/Pago_Pago", "2026-11-02", "2026-11-02T11:00:00Z", "2026-11-03T11:00:00Z"},
		{"Apia before skipped date", "Pacific/Apia", "2011-12-29", "2011-12-29T10:00:00Z", "2011-12-30T10:00:00Z"},
		{"Apia after skipped date", "Pacific/Apia", "2011-12-31", "2011-12-30T10:00:00Z", "2011-12-31T10:00:00Z"},
		{"embedded leap end", "America/New_York", "2028-12-31", "2028-12-31T05:00:00Z", "2029-01-01T05:00:00Z"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			l := civilPreimageLocation(t, tc.zone)
			d := civilPreimageDate(t, tc.date)
			want := civilPreimageBounds(t, tc.start, tc.end)
			got, err := l.CivilDayBounds(d)
			if err != nil || got != want {
				t.Fatalf("CivilDayBounds(%s,%s)=(%v,%v),want(%v,nil)", tc.zone, tc.date, got, err, want)
			}
			// Native wall-date projection independently checks every minute and the
			// immediate neighbours of both exact endpoints; no ZoneBounds in oracle.
			begin, _ := got.Start.Time()
			finish, _ := got.End.Time()
			for p := begin; p.Before(finish); p = p.Add(time.Minute) {
				actual, e := civilDateFromNative(p.In(l.value))
				if e != nil || actual != d {
					t.Fatalf("day member %v projects(%v,%v),want%v", p, actual, e, d)
				}
			}
			for _, p := range []time.Time{begin.Add(-time.Nanosecond), finish} {
				actual, e := civilDateFromNative(p.In(l.value))
				if e != nil || actual == d {
					t.Fatalf("day exterior %v projects(%v,%v),wantotherdate", p, actual, e)
				}
			}
		})
	}
}

func TestCivilDayBoundsRefusesIncompleteAuthority(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		zone LocationName
		date string
		want error
	}{
		{"skipped entire date", "Pacific/Apia", "2011-12-30", core.ErrTemporalContract},
		{"separated date preimages", "America/Goose_Bay", "1987-10-25", core.ErrTemporalContract},
		{"lower partial Instant day", LocationUTC, "1677-09-21", core.ErrTemporalOverflow},
		{"upper partial Instant day", LocationUTC, "2262-04-11", core.ErrTemporalOverflow},
		{"minimum civil profile outside Instant", LocationUTC, "0000-01-01", core.ErrTemporalOverflow},
		{"maximum civil profile outside Instant", LocationUTC, "9999-12-31", core.ErrTemporalOverflow},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			l := civilPreimageLocation(t, tc.zone)
			d := civilPreimageDate(t, tc.date)
			got, err := l.CivilDayBounds(d)
			if !errors.Is(err, tc.want) || got != (IntervalBounds{}) {
				t.Fatalf("CivilDayBounds(%s,%s)=(%v,%v),wantzero,%v", tc.zone, tc.date, got, err, tc.want)
			}
		})
	}
	t.Run("unset location", func(t *testing.T) {
		t.Parallel()
		got, err := (Location{}).CivilDayBounds(civilPreimageDate(t, "2026-11-02"))
		if !errors.Is(err, core.ErrTemporalContract) || got != (IntervalBounds{}) {
			t.Fatalf("unsetlocation=(%v,%v),wantzero,contract", got, err)
		}
	})
	t.Run("unset date", func(t *testing.T) {
		t.Parallel()
		got, err := civilPreimageLocation(t, LocationUTC).CivilDayBounds(CivilDate{})
		if !errors.Is(err, core.ErrTemporalContract) || got != (IntervalBounds{}) {
			t.Fatalf("unsetdate=(%v,%v),wantzero,contract", got, err)
		}
	})
}
