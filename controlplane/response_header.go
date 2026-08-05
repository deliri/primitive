package controlplane

import (
	"encoding/json"

	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/lease"
	"github.com/deliri/primitive/v2026/receipt"
	"github.com/deliri/primitive/v2026/temporal"
)

// ResponseHeaderJSONMaximumBytes bounds an accepted response header.
const ResponseHeaderJSONMaximumBytes = 2 << 10

// ResponseHeader is the decision-bearing header every authenticated
// control-plane response carries. A response signature must cover this header
// and its complete body together, never one without the other.
type ResponseHeader struct {
	ProviderTime temporal.Instant         `json:"provider_time"`
	RequestNonce controlwire.RequestNonce `json:"request_nonce"`
	Account      receipt.AccountIdentity  `json:"account"`
	Installation lease.DeviceID           `json:"installation"`
	Revision     controlwire.Revision     `json:"revision"`
	Status       ProductStatus            `json:"status"`
	Offering     core.Offering            `json:"offering"`
	Policy       controlwire.PolicyCursor `json:"policy"`
}

type responseHeaderWire ResponseHeader

// ResponseExpectation is the caller-owned state one authentic response header
// is checked against.
//
// PriorProviderTime may be unset, which is the first-contact case. Once set, an
// authority's clock may not move backward.
type ResponseExpectation struct {
	PriorProviderTime temporal.Instant
	RequestNonce      controlwire.RequestNonce
	Account           receipt.AccountIdentity
	Installation      lease.DeviceID
	Revision          controlwire.Revision
	Offering          core.Offering
}

// ResponseHeaderField names one bound fact, so a binding failure says which
// fact disagreed rather than only that something did.
type ResponseHeaderField uint8

const (
	// ResponseHeaderFieldUnknown is the unset field and names no disagreement.
	ResponseHeaderFieldUnknown ResponseHeaderField = iota
	ResponseHeaderFieldRequestNonce
	ResponseHeaderFieldAccount
	ResponseHeaderFieldInstallation
	ResponseHeaderFieldRevision
	ResponseHeaderFieldOffering
	responseHeaderFieldLimit
)

func responseHeaderFieldTokens() [responseHeaderFieldLimit]string {
	return [...]string{
		ResponseHeaderFieldUnknown:      "",
		ResponseHeaderFieldRequestNonce: protocolMemberRequestNonce,
		ResponseHeaderFieldAccount:      core.ProtocolMemberAccount,
		ResponseHeaderFieldInstallation: protocolMemberInstallation,
		ResponseHeaderFieldRevision:     protocolMemberRevision,
		ResponseHeaderFieldOffering:     core.ProtocolMemberOffering,
	}
}

// Validate rejects the unset field and every field outside the domain.
func (f ResponseHeaderField) Validate() error {
	if f <= ResponseHeaderFieldUnknown || f >= responseHeaderFieldLimit {
		return responseHeaderError()
	}
	return nil
}

// IsValid reports whether f names a real bound fact.
func (f ResponseHeaderField) IsValid() bool { return f.Validate() == nil }

// String returns the field's wire name, or empty text when unset.
func (f ResponseHeaderField) String() string {
	if f >= responseHeaderFieldLimit {
		return ""
	}
	return responseHeaderFieldTokens()[f]
}

// ParseResponseHeaderField accepts one exact bound-fact name.
func ParseResponseHeaderField(value string) (ResponseHeaderField, error) {
	tokens := responseHeaderFieldTokens()
	for field := ResponseHeaderFieldUnknown + 1; field < responseHeaderFieldLimit; field++ {
		if tokens[field] != "" && tokens[field] == value {
			return field, nil
		}
	}
	return ResponseHeaderFieldUnknown, responseHeaderError()
}

// MarshalJSON emits the field's wire name and refuses the unset field.
//
// A binding failure names the exact fact that disagreed, and that name travels
// in diagnostics. Emitting the unset field would report a disagreement about
// nothing, which reads as a passing check.
func (f ResponseHeaderField) MarshalJSON() ([]byte, error) {
	if err := f.Validate(); err != nil {
		return nil, jsonError(err)
	}
	encoded, err := core.MarshalCanonicalJSONString(f.String())
	if err != nil {
		return nil, jsonError(responseHeaderError(err))
	}
	return encoded, nil
}

// UnmarshalJSON accepts only a named bound fact and leaves f unchanged on every
// rejection.
func (f *ResponseHeaderField) UnmarshalJSON(data []byte) error {
	if f == nil {
		return jsonError(responseHeaderError())
	}
	token, err := core.DecodeJSONStringToken(data)
	if err != nil {
		return jsonError(responseHeaderError(err))
	}
	parsed, err := ParseResponseHeaderField(token)
	if err != nil {
		return jsonError(err)
	}
	*f = parsed
	return nil
}

// Validate closes the header's intrinsic shape.
func (h ResponseHeader) Validate() error {
	if err := h.validateIdentity(); err != nil {
		return err
	}
	if err := h.Revision.Validate(); err != nil {
		return responseHeaderError(err)
	}
	if err := h.Status.Validate(); err != nil {
		return responseHeaderError(err)
	}
	if err := h.Offering.Validate(); err != nil {
		return responseHeaderError(err)
	}
	if err := h.Policy.Validate(); err != nil {
		return responseHeaderError(err)
	}
	return nil
}

func (h ResponseHeader) validateIdentity() error {
	if err := h.ProviderTime.Validate(); err != nil {
		return responseHeaderError(err)
	}
	if err := h.RequestNonce.Validate(); err != nil {
		return responseHeaderError(err)
	}
	if err := h.Account.Validate(); err != nil {
		return responseHeaderError(err)
	}
	if err := h.Installation.Validate(); err != nil {
		return responseHeaderError(err)
	}
	return nil
}

// Validate closes the complete caller expectation.
func (e ResponseExpectation) Validate() error {
	if err := e.RequestNonce.Validate(); err != nil {
		return responseHeaderError(err)
	}
	if err := e.Account.Validate(); err != nil {
		return responseHeaderError(err)
	}
	if err := e.Installation.Validate(); err != nil {
		return responseHeaderError(err)
	}
	if err := e.Revision.Validate(); err != nil {
		return responseHeaderError(err)
	}
	if err := e.Offering.Validate(); err != nil {
		return responseHeaderError(err)
	}
	if !e.PriorProviderTime.IsSet() {
		return nil
	}
	if err := e.PriorProviderTime.Validate(); err != nil {
		return responseHeaderError(err)
	}
	return nil
}

// ValidateAgainst binds a received header to the exact request that produced
// it and refuses a clock that moved backward.
//
// Every rejection carries a distinct compiler-visible identity: an intrinsic
// shape failure keeps its owning sentinel, a disagreeing bound fact returns a
// binding error naming that exact field, and a backward authority instant
// returns the rollback identity. A caller that can only see "invalid" cannot
// tell a forged response from a stale one.
func (h ResponseHeader) ValidateAgainst(expectation ResponseExpectation) error {
	if err := h.Validate(); err != nil {
		return err
	}
	if err := expectation.Validate(); err != nil {
		return err
	}
	if field, mismatched := h.boundFieldMismatch(expectation); mismatched {
		return NewResponseBindingError(field)
	}
	return h.validateProviderTimeAdvance(expectation.PriorProviderTime)
}

// boundFieldMismatch names the first bound fact that disagrees with the
// caller's trusted expectation.
func (h ResponseHeader) boundFieldMismatch(expectation ResponseExpectation) (ResponseHeaderField, bool) {
	switch {
	case h.RequestNonce != expectation.RequestNonce:
		return ResponseHeaderFieldRequestNonce, true
	case h.Account != expectation.Account:
		return ResponseHeaderFieldAccount, true
	case h.Installation != expectation.Installation:
		return ResponseHeaderFieldInstallation, true
	case h.Revision != expectation.Revision:
		return ResponseHeaderFieldRevision, true
	case h.Offering != expectation.Offering:
		return ResponseHeaderFieldOffering, true
	}
	return ResponseHeaderFieldUnknown, false
}

// validateProviderTimeAdvance refuses an authority instant that moved backward
// from a previously trusted one. An unset prior instant is first contact and
// admits any valid instant.
func (h ResponseHeader) validateProviderTimeAdvance(prior temporal.Instant) error {
	if !prior.IsSet() {
		return nil
	}
	comparison, err := h.ProviderTime.Compare(prior)
	if err != nil {
		return responseHeaderError(err)
	}
	if comparison == core.ComparisonLess {
		return documentError(core.ErrControlPlaneProviderTimeRollback)
	}
	return nil
}

// MarshalJSON emits one validated canonical header.
func (h ResponseHeader) MarshalJSON() ([]byte, error) {
	if err := h.Validate(); err != nil {
		return nil, jsonError(err)
	}
	encoded, err := core.MarshalCanonicalJSONDocument(responseHeaderWire(h))
	if err != nil || len(encoded) > ResponseHeaderJSONMaximumBytes {
		return nil, jsonError(responseHeaderError(err))
	}
	return encoded, nil
}

// UnmarshalJSON strictly decodes without mutating the receiver on rejection.
func (h *ResponseHeader) UnmarshalJSON(data []byte) error {
	if h == nil {
		return jsonError(responseHeaderError())
	}
	limits, err := documentJSONLimits(ResponseHeaderJSONMaximumBytes)
	if err != nil {
		return jsonError(responseHeaderError(err))
	}
	wire, err := core.DecodeStrictJSONStructure[responseHeaderWire](data, limits)
	if err != nil {
		return jsonError(responseHeaderError(err))
	}
	candidate := ResponseHeader(wire)
	if err := candidate.Validate(); err != nil {
		return jsonError(err)
	}
	*h = candidate
	return nil
}

var (
	_ core.Validatable = ResponseHeader{}
	_ core.Validatable = ResponseExpectation{}
	_ core.Validatable = ResponseHeaderFieldUnknown
	_ json.Marshaler   = ResponseHeader{}
	_ json.Unmarshaler = (*ResponseHeader)(nil)
)
