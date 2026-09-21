package upgradereport

import "github.com/deliri/primitive/v2026/core"

var (
	_ core.ValidatedJSONMarshaler = Outcome(0)
	_ core.ValidatedJSONMarshaler = Stage(0)
	_ core.ValidatedJSONMarshaler = Request{}
	_ core.ValidatedJSONMarshaler = Response{}
	_ core.ValidatedJSONMarshaler = EvidenceRequest{}
)
