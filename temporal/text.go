package temporal

import "time"

// TimeLayout is a Go calendar layout selected by the caller's protocol.
// At least one native Gregorian date or weekday coordinate must be present;
// clock-only, zone-only and literal label layouts are outside this agreement.
// The zero value is invalid. Temporal owns parsing and formatting mechanics.
type TimeLayout string

// NewTimeLayout admits one caller-selected native reference-time layout.
func NewTimeLayout(value string) (TimeLayout, error) {
	layout := TimeLayout(value)
	if err := layout.Validate(); err != nil {
		return "", err
	}
	return layout, nil
}

func (l TimeLayout) Validate() error {
	if l == "" {
		return contractError("time layout is empty")
	}
	// Go is the token interpreter. Two independent Gregorian dates must
	// produce different output; a literal spelling such as YYYY-MM-DD does not.
	// Source: https://pkg.go.dev/time#Time.Format
	first := time.Date(2006, time.January, 2, 15, 4, 5, 123456789, time.UTC)
	second := time.Date(1998, time.November, 24, 15, 4, 5, 123456789, time.UTC)
	if first.Format(string(l)) == second.Format(string(l)) {
		return contractError("time layout contains no native calendar tokens")
	}
	return nil
}

// NativeTimeFormatRequest projects a standard-library timestamp at an SDK
// boundary. Unlike Instant, Go's timestamp includes its location and supports
// calendar years outside signed Unix nanoseconds. Its zero value is the actual
// Go calendar value, not a fabricated current timestamp.
type NativeTimeFormatRequest struct {
	Time   time.Time
	Layout TimeLayout
	Clock  ClockLayout
}

func (r NativeTimeFormatRequest) Validate() error {
	if r.Clock != ClockLayoutUnknown {
		if err := r.Clock.Validate(); err != nil {
			return err
		}
		if r.Layout != "" {
			return contractError("native formatting cannot choose both calendar and clock layouts")
		}
		return nil
	}
	return r.Layout.Validate()
}

func FormatNativeTime(r NativeTimeFormatRequest) (string, error) {
	if err := r.Validate(); err != nil {
		return "", err
	}
	if r.Clock == ClockHourMinuteSecond {
		return r.Time.Format(time.TimeOnly), nil
	}
	return r.Time.Format(string(r.Layout)), nil
}

// LocationName identifies Go's timezone data. It never selects the ambient
// machine timezone implicitly. The caller supplies UTC or a named location.
type LocationName string

const LocationUTC LocationName = "UTC"

func (n LocationName) Validate() error {
	_, err := n.location()
	return err
}

func (n LocationName) location() (*time.Location, error) {
	location, err := NewLocation(n)
	if err != nil {
		return nil, err
	}
	return location.value, nil
}

// ParseTimeRequest admits one canonical UTC representation under its layout.
// Go owns calendar syntax; Instant owns native nanosecond representability.
type ParseTimeRequest struct {
	Layout TimeLayout
	Text   string
}

func (r ParseTimeRequest) Validate() error {
	_, err := r.parseUTC()
	return err
}

func (r ParseTimeRequest) parseUTC() (time.Time, error) {
	if err := r.Layout.Validate(); err != nil {
		return time.Time{}, err
	}
	if r.Text == "" {
		return time.Time{}, contractError("time text is empty")
	}
	value, err := time.Parse(string(r.Layout), r.Text)
	if err != nil {
		return time.Time{}, contractError("time text is invalid", err)
	}
	if value.UTC().Format(string(r.Layout)) != r.Text {
		return time.Time{}, contractError("time text is not canonical UTC")
	}
	return value, nil
}

func ParseTimeUTC(r ParseTimeRequest) (Instant, error) {
	value, err := r.parseUTC()
	if err != nil {
		return Instant{}, err
	}
	return NewInstant(value)
}

// FormatTimeRequest projects an already-owned instant. The caller owns the
// layout and location policy; Temporal never repairs an invalid location.
type FormatTimeRequest struct {
	Layout   TimeLayout
	Location LocationName
	Instant  Instant
}

func (r FormatTimeRequest) Validate() error {
	if err := r.Instant.Validate(); err != nil {
		return err
	}
	if err := r.Layout.Validate(); err != nil {
		return err
	}
	return r.Location.Validate()
}

func FormatTime(r FormatTimeRequest) (string, error) {
	value, err := r.time()
	if err != nil {
		return "", err
	}
	return value.Format(string(r.Layout)), nil
}

// AppendTime keeps caller-owned storage and preserves it on refusal. Go owns
// append formatting; Temporal owns the validated instant, layout and zone.
func AppendTime(destination []byte, r FormatTimeRequest) ([]byte, error) {
	value, err := r.time()
	if err != nil {
		return destination, err
	}
	return value.AppendFormat(destination, string(r.Layout)), nil
}

func (r FormatTimeRequest) time() (time.Time, error) {
	if err := r.Layout.Validate(); err != nil {
		return time.Time{}, err
	}
	location, err := r.Location.location()
	if err != nil {
		return time.Time{}, err
	}
	value, err := r.Instant.Time()
	if err != nil {
		return time.Time{}, err
	}
	return value.In(location), nil
}
