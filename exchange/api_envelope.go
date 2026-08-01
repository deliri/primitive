package exchange

import (
	"encoding/json"
	"errors"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/deliri/primitive/v2026/core"
)

const (
	// APIRequestIDMissing is the explicit token a correlation identifier
	// degrades to when the caller supplied nothing usable. A response still
	// carries a request identifier so an operator reading a log line can tell
	// "the client sent nothing" apart from "the field was dropped".
	APIRequestIDMissing = "missing"
	// APIRequestIDMaximumRunes bounds one correlation identifier.
	APIRequestIDMaximumRunes = 256
	// APIErrorMessageMaximumRunes bounds one operator-facing failure message.
	APIErrorMessageMaximumRunes = 1024
	// APIErrorTipMaximumRunes bounds one operator-facing remediation tip.
	APIErrorTipMaximumRunes = 1024

	// APICodeTokenNotFound is the wire token for a missing addressed resource.
	APICodeTokenNotFound = "not_found"
	// APICodeTokenInvalidInput is the wire token for a rejected request value.
	APICodeTokenInvalidInput = "invalid_input"
	// APICodeTokenConflict is the wire token for a rejected state transition.
	APICodeTokenConflict = "conflict"
	// APICodeTokenUnauthorized is the wire token for absent authentication.
	APICodeTokenUnauthorized = "unauthorized"
	// APICodeTokenForbidden is the wire token for insufficient authorization.
	APICodeTokenForbidden = "forbidden"
	// APICodeTokenPayloadTooLarge is the wire token for an over-extent body.
	APICodeTokenPayloadTooLarge = "payload_too_large" // #nosec G101 -- public API failure token, not a credential.
	// APICodeTokenServiceUnavailable is the wire token for a refused attempt
	// the client may retry later.
	APICodeTokenServiceUnavailable = "service_unavailable"
	// APICodeTokenInternal is the wire token for an unattributed server fault.
	APICodeTokenInternal = "internal"
)

// APICode is the closed machine-readable reason one API response failed. It is
// what a client branches on; APIErrorBody.Message is what an operator reads.
// Unlike Exchange's transport enums this one crosses the wire, so it owns an
// exact token grammar in both directions rather than a diagnostic projection.
type APICode uint8

const (
	// APICodeUnknown is the invalid zero failure reason.
	APICodeUnknown APICode = iota
	// APICodeNotFound reports that the addressed resource does not exist.
	APICodeNotFound
	// APICodeInvalidInput reports a rejected request value.
	APICodeInvalidInput
	// APICodeConflict reports a rejected state transition.
	APICodeConflict
	// APICodeUnauthorized reports absent or unusable authentication.
	APICodeUnauthorized
	// APICodeForbidden reports authenticated but insufficient authorization.
	APICodeForbidden
	// APICodePayloadTooLarge reports an over-extent request body.
	APICodePayloadTooLarge
	// APICodeServiceUnavailable reports a refused attempt the client may retry.
	APICodeServiceUnavailable
	// APICodeInternal reports an unattributed server fault.
	APICodeInternal
	apiCodeLimit
)

func apiCodeTokens() [apiCodeLimit]string {
	return [...]string{
		APICodeNotFound:           APICodeTokenNotFound,
		APICodeInvalidInput:       APICodeTokenInvalidInput,
		APICodeConflict:           APICodeTokenConflict,
		APICodeUnauthorized:       APICodeTokenUnauthorized,
		APICodeForbidden:          APICodeTokenForbidden,
		APICodePayloadTooLarge:    APICodeTokenPayloadTooLarge,
		APICodeServiceUnavailable: APICodeTokenServiceUnavailable,
		APICodeInternal:           APICodeTokenInternal,
	}
}

// Validate rejects failure reasons outside the closed domain.
func (c APICode) Validate() error {
	if c >= apiCodeLimit || apiCodeTokens()[c] == "" {
		return apiContractError(errors.New(apiCodeDomainErrorText))
	}
	return nil
}

// IsValid reports whether c belongs to the closed failure domain.
func (c APICode) IsValid() bool { return c.Validate() == nil }

// String returns the exact wire token, or the empty string for a value outside
// the closed domain.
func (c APICode) String() string {
	if !c.IsValid() {
		return ""
	}
	return apiCodeTokens()[c]
}

// ParseAPICode admits one exact wire token. Token matching is byte-exact: a
// near-miss spelling is a protocol defect, not a value to be repaired.
func ParseAPICode(token string) (APICode, error) {
	for code := APICodeUnknown + 1; code < apiCodeLimit; code++ {
		if apiCodeTokens()[code] == token {
			return code, nil
		}
	}
	return APICodeUnknown, apiContractError(errors.New(apiCodeTokenErrorText))
}

// MarshalJSON emits the exact wire token as one canonical JSON string.
func (c APICode) MarshalJSON() ([]byte, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}
	encoded, err := core.MarshalCanonicalJSONString(c.String())
	if err != nil {
		return nil, apiContractError(err)
	}
	return encoded, nil
}

// UnmarshalJSON admits one exact wire token and leaves the receiver untouched
// on every rejection.
func (c *APICode) UnmarshalJSON(data []byte) error {
	if c == nil {
		return apiContractError(errors.New(apiCodeReceiverErrorText))
	}
	token, err := core.DecodeJSONStringToken(data)
	if err != nil {
		return apiContractError(err)
	}
	parsed, err := ParseAPICode(token)
	if err != nil {
		return err
	}
	*c = parsed
	return nil
}

// APIRequestID correlates one client request with one server log line. It is a
// diagnostic aid rather than a protocol decision, which is why NewAPIRequestID
// degrades an unusable value instead of failing the response that carries it.
type APIRequestID struct {
	value string
}

// NewAPIRequestID normalizes caller-supplied correlation text and never fails.
// Control runes are dropped, the result is bounded in runes and trimmed, and a
// value that still cannot satisfy the contract becomes APIRequestIDMissing.
func NewAPIRequestID(value string) APIRequestID {
	candidate := APIRequestID{value: normalizeAPIRequestID(value)}
	if candidate.Validate() != nil {
		return APIRequestID{value: APIRequestIDMissing}
	}
	return candidate
}

// ParseAPIRequestID admits one identifier that is already canonical. It is the
// strict counterpart of NewAPIRequestID and is what the wire boundary uses, so
// a received identifier is never silently repaired into a different value.
func ParseAPIRequestID(value string) (APIRequestID, error) {
	candidate := APIRequestID{value: value}
	if err := candidate.Validate(); err != nil {
		return APIRequestID{}, err
	}
	return candidate, nil
}

// String returns the wire value.
func (id APIRequestID) String() string { return id.value }

// Validate rejects an unset or non-canonical identifier.
func (id APIRequestID) Validate() error {
	if err := validateAPIText(id.value, APIRequestIDMaximumRunes); err != nil {
		return apiContractError(err)
	}
	return nil
}

// MarshalJSON emits the identifier as one canonical JSON string.
func (id APIRequestID) MarshalJSON() ([]byte, error) {
	if err := id.Validate(); err != nil {
		return nil, err
	}
	encoded, err := core.MarshalCanonicalJSONString(id.value)
	if err != nil {
		return nil, apiContractError(err)
	}
	return encoded, nil
}

// UnmarshalJSON admits one canonical identifier and leaves the receiver
// untouched on every rejection.
func (id *APIRequestID) UnmarshalJSON(data []byte) error {
	if id == nil {
		return apiContractError(errors.New(apiRequestIDReceiverErrorText))
	}
	value, err := core.DecodeJSONStringToken(data)
	if err != nil {
		return apiContractError(err)
	}
	parsed, err := ParseAPIRequestID(value)
	if err != nil {
		return err
	}
	*id = parsed
	return nil
}

// APIErrorBody is the typed failure payload. Code is the closed fact a client
// branches on; Message and Tip are bounded operator text and carry no protocol
// meaning.
type APIErrorBody struct {
	Message string  `json:"message"`
	Tip     string  `json:"tip,omitempty"`
	Code    APICode `json:"code"`
}

// Validate closes the failure reason and bounds both operator strings. An
// absent tip is admitted; a present but non-canonical one is not.
func (b APIErrorBody) Validate() error {
	if err := b.Code.Validate(); err != nil {
		return err
	}
	if err := validateAPIText(b.Message, APIErrorMessageMaximumRunes); err != nil {
		return apiContractError(err)
	}
	if b.Tip == "" {
		return nil
	}
	if err := validateAPIText(b.Tip, APIErrorTipMaximumRunes); err != nil {
		return apiContractError(err)
	}
	return nil
}

// MarshalJSON emits one canonical failure payload.
func (b APIErrorBody) MarshalJSON() ([]byte, error) {
	if err := b.Validate(); err != nil {
		return nil, err
	}
	type wire APIErrorBody
	return marshalCanonicalAPIDocument(wire(b))
}

// APINoBody is the compiler-owned absent success payload. It exists so a
// failure envelope has a concrete data type to instantiate, and it is a
// different fact from NoBody: NoBody means the HTTP message carries no body at
// all, while an envelope holding APINoBody is itself a body whose data arm is
// unused.
type APINoBody struct{}

// Validate refuses use as a data arm. APIEnvelope does not validate its body
// type on the failure path, so APINoBody can instantiate that path without
// creating a second spelling for success-without-data.
func (APINoBody) Validate() error {
	return apiContractError(errors.New(apiNoBodyDataErrorText))
}

// MarshalJSON refuses encoding for the same reason. The method exists only so
// APINoBody satisfies the envelope's compiler-owned body constraint on the
// failure path; there is intentionally no wire spelling for it as data.
func (APINoBody) MarshalJSON() ([]byte, error) {
	return nil, apiContractError(errors.New(apiNoBodyDataErrorText))
}

// APIOutcome is the closed reading of one envelope. It is derived from the arm
// the envelope carries and never appears on the wire: the document expresses
// the same fact by carrying exactly one arm, so a second encoded copy could
// disagree with the members beside it.
type APIOutcome uint8

const (
	// APIOutcomeUnknown is the invalid zero reading.
	APIOutcomeUnknown APIOutcome = iota
	// APIOutcomeSuccess reports that the envelope carries the data arm.
	APIOutcomeSuccess
	// APIOutcomeFailure reports that the envelope carries the error arm.
	APIOutcomeFailure
	apiOutcomeLimit
)

// Validate rejects readings outside the closed domain.
func (o APIOutcome) Validate() error {
	if o <= APIOutcomeUnknown || o >= apiOutcomeLimit {
		return apiContractError(errors.New(apiOutcomeDomainErrorText))
	}
	return nil
}

// IsValid reports whether o belongs to the closed reading domain.
func (o APIOutcome) IsValid() bool { return o.Validate() == nil }

// OffWireEnum declares APIOutcome as a derived reading rather than wire syntax.
func (APIOutcome) OffWireEnum() {}

// String returns a diagnostic projection, not a wire value.
func (o APIOutcome) String() string {
	if !o.IsValid() {
		return ""
	}
	return [...]string{
		APIOutcomeUnknown: "",
		APIOutcomeSuccess: "success",
		APIOutcomeFailure: "failure",
	}[o]
}

// APIEnvelope carries exactly one of data or error. An envelope holding both,
// or neither, is rejected rather than resolved by precedence: a response that
// reports success and failure at once has no correct reading, and choosing one
// arm would hide the producer defect that created it.
//
// The body constrains to core.ValidatedJSONMarshaler rather than
// core.Validatable so a payload always owns an explicit JSON representation.
// Exchange already demands that of every JSON request body, and Core's strict
// encoder cannot accept a value without it; a payload that silently encoded as
// an empty object would otherwise pass every validation the envelope performs.
type APIEnvelope[Body core.ValidatedJSONMarshaler] struct {
	Data      *Body         `json:"data,omitempty"`
	Error     *APIErrorBody `json:"error,omitempty"`
	RequestID APIRequestID  `json:"request_id"`
}

// Validate requires a canonical correlation identifier and exactly one arm.
func (e APIEnvelope[Body]) Validate() error {
	if err := e.RequestID.Validate(); err != nil {
		return err
	}
	switch {
	case e.Data != nil && e.Error == nil:
		if err := (*e.Data).Validate(); err != nil {
			return apiContractError(err)
		}
		return nil
	case e.Data == nil && e.Error != nil:
		return e.Error.Validate()
	default:
		return apiContractError(errors.New(apiEnvelopeArmErrorText))
	}
}

// Outcome reports which arm the envelope carries. It exists so a consumer
// branches on a compiler-visible closed fact instead of repeating a
// pointer-nullity convention at every call site, which is how both mined
// implementations read their envelopes.
func (e APIEnvelope[Body]) Outcome() (APIOutcome, error) {
	if err := e.Validate(); err != nil {
		return APIOutcomeUnknown, err
	}
	if e.Data != nil {
		return APIOutcomeSuccess, nil
	}
	return APIOutcomeFailure, nil
}

// Payload returns the success payload. It fails unless the envelope carries the
// data arm, so a well-formed failure envelope cannot be read as an empty
// success and a caller cannot forget to check before dereferencing.
func (e APIEnvelope[Body]) Payload() (Body, error) {
	var zero Body
	if err := e.Validate(); err != nil {
		return zero, err
	}
	if e.Data == nil {
		return zero, apiContractError(errors.New(apiEnvelopeSuccessErrorText))
	}
	return *e.Data, nil
}

// Failure returns the failure payload. It fails unless the envelope carries the
// error arm.
func (e APIEnvelope[Body]) Failure() (APIErrorBody, error) {
	if err := e.Validate(); err != nil {
		return APIErrorBody{}, err
	}
	if e.Error == nil {
		return APIErrorBody{}, apiContractError(errors.New(apiEnvelopeFailureErrorText))
	}
	return *e.Error, nil
}

// MarshalJSON emits one canonical envelope. The absent arm is omitted rather
// than emitted as null, so exactly one arm is ever present on the wire.
func (e APIEnvelope[Body]) MarshalJSON() ([]byte, error) {
	if err := e.Validate(); err != nil {
		return nil, err
	}
	type wire APIEnvelope[Body]
	return marshalCanonicalAPIDocument(wire(e))
}

// marshalCanonicalAPIDocument projects one API wire value through Core's single
// canonical document encoder and restates the failure under this package's
// stable identity. The encoding rule itself has one owner; this adds only the
// error identity an Exchange caller branches on.
func marshalCanonicalAPIDocument[Document any](document Document) ([]byte, error) {
	encoded, err := core.MarshalCanonicalJSONDocument(document)
	if err != nil {
		return nil, apiContractError(err)
	}
	return encoded, nil
}

// validateAPIText owns the admission rule every operator-facing API token
// shares: present, valid UTF-8, free of control runes and of the replacement
// rune, carrying no surrounding whitespace so one value has one spelling, and
// bounded in runes rather than bytes so the bound means the same thing in every
// script. The replacement rune is refused because a literal U+FFFD in a
// diagnostic token is always the residue of a lossy decode upstream, and two
// separately mangled values would otherwise correlate as one.
func validateAPIText(value string, maximumRunes int) error {
	if value == "" {
		return errors.New(apiTextEmptyErrorText)
	}
	if !utf8.ValidString(value) {
		return errors.New(apiTextEncodingErrorText)
	}
	if strings.TrimSpace(value) != value {
		return errors.New(apiTextWhitespaceErrorText)
	}
	runeCount := 0
	for _, character := range value {
		if character == unicode.ReplacementChar || unicode.IsControl(character) {
			return errors.New(apiTextRuneErrorText)
		}
		runeCount++
		if runeCount > maximumRunes {
			return errors.New(apiTextExtentErrorText)
		}
	}
	return nil
}

// normalizeAPIRequestID repairs caller-supplied correlation text. Trimming runs
// again after truncation because cutting at the rune bound can expose trailing
// whitespace that was interior before the cut.
func normalizeAPIRequestID(value string) string {
	trimmed := strings.TrimSpace(dropAPIControlRunes(value))
	return strings.TrimSpace(truncateAPIRunes(trimmed, APIRequestIDMaximumRunes))
}

func dropAPIControlRunes(value string) string {
	if !strings.ContainsFunc(value, unicode.IsControl) {
		return value
	}
	var builder strings.Builder
	builder.Grow(len(value))
	for _, character := range value {
		if !unicode.IsControl(character) {
			_, _ = builder.WriteRune(character) // strings.Builder.WriteRune never fails.
		}
	}
	return builder.String()
}

func truncateAPIRunes(value string, maximumRunes int) string {
	runeCount := 0
	for index := range value {
		if runeCount == maximumRunes {
			return value[:index]
		}
		runeCount++
	}
	return value
}

func apiContractError(cause error) error {
	return errors.Join(core.ErrExchangeContract, cause)
}

var (
	_ core.Validatable            = APIOutcomeUnknown
	_ core.OffWireEnum            = APIOutcomeUnknown
	_ core.ValidatedJSONMarshaler = APICodeUnknown
	_ json.Unmarshaler            = (*APICode)(nil)
	_ core.ValidatedJSONMarshaler = APIRequestID{}
	_ json.Unmarshaler            = (*APIRequestID)(nil)
	_ core.ValidatedJSONMarshaler = APIErrorBody{}
	_ core.ValidatedJSONMarshaler = APINoBody{}
	_ core.ValidatedJSONMarshaler = APIEnvelope[APINoBody]{}
)
