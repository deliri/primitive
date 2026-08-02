package receipt

import "github.com/deliri/primitive/v2026/core"

var (
	_ core.ValidatedJSONMarshaler = Revision(0)
	_ core.ValidatedJSONMarshaler = ReceiptID{}
	_ core.ValidatedJSONMarshaler = Generation{}
	_ core.ValidatedJSONMarshaler = EvidenceBody{}
	_ core.ValidatedJSONMarshaler = Header{}
	_ core.ValidatedJSONMarshaler = EvidencePayload{}
	_ core.ValidatedJSONMarshaler = EvidenceDocument{}
	_ core.ValidatedJSONMarshaler = AccountIdentity{}
	_ core.ValidatedJSONMarshaler = OfferingIdentity{}
	_ core.ValidatedJSONMarshaler = SubmissionIdentity{}
	_ core.ValidatedJSONMarshaler = ObjectIdentity{}
	_ core.ValidatedJSONMarshaler = CursorDigest{}
	_ core.ValidatedJSONMarshaler = ChainHash{}
	_ core.ValidatedJSONMarshaler = Scope{}
	_ core.ValidatedJSONMarshaler = Watermark{}
)
