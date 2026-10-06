package temporal

import "time"

// TimeLayout is a Go reference-time layout selected by the caller's protocol.
// The zero value is invalid. Temporal owns parsing and formatting mechanics.
type TimeLayout string

func (l TimeLayout) Validate() error {
	if l == "" {
		return contractError("time layout is empty")
	}
	return nil
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
	if n == "" || n == "Local" {
		return nil, contractError("time location must be explicit")
	}
	location, err := time.LoadLocation(string(n))
	if err != nil {
		return nil, contractError("time location is invalid", err)
	}
	return location, nil
}

// ParseTimeRequest admits one canonical UTC representation under its layout.
// Go owns calendar syntax; Instant owns native nanosecond representability.
type ParseTimeRequest struct {
	Layout TimeLayout
	Text   string
}

func (r ParseTimeRequest) Validate() error {
	if err := r.Layout.Validate(); err != nil {
		return err
	}
	if r.Text == "" {
		return contractError("time text is empty")
	}
	return nil
}

func ParseTimeUTC(r ParseTimeRequest) (Instant, error) {
	if err := r.Validate(); err != nil {
		return Instant{}, err
	}
	value, err := time.Parse(string(r.Layout), r.Text)
	if err != nil {
		return Instant{}, contractError("time text is invalid", err)
	}
	if value.UTC().Format(string(r.Layout)) != r.Text {
		return Instant{}, contractError("time text is not canonical UTC")
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
	if err := r.Instant.Validate(); err != nil {
		return "", err
	}
	if err := r.Layout.Validate(); err != nil {
		return "", err
	}
	location, err := r.Location.location()
	if err != nil {
		return "", err
	}
	value, err := r.Instant.Time()
	if err != nil {
		return "", err
	}
	return value.In(location).Format(string(r.Layout)), nil
}
