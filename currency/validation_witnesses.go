package currency

import "github.com/deliri/primitive/v2026/core"

var (
	_ core.ValidatedJSONMarshaler = Code(0)
	_ core.ValidatedJSONMarshaler = Amount{}
	_ core.ValidatedJSONMarshaler = minorUnitsJSON{}
)
