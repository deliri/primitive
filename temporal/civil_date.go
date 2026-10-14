package temporal

import (
	"time"

	"github.com/deliri/primitive/v2026/core"
)

// CivilDateRequest supplies a proleptic Gregorian date in the four-digit
// ISO date profile (year 0000 through 9999). It has no timezone or clock.
type CivilDateRequest struct {
	Year  int32
	Month Month
	Day   uint8
}

// CivilDate is one sealed calendar date. Its zero value is invalid.
type CivilDate struct {
	year  int32
	month Month
	day   uint8
}

// NewCivilDate refuses impossible dates instead of time.Date normalization.
// Source: https://pkg.go.dev/time#Date
func NewCivilDate(request CivilDateRequest) (CivilDate, error) {
	date := CivilDate{year: request.Year, month: request.Month, day: request.Day}
	if err := date.Validate(); err != nil {
		return CivilDate{}, err
	}
	return date, nil
}

// Validate proves both Gregorian coordinates and the canonical date profile.
func (d CivilDate) Validate() error {
	if d.year < 0 || d.year > 9999 || d.month.Validate() != nil || d.day == 0 || d.day > 31 {
		return contractError("civil date coordinates are invalid")
	}
	value := d.native()
	year, month, day := value.Date()
	if year != int(d.year) || Month(month) != d.month || day != int(d.day) {
		return contractError("civil date is not Gregorian")
	}
	return nil
}

func (d CivilDate) native() time.Time {
	return time.Date(int(d.year), time.Month(d.month), int(d.day), 0, 0, 0, 0, time.UTC)
}

// Coordinates projects exact validated coordinates.
func (d CivilDate) Coordinates() (CivilDateRequest, error) {
	if err := d.Validate(); err != nil {
		return CivilDateRequest{}, err
	}
	return CivilDateRequest{Year: d.year, Month: d.month, Day: d.day}, nil
}

// ParseCivilDate admits only canonical ISO date-only text.
// Source: https://pkg.go.dev/time#Parse
func ParseCivilDate(text string) (CivilDate, error) {
	value, err := time.Parse(time.DateOnly, text)
	if err != nil {
		return CivilDate{}, contractError("civil date text is invalid", err)
	}
	if value.Format(time.DateOnly) != text {
		return CivilDate{}, contractError("civil date text is not canonical")
	}
	return civilDateFromNative(value)
}

func civilDateFromNative(value time.Time) (CivilDate, error) {
	year, month, day := value.Date()
	if year < 0 || year > 9999 {
		return CivilDate{}, overflowError("civil date exceeds four-digit year profile")
	}
	return NewCivilDate(CivilDateRequest{Year: int32(year), Month: Month(month), Day: uint8(day)})
}

// Text returns canonical ISO date-only text.
func (d CivilDate) Text() (string, error) {
	if err := d.Validate(); err != nil {
		return "", err
	}
	return d.native().Format(time.DateOnly), nil
}

// AddDays advances calendar dates, independent of elapsed hours or DST.
func (d CivilDate) AddDays(days int32) (CivilDate, error) {
	if err := d.Validate(); err != nil {
		return CivilDate{}, err
	}
	return civilDateFromNative(d.native().AddDate(0, 0, int(days)))
}

// AddCalendar delegates Go's documented calendar normalization semantics.
func (d CivilDate) AddCalendar(delta CalendarDelta) (CivilDate, error) {
	if err := d.Validate(); err != nil {
		return CivilDate{}, err
	}
	return civilDateFromNative(d.native().AddDate(int(delta.Years), int(delta.Months), int(delta.Days)))
}

// CalendarBoundary is the closed set of civil period starts.
type CalendarBoundary uint8

const (
	CalendarBoundaryUnknown CalendarBoundary = iota
	CalendarMonth
	CalendarQuarter
	CalendarYear
)

func (b CalendarBoundary) Validate() error {
	if b < CalendarMonth || b > CalendarYear {
		return contractError("calendar boundary is invalid")
	}
	return nil
}
func (b CalendarBoundary) IsValid() bool { return b.Validate() == nil }
func (CalendarBoundary) OffWireEnum()    {}
func (b CalendarBoundary) String() string {
	switch b {
	case CalendarMonth:
		return "month"
	case CalendarQuarter:
		return "quarter"
	case CalendarYear:
		return "year"
	default:
		return core.UnknownEnumDiagnostic
	}
}

// PeriodStart returns the first date of the containing month, quarter or year.
func (d CivilDate) PeriodStart(boundary CalendarBoundary) (CivilDate, error) {
	if err := d.Validate(); err != nil {
		return CivilDate{}, err
	}
	if err := boundary.Validate(); err != nil {
		return CivilDate{}, err
	}
	month := d.month
	switch boundary {
	case CalendarMonth:
	case CalendarQuarter:
		month = Month((int(month)-1)/3*3 + 1)
	case CalendarYear:
		month = January
	default:
		return CivilDate{}, core.ErrTemporalContract
	}
	return NewCivilDate(CivilDateRequest{Year: d.year, Month: month, Day: 1})
}

// PeriodEnd returns the final date of the containing month, quarter or year.
// Go owns crossing the next period boundary; subtraction precedes projection,
// so the final four-digit year remains valid rather than overflowing early.
func (d CivilDate) PeriodEnd(boundary CalendarBoundary) (CivilDate, error) {
	start, err := d.PeriodStart(boundary)
	if err != nil {
		return CivilDate{}, err
	}
	months := 1
	switch boundary {
	case CalendarMonth:
	case CalendarQuarter:
		months = 3
	case CalendarYear:
		months = 12
	default:
		return CivilDate{}, core.ErrTemporalContract
	}
	return civilDateFromNative(start.native().AddDate(0, months, -1))
}

// CompletedYearsUntil counts full native calendar anniversaries. Go's AddDate
// normalization makes a leap-day anniversary March 1 in a non-leap year.
func (d CivilDate) CompletedYearsUntil(end CivilDate) (int32, error) {
	if err := d.Validate(); err != nil {
		return 0, err
	}
	if err := end.Validate(); err != nil {
		return 0, err
	}
	if end.native().Before(d.native()) {
		return 0, contractError("civil year interval is reversed")
	}
	years := end.year - d.year
	if d.native().AddDate(int(years), 0, 0).After(end.native()) {
		years--
	}
	return years, nil
}

var (
	_ core.Validatable = CivilDate{}
	_ core.OffWireEnum = CalendarBoundaryUnknown
)
