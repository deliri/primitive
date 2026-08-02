package attest

import "github.com/deliri/primitive/v2026/core"

var (
	_ core.ValidatedJSONMarshaler = domainToken{}
	_ core.ValidatedJSONMarshaler = Signature{}
)
