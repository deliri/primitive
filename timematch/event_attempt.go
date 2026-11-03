package timematch

import "github.com/deliri/primitive/v2026/core"

// EventCreationAttempt separates an authorized first dispatch from recovery
// after a request may have reached the provider. The durable caller records
// its dispatch before sending it. Recovery is restricted to that same current
// intent. RecoverOnly is always read-only. Guarded recovery separately grants
// permission to repeat a provider-supported conditional create; where that
// guard is unavailable it remains read-only. Neither chooses a fresh identity.
type EventCreationAttempt uint8

const (
	EventCreateFirst EventCreationAttempt = iota + 1
	EventCreateRecoverOnly
	EventCreateGuardedRecovery
)

func (a EventCreationAttempt) Validate() error {
	switch a {
	case EventCreateFirst, EventCreateRecoverOnly, EventCreateGuardedRecovery:
		return nil
	default:
		return core.ErrTimeMatchContract
	}
}

func eventCreationAttemptNames() []string {
	return []string{"first", "recover-only", "recover-guarded"}
}

func (a EventCreationAttempt) IsValid() bool  { return a.Validate() == nil }
func (a EventCreationAttempt) String() string { return enumName(a, eventCreationAttemptNames()) }
func (a EventCreationAttempt) MarshalJSON() ([]byte, error) {
	return marshalEnum(a, eventCreationAttemptNames())
}
func (a *EventCreationAttempt) UnmarshalJSON(data []byte) error {
	return unmarshalEnum(a, data, eventCreationAttemptNames())
}
