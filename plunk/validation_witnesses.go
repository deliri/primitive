package plunk

import "github.com/deliri/primitive/v2026/core"

// Every JSON marshaler here also validates; the compiler holds the pair.
var (
	_ core.ValidatedJSONMarshaler = SendStatus(0)
	_ core.ValidatedJSONMarshaler = SendTimestamp{}
	_ core.ValidatedJSONMarshaler = TemplateKind(0)
)
