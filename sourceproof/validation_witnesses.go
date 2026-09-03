package sourceproof

import "github.com/deliri/primitive/v2026/core"

var (
	_ core.Validatable = EvidenceReference{}
	_ core.Validatable = RequirementResult{}
	_ core.Validatable = Result{}
	_ core.Validatable = Summary{}

	_ core.ValidatedJSONMarshaler = State(0)
	_ core.ValidatedJSONMarshaler = EvidenceKind(0)
	_ core.ValidatedJSONMarshaler = Result{}
	_ core.ValidatedJSONMarshaler = Summary{}
)
