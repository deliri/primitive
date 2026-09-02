package proofledger

import "github.com/deliri/primitive/v2026/core"

var (
	_ core.ValidatedJSONMarshaler = LedgerIdentity{}
	_ core.ValidatedJSONMarshaler = EventIdentity{}
	_ core.ValidatedJSONMarshaler = Sequence(0)
	_ core.ValidatedJSONMarshaler = Position(0)
	_ core.ValidatedJSONMarshaler = PageLimit{}
	_ core.ValidatedJSONMarshaler = Receipt{}
	_ core.ValidatedJSONMarshaler = ReceiptDocument{}
)
