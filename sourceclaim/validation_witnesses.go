package sourceclaim

import "github.com/deliri/primitive/v2026/core"

var (
	_ core.Validatable = Boundary{}
	_ core.Validatable = Narrative{}
	_ core.Validatable = ExecutionRequirement{}
	_ core.Validatable = CompilerRequirement{}
	_ core.Validatable = Requirement{}
	_ core.Validatable = Claim{}
	_ core.Validatable = Summary{}

	_ core.ValidatedJSONMarshaler = ID{}
	_ core.ValidatedJSONMarshaler = Text{}
	_ core.ValidatedJSONMarshaler = Reference{}
	_ core.ValidatedJSONMarshaler = RequirementMode(0)
	_ core.ValidatedJSONMarshaler = ExecutionKind(0)
	_ core.ValidatedJSONMarshaler = CompilerPredicate(0)
	_ core.ValidatedJSONMarshaler = Claim{}
	_ core.ValidatedJSONMarshaler = Summary{}
)
