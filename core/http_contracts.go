package core

import (
	"errors"
	"mime"
	"net/textproto"
	"strconv"
	"strings"
)

// HTTPMethod is a closed set of admitted HTTP request methods.
type HTTPMethod uint8

const (
	// HTTPMethodUnknown is the invalid zero method.
	HTTPMethodUnknown HTTPMethod = iota
	// HTTPMethodGet is GET.
	HTTPMethodGet
	// HTTPMethodHead is HEAD.
	HTTPMethodHead
	// HTTPMethodPost is POST.
	HTTPMethodPost
	// HTTPMethodPut is PUT.
	HTTPMethodPut
	// HTTPMethodPatch is PATCH.
	HTTPMethodPatch
	// HTTPMethodDelete is DELETE.
	HTTPMethodDelete
	// HTTPMethodOptions is OPTIONS.
	HTTPMethodOptions
	httpMethodLimit
	// HTTPMethodCount is the compiler-owned size of the complete method domain,
	// including the invalid zero value.
	HTTPMethodCount = uint8(httpMethodLimit)
)

const (
	httpMethodGetText               = "GET"
	httpMethodHeadText              = "HEAD"
	httpMethodPostText              = "POST"
	httpMethodPutText               = "PUT"
	httpMethodPatchText             = "PATCH"
	httpMethodDeleteText            = "DELETE"
	httpMethodOptionsText           = "OPTIONS"
	httpMediaTypeJSONText           = "application/json"
	httpMediaTypeOctetStreamText    = "application/octet-stream"
	httpMediaTypeTextPlainText      = "text/plain"
	httpMediaTypeTimestampQueryText = "application/timestamp-query"
	httpMediaTypeTimestampReplyText = "application/timestamp-reply"
	httpMediaTypePKIXCRLText        = "application/pkix-crl"
	httpMediaTypeSyntaxErrorText    = "HTTP media type syntax is invalid"
	httpStatusRangeErrorText        = "HTTP status code is outside 100..599"
	// HTTPMethodTokenMaximumBytes bounds admitted method text.
	HTTPMethodTokenMaximumBytes = len(httpMethodOptionsText)
	// HTTPHeaderNameMaximumBytes bounds a parsed field name.
	HTTPHeaderNameMaximumBytes = 256
	// HTTPMediaTypeMaximumBytes bounds a complete media type with parameters.
	HTTPMediaTypeMaximumBytes = 4096
)

// ParseHTTPMethod accepts one exact uppercase admitted method token.
func ParseHTTPMethod(value string) (HTTPMethod, error) {
	if len(value) == 0 || len(value) > HTTPMethodTokenMaximumBytes {
		return HTTPMethodUnknown, httpContractError("HTTP method has invalid length")
	}
	for method := HTTPMethodGet; method < httpMethodLimit; method++ {
		if method.String() == value {
			return method, nil
		}
	}
	return HTTPMethodUnknown, httpContractError("HTTP method is not admitted")
}

// String returns the standard method token, or empty text when invalid.
func (m HTTPMethod) String() string {
	if m >= httpMethodLimit {
		return ""
	}
	return httpMethodFacts()[m]
}

// Validate rejects methods outside the closed domain.
func (m HTTPMethod) Validate() error {
	if m <= HTTPMethodUnknown ||
		m >= httpMethodLimit ||
		httpMethodFacts()[m] == "" {
		return httpContractError("HTTP method is invalid")
	}
	return nil
}

// IsValid reports whether m belongs to the closed method domain.
func (m HTTPMethod) IsValid() bool { return m.Validate() == nil }

func httpMethodFacts() [httpMethodLimit]string {
	return [...]string{
		HTTPMethodUnknown: "",
		HTTPMethodGet:     httpMethodGetText,
		HTTPMethodHead:    httpMethodHeadText,
		HTTPMethodPost:    httpMethodPostText,
		HTTPMethodPut:     httpMethodPutText,
		HTTPMethodPatch:   httpMethodPatchText,
		HTTPMethodDelete:  httpMethodDeleteText,
		HTTPMethodOptions: httpMethodOptionsText,
	}
}

// MarshalJSON emits the canonical uppercase method token.
func (m HTTPMethod) MarshalJSON() ([]byte, error) {
	if err := m.Validate(); err != nil {
		return nil, errors.Join(ErrJSONContract, err)
	}
	return marshalJSONString(m.String())
}

// UnmarshalJSON accepts one canonical uppercase admitted method token.
func (m *HTTPMethod) UnmarshalJSON(data []byte) error {
	if m == nil {
		return errors.Join(ErrJSONContract, errors.New("nil HTTP method receiver"))
	}
	value, err := DecodeJSONStringToken(data)
	if err != nil {
		return err
	}
	decoded, err := ParseHTTPMethod(value)
	if err != nil {
		return errors.Join(ErrJSONContract, err)
	}
	*m = decoded
	return nil
}

const (
	// HTTPStatusCodeMinimum is the first syntactically valid status code.
	HTTPStatusCodeMinimum = 100
	// HTTPStatusInformationalMaximum is the last 1xx status code.
	HTTPStatusInformationalMaximum = 199
	// HTTPStatusSuccessMinimum is the first 2xx status code.
	HTTPStatusSuccessMinimum = 200
	// HTTPStatusSuccessMaximum is the last 2xx status code.
	HTTPStatusSuccessMaximum = 299
	// HTTPStatusRedirectMinimum is the first 3xx status code.
	HTTPStatusRedirectMinimum = 300
	// HTTPStatusRedirectMaximum is the last 3xx status code.
	HTTPStatusRedirectMaximum = 399
	// HTTPStatusClientErrorMinimum is the first 4xx status code.
	HTTPStatusClientErrorMinimum = 400
	// HTTPStatusClientErrorMaximum is the last 4xx status code.
	HTTPStatusClientErrorMaximum = 499
	// HTTPStatusServerErrorMinimum is the first 5xx status code.
	HTTPStatusServerErrorMinimum = 500
	// HTTPStatusCodeMaximum is the last syntactically valid status code.
	HTTPStatusCodeMaximum = 599
)

// HTTPStatusCode is an integer in the inclusive range 100 through 599.
type HTTPStatusCode struct {
	value uint16
}

// NewHTTPStatusCode validates and constructs a status code.
func NewHTTPStatusCode(value int) (HTTPStatusCode, error) {
	if value < HTTPStatusCodeMinimum || value > HTTPStatusCodeMaximum {
		return HTTPStatusCode{}, httpContractError(httpStatusRangeErrorText)
	}
	return HTTPStatusCode{value: uint16(value)}, nil
}

// Int returns the validated status code as int.
func (s HTTPStatusCode) Int() (int, error) {
	if err := s.Validate(); err != nil {
		return 0, err
	}
	return int(s.value), nil
}

// Validate rejects unset or out-of-range status codes.
func (s HTTPStatusCode) Validate() error {
	if s.value < HTTPStatusCodeMinimum || s.value > HTTPStatusCodeMaximum {
		return httpContractError(httpStatusRangeErrorText)
	}
	return nil
}

// IsInformational reports whether s is in the 1xx class.
func (s HTTPStatusCode) IsInformational() bool {
	return s.value >= HTTPStatusCodeMinimum && s.value <= HTTPStatusInformationalMaximum
}

// IsSuccess reports whether s is in the 2xx class.
func (s HTTPStatusCode) IsSuccess() bool {
	return s.value >= HTTPStatusSuccessMinimum && s.value <= HTTPStatusSuccessMaximum
}

// IsRedirect reports whether s is in the 3xx class.
func (s HTTPStatusCode) IsRedirect() bool {
	return s.value >= HTTPStatusRedirectMinimum && s.value <= HTTPStatusRedirectMaximum
}

// IsClientError reports whether s is in the 4xx class.
func (s HTTPStatusCode) IsClientError() bool {
	return s.value >= HTTPStatusClientErrorMinimum && s.value <= HTTPStatusClientErrorMaximum
}

// IsServerError reports whether s is in the 5xx class.
func (s HTTPStatusCode) IsServerError() bool {
	return s.value >= HTTPStatusServerErrorMinimum && s.value <= HTTPStatusCodeMaximum
}

// PermitsResponseBody reports whether the status alone permits a response body.
// Informational responses, 204, and 304 never carry one. Request-method rules,
// such as HEAD suppressing a body that the status otherwise permits, remain the
// HTTP operation owner's decision.
func (s HTTPStatusCode) PermitsResponseBody() bool {
	return s.value > HTTPStatusInformationalMaximum &&
		s.value != 204 &&
		s.value != 304
}

// MarshalJSON emits the status as a canonical JSON integer.
func (s HTTPStatusCode) MarshalJSON() ([]byte, error) {
	if err := s.Validate(); err != nil {
		return nil, errors.Join(ErrJSONContract, err)
	}
	return strconv.AppendUint(nil, uint64(s.value), 10), nil
}

// UnmarshalJSON accepts a canonical JSON integer from 100 through 599.
func (s *HTTPStatusCode) UnmarshalJSON(data []byte) error {
	if s == nil {
		return errors.Join(ErrJSONContract, errors.New("nil HTTP status receiver"))
	}
	value, err := parseCanonicalUint64JSON(data)
	if err != nil {
		return err
	}
	if value > uint64(HTTPStatusCodeMaximum) {
		return errors.Join(ErrJSONContract, httpContractError(httpStatusRangeErrorText))
	}
	decoded, err := NewHTTPStatusCode(int(value))
	if err != nil {
		return errors.Join(ErrJSONContract, err)
	}
	*s = decoded
	return nil
}

// HTTPHeaderName is a canonical MIME-style HTTP field name.
type HTTPHeaderName struct {
	value string
}

// ParseHTTPHeaderName validates HTTP token syntax and canonicalizes letter case.
func ParseHTTPHeaderName(value string) (HTTPHeaderName, error) {
	if value == "" || len(value) > HTTPHeaderNameMaximumBytes {
		return HTTPHeaderName{}, httpContractError("HTTP header name has invalid length")
	}
	for index := range len(value) {
		if !isHTTPTokenByte(value[index]) {
			return HTTPHeaderName{}, httpContractError("HTTP header name contains an invalid byte")
		}
	}
	return HTTPHeaderName{value: textproto.CanonicalMIMEHeaderKey(value)}, nil
}

// String returns the canonical field name.
func (n HTTPHeaderName) String() string {
	return n.value
}

// Validate rejects the unset zero value.
func (n HTTPHeaderName) Validate() error {
	if n.value == "" {
		return httpContractError("HTTP header name is unset")
	}
	return nil
}

// MarshalJSON emits the canonical MIME-style field name.
func (n HTTPHeaderName) MarshalJSON() ([]byte, error) {
	if err := n.Validate(); err != nil {
		return nil, errors.Join(ErrJSONContract, err)
	}
	return marshalJSONString(n.String())
}

// UnmarshalJSON accepts HTTP token syntax and stores canonical MIME-style text.
func (n *HTTPHeaderName) UnmarshalJSON(data []byte) error {
	if n == nil {
		return errors.Join(ErrJSONContract, httpContractError("nil HTTP header name receiver"))
	}
	value, err := DecodeJSONStringToken(data)
	if err != nil {
		return err
	}
	decoded, err := ParseHTTPHeaderName(value)
	if err != nil {
		return errors.Join(ErrJSONContract, err)
	}
	*n = decoded
	return nil
}

const (
	httpHeaderContentTypeText     = "Content-Type"
	httpHeaderAcceptText          = "Accept"
	httpHeaderAuthorizationText   = "Authorization"
	httpHeaderRetryAfterText      = "Retry-After"
	httpHeaderContentLengthText   = "Content-Length"
	httpHeaderContentRangeText    = "Content-Range"
	httpHeaderContentEncodingText = "Content-Encoding"
	httpHeaderAcceptEncodingText  = "Accept-Encoding"
	httpHeaderIdempotencyKeyText  = "Idempotency-Key"
	httpHeaderLocationText        = "Location"
	httpHeaderCacheControlText    = "Cache-Control"
)

// HTTPHeaderContentType returns the validated Content-Type field name.
func HTTPHeaderContentType() HTTPHeaderName {
	return HTTPHeaderName{value: httpHeaderContentTypeText}
}

// HTTPHeaderAccept returns the validated Accept field name.
func HTTPHeaderAccept() HTTPHeaderName {
	return HTTPHeaderName{value: httpHeaderAcceptText}
}

// HTTPHeaderAuthorization returns the validated Authorization field name.
func HTTPHeaderAuthorization() HTTPHeaderName {
	return HTTPHeaderName{value: httpHeaderAuthorizationText}
}

// HTTPHeaderRetryAfter returns the validated Retry-After field name.
func HTTPHeaderRetryAfter() HTTPHeaderName {
	return HTTPHeaderName{value: httpHeaderRetryAfterText}
}

// HTTPHeaderContentLength returns the validated Content-Length field name.
func HTTPHeaderContentLength() HTTPHeaderName {
	return HTTPHeaderName{value: httpHeaderContentLengthText}
}

// HTTPHeaderContentRange returns the validated Content-Range field name.
func HTTPHeaderContentRange() HTTPHeaderName {
	return HTTPHeaderName{value: httpHeaderContentRangeText}
}

// HTTPHeaderContentEncoding returns the validated Content-Encoding field name.
func HTTPHeaderContentEncoding() HTTPHeaderName {
	return HTTPHeaderName{value: httpHeaderContentEncodingText}
}

// HTTPHeaderAcceptEncoding returns the validated Accept-Encoding field name.
func HTTPHeaderAcceptEncoding() HTTPHeaderName {
	return HTTPHeaderName{value: httpHeaderAcceptEncodingText}
}

// HTTPHeaderIdempotencyKey returns the validated Idempotency-Key field name.
func HTTPHeaderIdempotencyKey() HTTPHeaderName {
	return HTTPHeaderName{value: httpHeaderIdempotencyKeyText}
}

// HTTPHeaderLocation returns the validated Location field name.
func HTTPHeaderLocation() HTTPHeaderName {
	return HTTPHeaderName{value: httpHeaderLocationText}
}

// HTTPHeaderCacheControl returns the validated Cache-Control field name.
func HTTPHeaderCacheControl() HTTPHeaderName {
	return HTTPHeaderName{value: httpHeaderCacheControlText}
}

// HTTPMediaType is one canonical standard-library-parsed media type, including
// any parameters. Its zero value is unset. It is deliberately not a closed
// enum: HTTP protocols and providers define legitimate vendor media types.
type HTTPMediaType struct {
	value string
}

// ParseHTTPMediaType parses standard media-type syntax and stores the
// canonical standard-library projection.
func ParseHTTPMediaType(value string) (HTTPMediaType, error) {
	if len(value) == 0 || len(value) > HTTPMediaTypeMaximumBytes {
		return HTTPMediaType{}, httpContractError("HTTP media type has invalid length")
	}
	base, parameters, err := mime.ParseMediaType(value)
	if err != nil {
		return HTTPMediaType{}, httpContractError(httpMediaTypeSyntaxErrorText)
	}
	if strings.Count(base, "/") != 1 {
		return HTTPMediaType{}, httpContractError("HTTP media type requires type and subtype")
	}
	canonical := mime.FormatMediaType(base, parameters)
	if canonical == "" || len(canonical) > HTTPMediaTypeMaximumBytes {
		return HTTPMediaType{}, httpContractError("HTTP media type cannot be represented canonically")
	}
	return HTTPMediaType{value: canonical}, nil
}

// String returns the canonical media type and parameters.
func (m HTTPMediaType) String() string {
	return m.value
}

// Validate rejects unset or noncanonical media types.
func (m HTTPMediaType) Validate() error {
	parsed, err := ParseHTTPMediaType(m.value)
	if err != nil || parsed.value != m.value {
		return httpContractError("HTTP media type is invalid")
	}
	return nil
}

// IsValid reports whether m is a canonical parsed media type.
func (m HTTPMediaType) IsValid() bool { return m.Validate() == nil }

// IsZero reports whether no media type is set.
func (m HTTPMediaType) IsZero() bool { return m.value == "" }

// Base returns the normalized type/subtype without parameters.
func (m HTTPMediaType) Base() (string, error) {
	if err := m.Validate(); err != nil {
		return "", err
	}
	base, _, err := mime.ParseMediaType(m.value)
	if err != nil {
		return "", httpContractError(httpMediaTypeSyntaxErrorText)
	}
	return base, nil
}

// SameBase reports whether two media types share a normalized type/subtype.
func (m HTTPMediaType) SameBase(other HTTPMediaType) (bool, error) {
	left, err := m.Base()
	if err != nil {
		return false, err
	}
	right, err := other.Base()
	if err != nil {
		return false, err
	}
	return left == right, nil
}

// MarshalJSON emits the canonical media type.
func (m HTTPMediaType) MarshalJSON() ([]byte, error) {
	if err := m.Validate(); err != nil {
		return nil, errors.Join(ErrJSONContract, err)
	}
	return marshalJSONString(m.String())
}

// UnmarshalJSON accepts standard media-type syntax and stores its admitted base.
func (m *HTTPMediaType) UnmarshalJSON(data []byte) error {
	if m == nil {
		return errors.Join(ErrJSONContract, errors.New("nil HTTP media type receiver"))
	}
	value, err := DecodeJSONStringToken(data)
	if err != nil {
		return err
	}
	decoded, err := ParseHTTPMediaType(value)
	if err != nil {
		return errors.Join(ErrJSONContract, err)
	}
	*m = decoded
	return nil
}

// HTTPMediaTypeJSON returns canonical application/json.
func HTTPMediaTypeJSON() HTTPMediaType {
	return HTTPMediaType{value: httpMediaTypeJSONText}
}

// HTTPMediaTypeOctetStream returns canonical application/octet-stream.
func HTTPMediaTypeOctetStream() HTTPMediaType {
	return HTTPMediaType{value: httpMediaTypeOctetStreamText}
}

// HTTPMediaTypeTextPlain returns canonical text/plain.
func HTTPMediaTypeTextPlain() HTTPMediaType {
	return HTTPMediaType{value: httpMediaTypeTextPlainText}
}

// HTTPMediaTypeTimestampQuery returns canonical application/timestamp-query.
func HTTPMediaTypeTimestampQuery() HTTPMediaType {
	return HTTPMediaType{value: httpMediaTypeTimestampQueryText}
}

// HTTPMediaTypeTimestampReply returns canonical application/timestamp-reply.
func HTTPMediaTypeTimestampReply() HTTPMediaType {
	return HTTPMediaType{value: httpMediaTypeTimestampReplyText}
}

// HTTPMediaTypePKIXCRL returns canonical application/pkix-crl.
func HTTPMediaTypePKIXCRL() HTTPMediaType {
	return HTTPMediaType{value: httpMediaTypePKIXCRLText}
}

// ValidateHTTPFieldValue rejects field content that no HTTP message can carry.
// The admitted grammar is RFC 9110 field-value: visible ASCII, space,
// horizontal tab, and obs-text. Control bytes and DEL are refused because
// net/http refuses to transmit them, so accepting one here would defer a
// permanent contract violation into an opaque transport failure.
func ValidateHTTPFieldValue(value string) error {
	for index := range len(value) {
		if !isHTTPFieldValueByte(value[index]) {
			return httpContractError("HTTP field value contains an untransmittable byte")
		}
	}
	return nil
}

func isHTTPFieldValueByte(value byte) bool {
	const deleteByte = 0x7f
	return value == '\t' || (value >= ' ' && value != deleteByte)
}

func isHTTPTokenByte(value byte) bool {
	if value >= '0' && value <= '9' || value >= 'A' && value <= 'Z' || value >= 'a' && value <= 'z' {
		return true
	}
	switch value {
	case '!', '#', '$', '%', '&', '\'', '*', '+', '-', '.', '^', '_', '`', '|', '~':
		return true
	default:
		return false
	}
}

func httpContractError(message string) error {
	return errors.Join(ErrPrimitiveContract, errors.New(message))
}
