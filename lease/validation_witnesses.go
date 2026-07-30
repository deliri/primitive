package lease

import (
	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/core"
)

var (
	_ core.Validatable = Product{}
	_ core.Validatable = EntitlementID{}
	_ core.Validatable = DeviceID{}
	_ core.Validatable = Subject{}
	_ core.Validatable = Generation{}
	_ core.Validatable = RevisionUnknown
	_ core.Validatable = OutcomeUnknown
	_ core.Validatable = RevocationReasonUnknown
	_ core.Validatable = DomainUnknown
	_ core.Validatable = Header{}
	_ core.Validatable = Grant{}
	_ core.Validatable = Refusal{}
	_ core.Validatable = Revocation{}
	_ core.Validatable = Decision{}
	_ core.Validatable = Document{}
	_ core.Validatable = VerifyRequest{}
	_ core.Validatable = Verified{}
	_ core.Validatable = EvaluateRequest{}
	_ core.Validatable = Assessment{}
	_ core.Validatable = AdvanceRequest{}
	_ core.Validatable = AdvanceResult{}
)

var (
	_ core.ValidatedJSONMarshaler = Product{}
	_ core.ValidatedJSONMarshaler = EntitlementID{}
	_ core.ValidatedJSONMarshaler = DeviceID{}
	_ core.ValidatedJSONMarshaler = Subject{}
	_ core.ValidatedJSONMarshaler = Generation{}
	_ core.ValidatedJSONMarshaler = RevisionUnknown
	_ core.ValidatedJSONMarshaler = OutcomeUnknown
	_ core.ValidatedJSONMarshaler = RevocationReasonUnknown
	_ core.ValidatedJSONMarshaler = Grant{}
	_ core.ValidatedJSONMarshaler = Refusal{}
	_ core.ValidatedJSONMarshaler = Revocation{}
	_ core.ValidatedJSONMarshaler = Decision{}
	_ core.ValidatedJSONMarshaler = Document{}
)

var (
	_ attest.CanonicalBody[Domain] = Decision{}
)
