package timematch

import "github.com/deliri/primitive/v2026/core"

// EventCreationAttempt separates an authorized first dispatch from recovery
// after a request may have reached the provider. The durable caller records
// its dispatch before sending it. Recovery never creates a second event when
// the first event's outcome cannot yet be established.
type EventCreationAttempt uint8

const (
	EventCreateFirst EventCreationAttempt = iota + 1
	EventCreateRecoverOnly
)

func (a EventCreationAttempt) Validate() error {
	switch a {
	case EventCreateFirst, EventCreateRecoverOnly:
		return nil
	default:
		return core.ErrTimeMatchContract
	}
}
