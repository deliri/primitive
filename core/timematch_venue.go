package core

import (
	"net/url"
	"strings"
	"unicode"
	"unicode/utf8"
)

// TimeMatchAppointmentVenue is a provider-independent destination supplied by
// the appointment organizer, never discovered from
// other calendar events. Address is a physical destination; JoinURL is an
// existing HTTPS video-call link. They are mutually exclusive. The zero value
// leaves an existing event's location untouched; product policy can require it.
type TimeMatchAppointmentVenue struct {
	Address string
	JoinURL string
}

// Validate admits a single physical destination or credential-free HTTPS call
// link. An unspecified SDK destination is neutral; applications may require it.
func (v TimeMatchAppointmentVenue) Validate() error {
	if v.Address != "" && v.JoinURL != "" {
		return ErrTimeMatchContract
	}
	value := v.Text()
	if !utf8.ValidString(value) || strings.TrimSpace(value) != value {
		return ErrTimeMatchContract
	}
	for _, r := range value {
		if unicode.IsControl(r) {
			return ErrTimeMatchContract
		}
	}
	if v.JoinURL != "" {
		parsed, err := url.Parse(v.JoinURL)
		if err != nil || parsed.Scheme != "https" || parsed.Hostname() == "" || parsed.User != nil || parsed.Opaque != "" {
			return ErrTimeMatchContract
		}
	}
	return nil
}

// Text returns the destination displayed in the calendar location field.
func (v TimeMatchAppointmentVenue) Text() string {
	if v.Address != "" {
		return v.Address
	}
	return v.JoinURL
}
