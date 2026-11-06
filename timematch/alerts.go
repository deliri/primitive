package timematch

import "github.com/deliri/primitive/v2026/core"

// AlertMinutes is the provider's event-relative lead time. Google permits
// zero (at the start) through four weeks before the event.
type AlertMinutes uint16

const (
	maximumAlertMinutes = 40320
	maximumEventAlerts  = 5
)

// EventAlerts requests calendar notifications, not email or attendee invites.
// Its bound is Google's documented maximum of five per-event overrides.
// https://developers.google.com/workspace/calendar/api/v3/reference/events
type EventAlerts []AlertMinutes

func (a EventAlerts) Validate() error {
	if len(a) > maximumEventAlerts {
		return core.ErrTimeMatchContract
	}
	for index, minutes := range a {
		if minutes > maximumAlertMinutes {
			return core.ErrTimeMatchContract
		}
		for _, prior := range a[:index] {
			if prior == minutes {
				return core.ErrTimeMatchContract
			}
		}
	}
	return nil
}
