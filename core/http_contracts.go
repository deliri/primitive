package core

import (
	"errors"
	"mime"
	"net/http"
	"net/textproto"
	"strconv"
	"strings"
)

const (
	httpMediaTypeOctetStreamText = "application/octet-stream"
	httpMediaTypeSyntaxErrorText = "HTTP media type syntax is invalid"
	httpStatusRangeErrorText     = "HTTP status code is outside 100..599"
	// httpHeaderNameMaximumBytes bounds a parsed field name.
	httpHeaderNameMaximumBytes = 256
	// httpMediaTypeMaximumBytes bounds a complete media type with parameters.
	httpMediaTypeMaximumBytes = 4096
)

const (
	// httpStatusCodeMinimum is the first syntactically valid status code.
	httpStatusCodeMinimum = 100
	// httpStatusInformationalMaximum is the last 1xx status code.
	httpStatusInformationalMaximum = 199
	// httpStatusSuccessMinimum is the first 2xx status code.
	httpStatusSuccessMinimum = 200
	// httpStatusSuccessMaximum is the last 2xx status code.
	httpStatusSuccessMaximum = 299
	// httpStatusRedirectMinimum is the first 3xx status code.
	httpStatusRedirectMinimum = 300
	// httpStatusRedirectMaximum is the last 3xx status code.
	httpStatusRedirectMaximum = 399
	// httpStatusClientErrorMinimum is the first 4xx status code.
	httpStatusClientErrorMinimum = 400
	// httpStatusClientErrorMaximum is the last 4xx status code.
	httpStatusClientErrorMaximum = 499
	// httpStatusServerErrorMinimum is the first 5xx status code.
	httpStatusServerErrorMinimum = 500
	// httpStatusCodeMaximum is the last syntactically valid status code.
	httpStatusCodeMaximum = 599
)

// HTTPStatusCode is an integer in the inclusive range 100 through 599.
type HTTPStatusCode struct {
	value uint16
}

// AdmitInt validates value as a status code and stores it on the receiver.
// It is the one admission door for a numeric code arriving from outside,
// such as a transport response; a code a caller expects by contract is a
// named constructor instead. The receiver is unchanged on rejection.
func (s *HTTPStatusCode) AdmitInt(value int) error {
	if s == nil {
		return httpContractError("nil HTTP status receiver")
	}
	if value < httpStatusCodeMinimum || value > httpStatusCodeMaximum {
		return httpContractError(httpStatusRangeErrorText)
	}
	s.value = uint16(value)
	return nil
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
	if s.value < httpStatusCodeMinimum || s.value > httpStatusCodeMaximum {
		return httpContractError(httpStatusRangeErrorText)
	}
	return nil
}

// IsInformational reports whether s is in the 1xx class.
func (s HTTPStatusCode) IsInformational() bool {
	return s.value >= httpStatusCodeMinimum && s.value <= httpStatusInformationalMaximum
}

// IsSuccess reports whether s is in the 2xx class.
func (s HTTPStatusCode) IsSuccess() bool {
	return s.value >= httpStatusSuccessMinimum && s.value <= httpStatusSuccessMaximum
}

// IsRedirect reports whether s is in the 3xx class.
func (s HTTPStatusCode) IsRedirect() bool {
	return s.value >= httpStatusRedirectMinimum && s.value <= httpStatusRedirectMaximum
}

// IsClientError reports whether s is in the 4xx class.
func (s HTTPStatusCode) IsClientError() bool {
	return s.value >= httpStatusClientErrorMinimum && s.value <= httpStatusClientErrorMaximum
}

// IsServerError reports whether s is in the 5xx class.
func (s HTTPStatusCode) IsServerError() bool {
	return s.value >= httpStatusServerErrorMinimum && s.value <= httpStatusCodeMaximum
}

// PermitsResponseBody reports whether the status alone permits a response body.
// Informational responses, 204, and 304 never carry one. Request-method rules,
// such as HEAD suppressing a body that the status otherwise permits, remain the
// HTTP operation owner's decision.
func (s HTTPStatusCode) PermitsResponseBody() bool {
	return s.value > httpStatusInformationalMaximum &&
		s.value != http.StatusNoContent &&
		s.value != http.StatusNotModified
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
	if value > uint64(httpStatusCodeMaximum) {
		return errors.Join(ErrJSONContract, httpContractError(httpStatusRangeErrorText))
	}
	if err := s.AdmitInt(int(value)); err != nil {
		return errors.Join(ErrJSONContract, err)
	}
	return nil
}

// HTTPStatusOK returns the validated 200 OK success status: the one code a
// caller that accepts exactly one response shape names as its expected
// status.
func HTTPStatusOK() HTTPStatusCode {
	return HTTPStatusCode{value: http.StatusOK}
}

// HTTPHeaderName is a canonical MIME-style HTTP field name.
type HTTPHeaderName struct {
	value string
}

// ParseHTTPHeaderName validates HTTP token syntax and canonicalizes letter case.
func ParseHTTPHeaderName(value string) (HTTPHeaderName, error) {
	if value == "" || len(value) > httpHeaderNameMaximumBytes {
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
	return MarshalCanonicalJSONString(n.String())
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
	httpHeaderContentLengthText   = "Content-Length"
	httpHeaderContentEncodingText = "Content-Encoding"
	httpHeaderAcceptEncodingText  = "Accept-Encoding"
	httpHeaderIdempotencyKeyText  = "Idempotency-Key"
)

// HTTPHeaderContentType returns the validated Content-Type field name.
func HTTPHeaderContentType() HTTPHeaderName {
	return HTTPHeaderName{value: httpHeaderContentTypeText}
}

// HTTPHeaderAccept returns the validated Accept field name.
func HTTPHeaderAccept() HTTPHeaderName {
	return HTTPHeaderName{value: httpHeaderAcceptText}
}

// HTTPHeaderContentLength returns the validated Content-Length field name.
func HTTPHeaderContentLength() HTTPHeaderName {
	return HTTPHeaderName{value: httpHeaderContentLengthText}
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

// HTTPMediaType is one canonical standard-library-parsed media type, including
// any parameters. Its zero value is unset. It is deliberately not a closed
// enum: HTTP protocols and providers define legitimate vendor media types.
type HTTPMediaType struct {
	value string
}

// ParseHTTPMediaType parses standard media-type syntax and stores the
// canonical standard-library projection.
func ParseHTTPMediaType(value string) (HTTPMediaType, error) {
	if len(value) == 0 || len(value) > httpMediaTypeMaximumBytes {
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
	if canonical == "" || len(canonical) > httpMediaTypeMaximumBytes {
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
	return MarshalCanonicalJSONString(m.String())
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

// HTTPMediaTypeOctetStream returns canonical application/octet-stream.
func HTTPMediaTypeOctetStream() HTTPMediaType {
	return HTTPMediaType{value: httpMediaTypeOctetStreamText}
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
