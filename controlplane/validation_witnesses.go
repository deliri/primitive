package controlplane

import (
	"encoding"
	json "encoding/json/v2"

	"github.com/deliri/primitive/v2026/core"
)

// The enum witnesses are written as conversions of their own named zero rather
// than as bare constants. The inventory ratchet reads these declarations
// through the AST and needs the type name to be present in the expression; a
// bare constant identifier names a value whose type it cannot recover.
var (
	_ core.Validatable = ProductStatus(ProductStatusInvalid)
	_ core.Validatable = SigningDomain(SigningDomainUnknown)
	_ core.Validatable = ResponseHeaderField(ResponseHeaderFieldUnknown)
	_ core.Validatable = UsageWatermark{}
	_ core.Validatable = ResponseHeader{}
	_ core.Validatable = ResponseExpectation{}

	_ core.ValidatedJSONMarshaler = ProductStatus(ProductStatusInvalid)
	_ core.ValidatedJSONMarshaler = SigningDomain(SigningDomainUnknown)
	_ core.ValidatedJSONMarshaler = ResponseHeaderField(ResponseHeaderFieldUnknown)
	_ core.ValidatedJSONMarshaler = UsageWatermark{}
	_ core.ValidatedJSONMarshaler = ResponseHeader{}

	_ json.Unmarshaler = (*ProductStatus)(nil)
	_ json.Unmarshaler = (*SigningDomain)(nil)
	_ json.Unmarshaler = (*ResponseHeaderField)(nil)
	_ json.Unmarshaler = (*UsageWatermark)(nil)
	_ json.Unmarshaler = (*ResponseHeader)(nil)

	// The signing domain also travels as text, because Attest covers the domain
	// text itself inside the signature. The JSON and text spellings are the same
	// bytes, so a document cannot verify under one and read as the other.
	_ encoding.TextMarshaler = SigningDomain(SigningDomainUnknown)
)
