package upgrade

import "github.com/deliri/primitive/v2026/core"

var (
	_ core.ValidatedJSONMarshaler = Slot(0)
	_ core.ValidatedJSONMarshaler = selectionRevision(0)
	_ core.ValidatedJSONMarshaler = selectionDocument{}
	_ core.ValidatedJSONMarshaler = trialRevision(0)
	_ core.ValidatedJSONMarshaler = trialDocument{}
)
