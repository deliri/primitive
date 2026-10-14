package temporal_test

import (
	"errors"
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

func TestCivilDateNativeAdmissionHostileMatrix(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, text string
		wantErr    bool
	}{
		{"year zero leap day", "0000-02-29", false}, {"first common year", "0001-01-01", false},
		{"Gregorian date before adoption", "1582-10-04", false}, {"nonleap century February", "1900-02-28", false},
		{"four hundred year leap century", "2000-02-29", false}, {"ordinary leap year", "2024-02-29", false},
		{"last canonical date", "9999-12-31", false}, {"common February", "2023-02-28", false},
		{"thirty day month", "2026-04-30", false}, {"thirty one day month", "2026-12-31", false},
		{"empty text", "", true}, {"missing year", "02-29", true}, {"short month", "2024-2-29", true},
		{"short day", "2024-02-9", true}, {"surrounding whitespace", " 2024-02-29 ", true},
		{"trailing time", "2024-02-29T00:00:00Z", true}, {"negative year", "-001-01-01", true},
		{"explicit plus year", "+2024-01-01", true}, {"wrong separator", "2024/02/29", true}, {"truncated day", "2024-02-", true},
		{"leap boundary previous day", "2024-02-28", false}, {"leap boundary next month", "2024-03-01", false},
		{"leap February overflow", "2024-02-30", true}, {"common year missing leap day", "2023-02-29", true},
		{"common February previous day", "2023-02-27", false}, {"common February next month", "2023-03-01", false},
		{"April final minus one", "2026-04-29", false}, {"April final plus one", "2026-04-31", true},
		{"January final minus one", "2026-01-30", false}, {"January final day", "2026-01-31", false},
		{"January final plus one", "2026-01-32", true}, {"day zero", "2026-03-00", true},
		{"year upper minus one", "9998-12-31", false}, {"year above canonical width", "10000-01-01", true},
		{"month below minimum", "2026-00-01", true}, {"minimum month", "2026-01-01", false},
		{"maximum month", "2026-12-01", false}, {"month above maximum", "2026-13-01", true},
		{"leap century minus one year", "1999-02-29", true}, {"leap century plus one year", "2001-02-29", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := temporal.ParseCivilDate(tc.text)
			if tc.wantErr {
				if got != (temporal.CivilDate{}) || !errors.Is(err, core.ErrTemporalContract) {
					t.Fatalf("ParseCivilDate(%q) = (%v, %v), want zero/contract", tc.text, got, err)
				}
				return
			}
			text, textErr := got.Text()
			coordinates, coordinatesErr := got.Coordinates()
			native, nativeErr := time.Parse(time.DateOnly, tc.text)
			year, month, day := native.Date()
			if err != nil || textErr != nil || coordinatesErr != nil || nativeErr != nil || text != tc.text || coordinates.Year != int32(year) || coordinates.Month != temporal.Month(month) || coordinates.Day != uint8(day) {
				t.Fatalf("civil date = %v/%q/%v errors %v/%v/%v/%v, want exact native date %q", got, text, coordinates, err, textErr, coordinatesErr, nativeErr, tc.text)
			}
		})
	}
}

func TestCalendarEnumsExhaustTheirWireRepresentations(t *testing.T) {
	t.Parallel()
	for raw := range 256 {
		t.Run(fmt.Sprintf("representation_%d", raw), func(t *testing.T) {
			t.Parallel()
			month, weekday, boundary := temporal.Month(raw), temporal.Weekday(raw), temporal.CalendarBoundary(raw)
			for _, row := range []struct {
				name  string
				valid bool
				got   core.OffWireEnum
				text  string
			}{
				{"month", raw >= 1 && raw <= 12, month, month.String()},
				{"weekday", raw >= 1 && raw <= 7, weekday, weekday.String()},
				{"boundary", raw >= 1 && raw <= 3, boundary, boundary.String()},
			} {
				err := row.got.Validate()
				if row.valid && (err != nil || row.text == core.UnknownEnumDiagnostic) || !row.valid && (!errors.Is(err, core.ErrTemporalContract) || row.text != core.UnknownEnumDiagnostic) {
					t.Fatalf("%s(%d) = %q/%v, want valid=%t and closed identity", row.name, raw, row.text, err, row.valid)
				}
			}
		})
	}
}

func TestCivilDateArithmeticPeriodsAndAgePreserveNativeCalendar(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, start, want string
		days              int32
		wantErr           error
	}{
		{"zero days is neutral", "2024-02-29", "2024-02-29", 0, nil},
		{"leap day crossing", "2024-02-28", "2024-02-29", 1, nil},
		{"leap February crossing", "2024-02-29", "2024-03-01", 1, nil},
		{"common February crossing", "2023-02-28", "2023-03-01", 1, nil},
		{"backward year crossing", "2026-01-01", "2025-12-31", -1, nil},
		{"below year profile", "0000-01-01", "", -1, core.ErrTemporalOverflow},
		{"above year profile", "9999-12-31", "", 1, core.ErrTemporalOverflow},
		{"native huge positive increment", "2026-01-01", "", math.MaxInt32, core.ErrTemporalOverflow},
		{"native huge negative increment", "2026-01-01", "", math.MinInt32, core.ErrTemporalOverflow},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			start, err := temporal.ParseCivilDate(tc.start)
			if err != nil {
				t.Fatalf("seed = %v, want nil", err)
			}
			got, err := start.AddDays(tc.days)
			if tc.wantErr != nil {
				if got != (temporal.CivilDate{}) || !errors.Is(err, tc.wantErr) {
					t.Fatalf("AddDays = %v/%v, want zero/%v", got, err, tc.wantErr)
				}
				return
			}
			text, textErr := got.Text()
			if err != nil || textErr != nil || text != tc.want {
				t.Fatalf("AddDays = %q/%v/%v, want %q", text, err, textErr, tc.want)
			}
		})
	}
	for month := temporal.January; month <= temporal.December; month++ {
		date, err := temporal.NewCivilDate(temporal.CivilDateRequest{Year: 2026, Month: month, Day: 15})
		if err != nil {
			t.Fatalf("seed = %v", err)
		}
		for boundary := temporal.CalendarMonth; boundary <= temporal.CalendarYear; boundary++ {
			got, err := date.PeriodStart(boundary)
			coordinates, coordinateErr := got.Coordinates()
			wantMonth := month
			if boundary == temporal.CalendarQuarter {
				wantMonth = temporal.Month((int(month)-1)/3*3 + 1)
			}
			if boundary == temporal.CalendarYear {
				wantMonth = temporal.January
			}
			if err != nil || coordinateErr != nil || coordinates != (temporal.CivilDateRequest{Year: 2026, Month: wantMonth, Day: 1}) {
				t.Fatalf("PeriodStart(%v,%v) = %v/%v/%v, want first day %v", month, boundary, coordinates, err, coordinateErr, wantMonth)
			}
			end, endErr := date.PeriodEnd(boundary)
			endCoordinates, endCoordinateErr := end.Coordinates()
			next, nextErr := end.AddDays(1)
			nextStart, nextStartErr := next.PeriodStart(boundary)
			lastMonth := wantMonth
			if boundary == temporal.CalendarQuarter {
				lastMonth += 2
			}
			if boundary == temporal.CalendarYear {
				lastMonth = temporal.December
			}
			nativeEnd := time.Date(2026, time.Month(lastMonth)+1, 0, 0, 0, 0, 0, time.UTC)
			if endErr != nil || endCoordinateErr != nil || nextErr != nil || nextStartErr != nil || next != nextStart || endCoordinates != (temporal.CivilDateRequest{Year: 2026, Month: lastMonth, Day: uint8(nativeEnd.Day())}) {
				t.Fatalf("PeriodEnd(%v,%v) = %v/%v next=%v/%v errors %v/%v/%v, want native final day and next period start", month, boundary, endCoordinates, endErr, next, nextStart, endCoordinateErr, nextErr, nextStartErr)
			}
		}
	}
	for _, text := range []string{"0000-02-15", "1900-02-15", "2000-02-15", "2024-02-15", "9999-12-15"} {
		date, err := temporal.ParseCivilDate(text)
		if err != nil {
			t.Fatalf("boundary seed = %v, want nil", err)
		}
		coordinates, err := date.Coordinates()
		if err != nil {
			t.Fatalf("boundary coordinates = %v, want nil", err)
		}
		for boundary := temporal.CalendarMonth; boundary <= temporal.CalendarYear; boundary++ {
			end, err := date.PeriodEnd(boundary)
			endCoordinates, coordinateErr := end.Coordinates()
			lastMonth := coordinates.Month
			if boundary == temporal.CalendarQuarter {
				lastMonth = temporal.Month((int(lastMonth)-1)/3*3 + 3)
			}
			if boundary == temporal.CalendarYear {
				lastMonth = temporal.December
			}
			native := time.Date(int(coordinates.Year), time.Month(lastMonth)+1, 0, 0, 0, 0, 0, time.UTC)
			if err != nil || coordinateErr != nil || endCoordinates != (temporal.CivilDateRequest{Year: coordinates.Year, Month: lastMonth, Day: uint8(native.Day())}) {
				t.Fatalf("PeriodEnd(%q,%v) = %v/%v/%v, want exact native final date", text, boundary, endCoordinates, err, coordinateErr)
			}
		}
	}
	birth, err := temporal.ParseCivilDate("2000-02-29")
	if err != nil {
		t.Fatalf("birth = %v", err)
	}
	for _, row := range []struct {
		text    string
		want    int32
		wantErr error
	}{{"2000-02-29", 0, nil}, {"2001-02-28", 0, nil}, {"2001-03-01", 1, nil}, {"2024-02-28", 23, nil}, {"2024-02-29", 24, nil}, {"1999-12-31", 0, core.ErrTemporalContract}} {
		end, err := temporal.ParseCivilDate(row.text)
		if err != nil {
			t.Fatalf("end = %v", err)
		}
		got, err := birth.CompletedYearsUntil(end)
		if got != row.want || !errors.Is(err, row.wantErr) {
			t.Fatalf("CompletedYearsUntil(%q) = %d/%v, want %d/%v", row.text, got, err, row.want, row.wantErr)
		}
	}
}

func FuzzCivilDateExactNativeAdmission(f *testing.F) {
	date, err := temporal.NewCivilDate(temporal.CivilDateRequest{Year: 2024, Month: temporal.February, Day: 29})
	if err != nil {
		f.Fatal(err)
	}
	text, err := date.Text()
	if err != nil {
		f.Fatal(err)
	}
	f.Add(text)
	f.Add("")
	f.Add("2023-02-29")
	f.Fuzz(func(t *testing.T, text string) {
		got, err := temporal.ParseCivilDate(text)
		native, nativeErr := time.Parse(time.DateOnly, text)
		if nativeErr != nil || native.Format(time.DateOnly) != text {
			if got != (temporal.CivilDate{}) || !errors.Is(err, core.ErrTemporalContract) {
				t.Fatalf("date refusal = %v/%v, want zero/contract", got, err)
			}
			return
		}
		canonical, canonicalErr := got.Text()
		roundTrip, roundErr := temporal.ParseCivilDate(canonical)
		if err != nil || got.Validate() != nil || canonicalErr != nil || roundErr != nil || roundTrip != got || canonical != text {
			t.Fatalf("date admission/roundtrip = %v/%q/%v/%v/%v, want exact %q", got, canonical, err, canonicalErr, roundErr, text)
		}
	})
}

func FuzzCivilDateCoordinatesNativeAdmission(f *testing.F) {
	seed, err := temporal.NewCivilDate(temporal.CivilDateRequest{Year: 2024, Month: temporal.February, Day: 29})
	if err != nil {
		f.Fatal(err)
	}
	coordinates, err := seed.Coordinates()
	if err != nil {
		f.Fatal(err)
	}
	f.Add(coordinates.Year, uint8(coordinates.Month), coordinates.Day)
	f.Fuzz(func(t *testing.T, year int32, month, day uint8) {
		got, err := temporal.NewCivilDate(temporal.CivilDateRequest{Year: year, Month: temporal.Month(month), Day: day})
		native := time.Date(int(year), time.Month(month), int(day), 0, 0, 0, 0, time.UTC)
		y, m, d := native.Date()
		exact := year >= 0 && year <= 9999 && y == int(year) && int(m) == int(month) && d == int(day)
		if !exact {
			if got != (temporal.CivilDate{}) || !errors.Is(err, core.ErrTemporalContract) {
				t.Fatalf("coordinate refusal=%v/%v, want zero/contract", got, err)
			}
			return
		}
		text, textErr := got.Text()
		roundTrip, roundErr := temporal.ParseCivilDate(text)
		if err != nil || got.Validate() != nil || textErr != nil || roundErr != nil || roundTrip != got {
			t.Fatalf("coordinate admission=%v/%v/%v/%v, want exact native date", got, err, textErr, roundErr)
		}
	})
}
