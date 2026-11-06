package timematch

import (
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

const (
	maximumAlertMinutes = 40320
	maximumEventAlerts  = 5
)

// EventAlerts requests calendar notifications, not email or attendee invites.
// Its bound is Google's documented maximum of five per-event overrides.
// https://developers.google.com/workspace/calendar/api/v3/reference/events
type EventAlerts []temporal.Duration

func (a EventAlerts) Validate() error {
	if len(a) > maximumEventAlerts {
		return core.ErrTimeMatchContract
	}
	for index, lead := range a {
		if lead.Validate() != nil || lead.Nanoseconds()%int64(temporal.NanosecondsPerMinute) != 0 || lead.Nanoseconds() > maximumAlertMinutes*int64(temporal.NanosecondsPerMinute) {
			return core.ErrTimeMatchContract
		}
		for _, prior := range a[:index] {
			if prior == lead {
				return core.ErrTimeMatchContract
			}
		}
	}
	return nil
}
