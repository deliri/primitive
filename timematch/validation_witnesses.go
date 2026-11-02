package timematch

import "github.com/deliri/primitive/v2026/core"

// Every production JSON marshaler is a compiler-visible validated marshaler.
var (
	_ core.ValidatedJSONMarshaler = Provider(0)
	_ core.ValidatedJSONMarshaler = Access(0)
	_ core.ValidatedJSONMarshaler = Lifetime(0)
	_ core.ValidatedJSONMarshaler = MicrosoftTenant(0)
	_ core.ValidatedJSONMarshaler = AvailabilityState(0)
	_ core.ValidatedJSONMarshaler = RevokeOutcome(0)
	_ core.ValidatedJSONMarshaler = DeleteOutcome(0)
	_ core.ValidatedJSONMarshaler = Secret{}
	_ core.ValidatedJSONMarshaler = BrowserAvailabilityRequest{}
	_ core.ValidatedJSONMarshaler = BrowserAvailability{}
	_ core.ValidatedJSONMarshaler = BrowserConnectionRequest{}
	_ core.ValidatedJSONMarshaler = BrowserConnectionStart{}
	_ core.ValidatedJSONMarshaler = BrowserAppleConnection{}
)
