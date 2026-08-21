package manual

import "github.com/deliri/primitive/v2026/core"

var (
	_ core.Validatable            = TopicName("")
	_ core.Validatable            = Line("")
	_ core.Validatable            = Schema("")
	_ core.Validatable            = Definition{}
	_ core.Validatable            = Outcome{}
	_ core.Validatable            = ViewUnknown
	_ core.Validatable            = SelectionModeUnknown
	_ core.Validatable            = Report{}
	_ core.Validatable            = PageReport{}
	_ core.ValidatedJSONMarshaler = View(0)
	_ core.ValidatedJSONMarshaler = SelectionMode(0)
	_ core.OffWireEnum            = ViewUnknown
	_ core.OffWireEnum            = SelectionModeUnknown
)
