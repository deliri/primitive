package temporal

import "time"

// LocalTimeRequest is an exact wall-clock coordinate bound to one reusable
// timezone capability. Gaps are refused; ambiguous folds follow Go's native
// time.Date choice, which does not promise which offset is selected.
type LocalTimeRequest struct {
	Location   Location
	Date       CivilDate
	Nanosecond int32
	Hour       uint8
	Minute     uint8
	Second     uint8
}

// Instant delegates timezone execution to Go and refuses normalization.
// Source: https://pkg.go.dev/time#Date
func (r LocalTimeRequest) Instant() (Instant, error) {
	value, err := r.native()
	if err != nil {
		return Instant{}, err
	}
	return NewInstant(value)
}

func (r LocalTimeRequest) Validate() error {
	_, err := r.native()
	return err
}

func (r LocalTimeRequest) native() (time.Time, error) {
	if err := r.Location.Validate(); err != nil {
		return time.Time{}, err
	}
	if err := r.Date.Validate(); err != nil {
		return time.Time{}, err
	}
	if r.Hour > 23 || r.Minute > 59 || r.Second > 59 || r.Nanosecond < 0 || r.Nanosecond >= 1000000000 {
		return time.Time{}, contractError("local clock coordinates are invalid")
	}
	value := time.Date(int(r.Date.year), time.Month(r.Date.month), int(r.Date.day), int(r.Hour), int(r.Minute), int(r.Second), int(r.Nanosecond), r.Location.value)
	year, month, day := value.Date()
	hour, minute, second := value.Clock()
	if year != int(r.Date.year) || Month(month) != r.Date.month || day != int(r.Date.day) || hour != int(r.Hour) || minute != int(r.Minute) || second != int(r.Second) || value.Nanosecond() != int(r.Nanosecond) {
		return time.Time{}, contractError("local wall clock falls in a timezone gap")
	}
	return value, nil
}

// ParseLocalTime admits canonical text under a caller layout and explicit zone.
// Zone-bearing layouts must agree with the selected zone's actual offset.
// Source: https://pkg.go.dev/time#ParseInLocation
func (l Location) ParseLocalTime(layout TimeLayout, text string) (Instant, error) {
	if err := l.Validate(); err != nil {
		return Instant{}, err
	}
	if err := layout.Validate(); err != nil {
		return Instant{}, err
	}
	value, err := time.ParseInLocation(string(layout), text, l.value)
	if err != nil {
		return Instant{}, contractError("local time text is invalid", err)
	}
	if value.In(l.value).Format(string(layout)) != text {
		return Instant{}, contractError("local time text is not canonical in its zone")
	}
	return NewInstant(value)
}

// DateAt projects a native local calendar date without timestamp string tricks.
func (l Location) DateAt(instant Instant) (CivilDate, error) {
	if err := l.Validate(); err != nil {
		return CivilDate{}, err
	}
	value, err := instant.Time()
	if err != nil {
		return CivilDate{}, err
	}
	return civilDateFromNative(value.In(l.value))
}

// StartOfDay returns the exact local midnight. A skipped midnight/date is
// refused rather than silently selecting a different day or clock.
func (l Location) StartOfDay(date CivilDate) (Instant, error) {
	return (LocalTimeRequest{Location: l, Date: date}).Instant()
}

// AddCalendar advances local calendar units, preserving the wall clock and
// refusing a resulting DST gap. It does not add a fixed 24-hour duration.
func (l Location) AddCalendar(instant Instant, delta CalendarDelta) (Instant, error) {
	if err := l.Validate(); err != nil {
		return Instant{}, err
	}
	value, err := instant.Time()
	if err != nil {
		return Instant{}, err
	}
	value = value.In(l.value)
	date, err := civilDateFromNative(value)
	if err != nil {
		return Instant{}, err
	}
	date, err = date.AddCalendar(delta)
	if err != nil {
		return Instant{}, err
	}
	return (LocalTimeRequest{Location: l, Date: date, Hour: uint8(value.Hour()), Minute: uint8(value.Minute()), Second: uint8(value.Second()), Nanosecond: int32(value.Nanosecond())}).Instant()
}
