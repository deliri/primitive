package timematch

import (
	"net/url"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/deliri/primitive/v2026/core"
)

// EventVenue is supplied by the appointment organizer, never discovered from
// other calendar events. Address is a physical destination; JoinURL is an
// existing HTTPS video-call link. They are mutually exclusive. The zero value
// leaves an existing event's location untouched; product policy can require it.
type EventVenue struct {
	Address string
	JoinURL string
}

func (v EventVenue) Validate() error {
	if v.Address != "" && v.JoinURL != "" {
		return core.ErrTimeMatchContract
	}
	value := v.Text()
	if !utf8.ValidString(value) || strings.TrimSpace(value) != value {
		return core.ErrTimeMatchContract
	}
	for _, r := range value {
		if unicode.IsControl(r) {
			return core.ErrTimeMatchContract
		}
	}
	if v.JoinURL != "" {
		parsed, err := url.Parse(v.JoinURL)
		if err != nil || parsed.Scheme != "https" || parsed.Hostname() == "" || parsed.User != nil || parsed.Opaque != "" {
			return core.ErrTimeMatchContract
		}
	}
	return nil
}

func (v EventVenue) Text() string {
	if v.Address != "" {
		return v.Address
	}
	return v.JoinURL
}
func (EventVenue) timematchProtocolFact() {}
