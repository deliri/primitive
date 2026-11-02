package temporal

import (
	"errors"
	"math"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func clampedDate(t testing.TB, y int32, m Month, d uint8) CivilDate {
	t.Helper()
	value, err := NewCivilDate(CivilDateRequest{Year: y, Month: m, Day: d})
	if err != nil {
		t.Fatalf("NewCivilDate(%d,%v,%d) error = %v, want nil", y, m, d, err)
	}
	return value
}

func TestCivilDateClampedMonthLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name      string
		year      int32
		month     Month
		day       uint8
		delta     int32
		wantYear  int32
		wantMonth Month
		wantDay   uint8
	}{
		{"ordinary date stays anchored", 2026, November, 2, 1, 2026, December, 2},
		{"January31 clamps to nonleap February", 2026, January, 31, 1, 2026, February, 28},
		{"January30 clamps to nonleap February", 2026, January, 30, 1, 2026, February, 28},
		{"January29 clamps to nonleap February", 2026, January, 29, 1, 2026, February, 28},
		{"January28 stays at exact February limit", 2026, January, 28, 1, 2026, February, 28},
		{"January27 stays below February limit", 2026, January, 27, 1, 2026, February, 27},
		{"leap February retains29", 2024, January, 29, 1, 2024, February, 29},
		{"leap February clamps30", 2024, January, 30, 1, 2024, February, 29},
		{"leap February clamps31", 2024, January, 31, 1, 2024, February, 29},
		{"Gregorian century is not leap", 2100, January, 31, 1, 2100, February, 28},
		{"Gregorian400 century is leap", 2000, January, 31, 1, 2000, February, 29},
		{"year zero is Gregorian leap", 0, January, 31, 1, 0, February, 29},
		{"March31 moves backward into February", 2026, March, 31, -1, 2026, February, 28},
		{"March31 moves backward into leap February", 2024, March, 31, -1, 2024, February, 29},
		{"thirty day target clamps31", 2026, March, 31, 1, 2026, April, 30},
		{"longer month keeps original30", 2026, April, 30, 1, 2026, May, 30},
		{"direct two month shift retains original31", 2026, January, 31, 2, 2026, March, 31},
		{"eighteen month horizon crosses year", 2026, August, 31, 18, 2028, February, 29},
		{"negative eighteen month horizon", 2028, August, 31, -18, 2027, February, 28},
		{"year transition forward", 2026, December, 31, 1, 2027, January, 31},
		{"year transition backward", 2026, January, 31, -1, 2025, December, 31},
		{"leap anniversary clamps", 2024, February, 29, 12, 2025, February, 28},
		{"negative leap anniversary clamps", 2024, February, 29, -12, 2023, February, 28},
		{"zero months is neutral", 2024, February, 29, 0, 2024, February, 29},
		{"minimum profile zero shift", 0, January, 1, 0, 0, January, 1},
		{"maximum profile zero shift", 9999, December, 31, 0, 9999, December, 31},
		{"exact minimum month", 1, January, 31, -12, 0, January, 31},
		{"exact maximum month", 9998, December, 31, 12, 9999, December, 31},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			date := clampedDate(t, tc.year, tc.month, tc.day)
			before := date
			got, err := date.AddMonthsClamped(tc.delta)
			want := clampedDate(t, tc.wantYear, tc.wantMonth, tc.wantDay)
			if err != nil || got != want || date != before {
				t.Fatalf("AddMonthsClamped(%d) = (%v,%v), receiver = %v, want (%v,nil) and receiver %v", tc.delta, got, err, date, want, before)
			}
		})
	}
	for _, tc := range []struct {
		name    string
		date    CivilDate
		delta   int32
		wantErr error
	}{
		{"unset receiver refuses", CivilDate{}, 0, core.ErrTemporalContract},
		{"one month below minimum", clampedDate(t, 0, January, 1), -1, core.ErrTemporalOverflow},
		{"one month above maximum", clampedDate(t, 9999, December, 31), 1, core.ErrTemporalOverflow},
		{"int32 minimum months", clampedDate(t, 2026, November, 2), math.MinInt32, core.ErrTemporalOverflow},
		{"int32 maximum months", clampedDate(t, 2026, November, 2), math.MaxInt32, core.ErrTemporalOverflow},
		{"full negative profile plus one", clampedDate(t, 9999, December, 31), -120000, core.ErrTemporalOverflow},
		{"full positive profile plus one", clampedDate(t, 0, January, 1), 120000, core.ErrTemporalOverflow},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := tc.date.AddMonthsClamped(tc.delta)
			if !errors.Is(err, tc.wantErr) || got != (CivilDate{}) {
				t.Fatalf("AddMonthsClamped(%d) = (%v,%v), want zero date and %v", tc.delta, got, err, tc.wantErr)
			}
		})
	}
}

// Independent Gregorian coordinate oracle: no time.Date/AddDate, no production
// helper, and no loop stepping normalized month ends.
func clampedMonthOracle(date CivilDateRequest, months int32) (CivilDateRequest, bool) {
	target := int64(date.Year)*12 + int64(date.Month) - 1 + int64(months)
	if target < 0 || target >= 120000 {
		return CivilDateRequest{}, false
	}
	year, month := int32(target/12), Month(target%12+1)
	days := [...]uint8{0, 31, 28, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31}
	limit := days[month]
	if month == February && year%4 == 0 && (year%100 != 0 || year%400 == 0) {
		limit++
	}
	return CivilDateRequest{Year: year, Month: month, Day: min(date.Day, limit)}, true
}

func TestCivilDateClampedMonthGregorianCycleOracle(t *testing.T) {
	t.Parallel()
	var count uint64
	for year := int32(2000); year < 2400; year++ {
		for month := January; month <= December; month++ {
			for day := uint8(1); day <= 31; day++ {
				input := CivilDateRequest{Year: year, Month: month, Day: day}
				date, err := NewCivilDate(input)
				if err != nil {
					continue // Impossible Gregorian dates are not the operation's domain.
				}
				for _, delta := range []int32{-18, -1, 0, 1, 18} {
					want, inProfile := clampedMonthOracle(input, delta)
					got, gotErr := date.AddMonthsClamped(delta)
					coordinates, coordinateErr := got.Coordinates()
					if !inProfile || gotErr != nil || coordinateErr != nil || coordinates != want {
						t.Fatalf("AddMonthsClamped(%v,%d) = (%v,%v,%v), want %v", input, delta, coordinates, gotErr, coordinateErr, want)
					}
					count++
				}
			}
		}
	}
	if count != 730485 {
		t.Fatalf("Gregorian cycle coordinate comparisons = %d, want 730485", count)
	}
}

func FuzzCivilDateClampedMonthSemanticOracle(f *testing.F) {
	for _, delta := range []int32{math.MinInt32, -120000, -18, -1, 0, 1, 18, 120000, math.MaxInt32} {
		seed := clampedDate(f, 2024, February, 29)
		coordinates, err := seed.Coordinates()
		if err != nil {
			f.Fatalf("seed.Coordinates() error = %v, want nil", err)
		}
		f.Add(coordinates.Year, uint8(coordinates.Month), coordinates.Day, delta)
	}
	f.Fuzz(func(t *testing.T, year int32, month, day uint8, delta int32) {
		request := CivilDateRequest{Year: year, Month: Month(month), Day: day}
		date, err := NewCivilDate(request)
		if err != nil {
			if !errors.Is(err, core.ErrTemporalContract) || date != (CivilDate{}) {
				t.Fatalf("NewCivilDate(%v) = (%v,%v), want zero and temporal contract", request, date, err)
			}
			return
		}
		want, inProfile := clampedMonthOracle(request, delta)
		got, gotErr := date.AddMonthsClamped(delta)
		if !inProfile {
			if !errors.Is(gotErr, core.ErrTemporalOverflow) || got != (CivilDate{}) {
				t.Fatalf("AddMonthsClamped(%v,%d) = (%v,%v), want zero and overflow", request, delta, got, gotErr)
			}
			return
		}
		coordinates, coordinateErr := got.Coordinates()
		if gotErr != nil || coordinateErr != nil || coordinates != want {
			t.Fatalf("AddMonthsClamped(%v,%d) = (%v,%v,%v), want %v", request, delta, coordinates, gotErr, coordinateErr, want)
		}
	})
}
