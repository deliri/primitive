package garble

import "github.com/deliri/primitive/v2026/core"

var (
	_ core.ValidatedJSONMarshaler = Seed{}

	_ core.OffWireEnum = LiteralPolicyUnknown
	_ core.OffWireEnum = DiagnosticPolicyUnknown
	_ core.OffWireEnum = ArgumentKindUnknown
	_ core.OffWireEnum = DerivationGenerationUnknown
	_ core.OffWireEnum = ToolIdentityUnknown
)
