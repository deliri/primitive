package temporal

import (
	"fmt"
	"slices"
	"testing"
	"time"
)

// The oracle uses native wall-date/clock projection, never ZoneBounds. Every
// case constructs the closed embedded Location, not the host timezone files.
func TestCivilEmbeddedYearBoundaryPreimages(t *testing.T) {
	t.Parallel()
	for _, zone := range []LocationName{LocationUTC, "America/New_York", "America/Toronto", "Europe/London", "Australia/Sydney"} {
		for _, year := range []int{2027, 2028, 2032, 2040} {
			t.Run(fmt.Sprintf("%s/%d", zone, year), func(t *testing.T) {
				t.Parallel()
				location := civilPreimageLocation(t, zone)
				date := civilPreimageDate(t, fmt.Sprintf("%04d-12-31", year))
				start := time.Date(year, time.December, 30, 0, 0, 0, 0, time.UTC)
				end := time.Date(year+1, time.January, 2, 0, 0, 0, 0, time.UTC)
				first, err := NewInstant(start)
				if err != nil {
					t.Fatalf("window start error=%v, want nil", err)
				}
				last, err := NewInstant(end)
				if err != nil {
					t.Fatalf("window end error=%v, want nil", err)
				}
				window := IntervalBounds{Start: first, End: last}
				t.Run("minute preimage", func(t *testing.T) {
					t.Parallel()
					got, intervalErr := location.CivilMinuteIntervals(window, CivilMinuteRange{Date: date, OpenMinute: 0, DurationMinutes: 1440})
					if intervalErr != nil {
						t.Fatalf("CivilMinuteIntervals(%s,%d) error=%v, want nil", zone, year, intervalErr)
					}
					if len(got) != 1 {
						t.Fatalf("full ordinary day preimages=%v, want one", got)
					}
					civilPreimagePointOracle(t, location, window, date, 0, 1440, got)
				})
				t.Run("date enumeration", func(t *testing.T) {
					t.Parallel()
					got, datesErr := location.CivilDates(window)
					if datesErr != nil {
						t.Fatalf("CivilDates(%s,%d) error=%v, want nil", zone, year, datesErr)
					}
					var want []CivilDate
					for instant := start; instant.Before(end); instant = instant.Add(time.Hour) {
						local := instant.In(location.value)
						d, dateErr := civilDateFromNative(local)
						if dateErr != nil {
							t.Fatalf("native date=%v error=%v, want nil", local, dateErr)
						}
						if !slices.Contains(want, d) {
							want = append(want, d)
						}
					}
					slices.SortFunc(want, func(a, b CivilDate) int { return a.native().Compare(b.native()) })
					if !slices.Equal(got, want) {
						t.Fatalf("CivilDates=%v, want native projected dates=%v", got, want)
					}
				})
			})
		}
	}
}
