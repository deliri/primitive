package wiring

import "github.com/deliri/primitive/v2026/core"

var (
	_ core.Validatable            = ErrorKind(0)
	_ core.ValidatedJSONMarshaler = ErrorKind(0)
)
