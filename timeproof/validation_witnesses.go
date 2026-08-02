package timeproof

import "github.com/deliri/primitive/v2026/core"

var (
	_ core.ValidatedJSONMarshaler = Authority(0)
	_ core.ValidatedJSONMarshaler = TimestampPolicy(0)
	_ core.ValidatedJSONMarshaler = authorityEvidenceWire{}
	_ core.ValidatedJSONMarshaler = AuthorityEvidence{}
	_ core.ValidatedJSONMarshaler = Nonce{}
	_ core.ValidatedJSONMarshaler = requestWire{}
	_ core.ValidatedJSONMarshaler = Request{}
	_ core.ValidatedJSONMarshaler = SerialNumber{}
	_ core.ValidatedJSONMarshaler = authoritativeTimestampWire{}
	_ core.ValidatedJSONMarshaler = AuthoritativeTimestamp{}
)
