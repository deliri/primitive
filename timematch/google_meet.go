package timematch

import "github.com/deliri/primitive/v2026/core"

// EventVenueStatus is the observed outcome of automatic video-call creation.
// Pending proves the owned event exists, not that a usable call is ready.
// Failed is an explicit provider refusal. The caller may read the same event
// again; this package never polls, chooses another identity or retries a failure.
type EventVenueStatus uint8

const (
	EventVenueNone EventVenueStatus = iota
	EventVenuePending
	EventVenueReady
	EventVenueFailed
)

func (s EventVenueStatus) Validate() error {
	switch s {
	case EventVenueNone, EventVenuePending, EventVenueReady, EventVenueFailed:
		return nil
	default:
		return core.ErrTimeMatchContract
	}
}

func (EventVenueStatus) OffWireEnum() {}

var (
	_ core.Validatable = EventVenueNone
	_ core.OffWireEnum = EventVenueNone
)
