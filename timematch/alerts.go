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
// Its shared bound is Google's maximum of five per-event overrides.
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

// ValidateForProvider refuses a policy the native calendar cannot represent.
// Graph has one reminderMinutesBeforeStart value per event; CalDAV alert
// creation is not supported here. Never silently truncate a caller's policy.
func (a EventAlerts) ValidateForProvider(provider Provider) error {
	if err := a.Validate(); err != nil {
		return err
	}
	switch provider {
	case ProviderGoogle:
		return nil
	case ProviderMicrosoft:
		if len(a) <= 1 {
			return nil
		}
	case ProviderApple:
		if len(a) == 0 {
			return nil
		}
	default:
		return core.ErrTimeMatchContract
	}
	return core.ErrTimeMatchContract
}
