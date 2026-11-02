package temporal

import "time"

// AddMonthsClamped moves directly to the requested Gregorian month, keeping
// the original day when it exists and otherwise choosing that month's last
// day. Each call uses its own receiver as the anchor: Jan 31 + 2 months is
// March 31, whereas two successive one-month calls can end on March 28.
// This is a date-only operation. It neither chooses a timezone nor resolves
// a missing or ambiguous wall time. Results outside years 0000..9999 fail.
func (d CivilDate) AddMonthsClamped(months int32) (CivilDate, error) {
	if err := d.Validate(); err != nil {
		return CivilDate{}, err
	}
	first := time.Date(int(d.year), time.Month(d.month), 1, 0, 0, 0, 0, time.UTC).AddDate(0, int(months), 0)
	last := first.AddDate(0, 1, -1)
	day := min(int(d.day), last.Day())
	return civilDateFromNative(first.AddDate(0, 0, day-1))
}
