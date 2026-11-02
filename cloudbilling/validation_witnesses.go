package cloudbilling

import "github.com/deliri/primitive/v2026/core"

// Every JSON marshaler here also validates; the compiler holds the pair.
var (
	_ core.ValidatedJSONMarshaler = GeographyType(0)
	_ core.ValidatedJSONMarshaler = Money{}
)
