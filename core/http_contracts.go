package core

import (
	"errors"
	"mime"
	"net/textproto"
	"strconv"
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
	switch m {
	case HTTPMethodGet:
		return httpMethodGetText
	case HTTPMethodHead:
		return httpMethodHeadText
	case HTTPMethodPost:
		return httpMethodPostText
	case HTTPMethodPut:
		return httpMethodPutText
	case HTTPMethodPatch:
		return httpMethodPatchText
	case HTTPMethodDelete:
		return httpMethodDeleteText
	case HTTPMethodOptions:
		return httpMethodOptionsText
	default:
		return ""
	}
}

// Validate rejects methods outside the closed domain.
func (m HTTPMethod) Validate() error {
	if m <= HTTPMethodUnknown || m >= httpMethodLimit {
		return httpContractError("HTTP method is invalid")
	}
	return nil
}

// IsValid reports whether m belongs to the closed method domain.
func (m HTTPMethod) IsValid() bool { return m.Validate() == nil }

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
	value, err := decodeJSONString(data)
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
	value, err := decodeJSONString(data)
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

// HTTPMediaType is a closed set of normalized media type bases. Parameters are
// accepted by ParseHTTPMediaType but are not retained.
type HTTPMediaType uint8

const (
	// HTTPMediaTypeUnknown is the invalid zero media type.
	HTTPMediaTypeUnknown HTTPMediaType = iota
	// HTTPMediaTypeJSON is application/json.
	HTTPMediaTypeJSON
	// HTTPMediaTypeOctetStream is application/octet-stream.
	HTTPMediaTypeOctetStream
	// HTTPMediaTypeTextPlain is text/plain.
	HTTPMediaTypeTextPlain
	// HTTPMediaTypeTimestampQuery is application/timestamp-query.
	HTTPMediaTypeTimestampQuery
	// HTTPMediaTypeTimestampReply is application/timestamp-reply.
	HTTPMediaTypeTimestampReply
	// HTTPMediaTypePKIXCRL is application/pkix-crl.
	HTTPMediaTypePKIXCRL
	httpMediaTypeLimit
)

// ParseHTTPMediaType parses standard media-type syntax, including parameters
// and case-insensitive type/subtype text, then returns an admitted base.
func ParseHTTPMediaType(value string) (HTTPMediaType, error) {
	if len(value) == 0 || len(value) > HTTPMediaTypeMaximumBytes {
		return HTTPMediaTypeUnknown, httpContractError("HTTP media type has invalid length")
	}
	base, _, err := mime.ParseMediaType(value)
	if err != nil {
		return HTTPMediaTypeUnknown, httpContractError("HTTP media type syntax is invalid")
	}
	for mediaType := HTTPMediaTypeJSON; mediaType < httpMediaTypeLimit; mediaType++ {
		if mediaType.String() == base {
			return mediaType, nil
		}
	}
	return HTTPMediaTypeUnknown, httpContractError("HTTP media type is not admitted")
}

// String returns the canonical lowercase media-type base.
func (m HTTPMediaType) String() string {
	switch m {
	case HTTPMediaTypeJSON:
		return httpMediaTypeJSONText
	case HTTPMediaTypeOctetStream:
		return httpMediaTypeOctetStreamText
	case HTTPMediaTypeTextPlain:
		return httpMediaTypeTextPlainText
	case HTTPMediaTypeTimestampQuery:
		return httpMediaTypeTimestampQueryText
	case HTTPMediaTypeTimestampReply:
		return httpMediaTypeTimestampReplyText
	case HTTPMediaTypePKIXCRL:
		return httpMediaTypePKIXCRLText
	default:
		return ""
	}
}

// Validate rejects media types outside the closed domain.
func (m HTTPMediaType) Validate() error {
	if m <= HTTPMediaTypeUnknown || m >= httpMediaTypeLimit {
		return httpContractError("HTTP media type is invalid")
	}
	return nil
}

// IsValid reports whether m belongs to the closed media-type domain.
func (m HTTPMediaType) IsValid() bool { return m.Validate() == nil }

// MarshalJSON emits the canonical lowercase media-type base without parameters.
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
	value, err := decodeJSONString(data)
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
