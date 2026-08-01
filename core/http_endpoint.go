package core

import (
	"errors"
	"net/url"
	"strconv"
	"strings"
)

const (
	// HTTPEndpointMaximumBytes bounds one absolute HTTP target.
	HTTPEndpointMaximumBytes = 16 * 1024
	httpSchemeText           = "http"
	httpsSchemeText          = "https"
	httpDefaultPortText      = "80"
	httpsDefaultPortText     = "443"
)

// HTTPEndpoint is one absolute, credential-free HTTP or HTTPS URL. It retains
// the standard library's parsed representation and returns value copies to
// callers.
type HTTPEndpoint struct {
	value url.URL
	set   bool
}

// ParseHTTPEndpoint parses and confines one absolute HTTP target.
func ParseHTTPEndpoint(value string) (HTTPEndpoint, error) {
	if len(value) == 0 || len(value) > HTTPEndpointMaximumBytes {
		return HTTPEndpoint{}, httpContractError("HTTP endpoint has invalid length")
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return HTTPEndpoint{}, httpContractError("HTTP endpoint syntax is invalid")
	}
	if err := validateHTTPURL(parsed); err != nil {
		return HTTPEndpoint{}, err
	}
	return HTTPEndpoint{value: *parsed, set: true}, nil
}

// Validate rejects unset, relative, credential-bearing, fragmented, or
// otherwise unusable HTTP targets.
func (e HTTPEndpoint) Validate() error {
	if !e.set {
		return httpContractError("HTTP endpoint is unset")
	}
	return validateHTTPURL(&e.value)
}

// HTTPURL returns a value copy of the parsed standard-library URL.
func (e HTTPEndpoint) HTTPURL() url.URL {
	return e.value
}

// SameOrigin reports whether both endpoints share one HTTP origin after
// normalizing scheme and host case and the default HTTP/HTTPS ports.
func (e HTTPEndpoint) SameOrigin(other HTTPEndpoint) bool {
	if err := e.Validate(); err != nil {
		return false
	}
	if err := other.Validate(); err != nil {
		return false
	}
	return strings.EqualFold(e.value.Scheme, other.value.Scheme) &&
		strings.EqualFold(e.value.Hostname(), other.value.Hostname()) &&
		normalizedHTTPPort(e.value) == normalizedHTTPPort(other.value)
}

// String returns the standard-library URL projection.
func (e HTTPEndpoint) String() string {
	if err := e.Validate(); err != nil {
		return ""
	}
	return e.value.String()
}

// MarshalJSON emits one absolute endpoint.
func (e HTTPEndpoint) MarshalJSON() ([]byte, error) {
	if err := e.Validate(); err != nil {
		return nil, errors.Join(ErrJSONContract, err)
	}
	return MarshalCanonicalJSONString(e.String())
}

// UnmarshalJSON parses one absolute endpoint without mutating on failure.
func (e *HTTPEndpoint) UnmarshalJSON(data []byte) error {
	if e == nil {
		return errors.Join(ErrJSONContract, httpContractError("nil HTTP endpoint receiver"))
	}
	value, err := DecodeJSONStringToken(data)
	if err != nil {
		return err
	}
	parsed, err := ParseHTTPEndpoint(value)
	if err != nil {
		return errors.Join(ErrJSONContract, err)
	}
	*e = parsed
	return nil
}

func validateHTTPURL(value *url.URL) error {
	if value == nil || !value.IsAbs() || value.Opaque != "" {
		return httpContractError("HTTP endpoint must be an absolute hierarchical URL")
	}
	if err := validateHTTPURLScheme(value); err != nil {
		return err
	}
	if err := validateHTTPURLAuthority(value); err != nil {
		return err
	}
	return validateHTTPURLPort(value)
}

func validateHTTPURLScheme(value *url.URL) error {
	scheme := strings.ToLower(value.Scheme)
	if scheme != httpSchemeText && scheme != httpsSchemeText {
		return httpContractError("HTTP endpoint scheme is not HTTP or HTTPS")
	}
	if value.Scheme != scheme {
		return httpContractError("HTTP endpoint scheme is not canonical")
	}
	return nil
}

func validateHTTPURLAuthority(value *url.URL) error {
	if value.Host == "" || value.Hostname() == "" {
		return httpContractError("HTTP endpoint host is empty")
	}
	if value.User != nil {
		return httpContractError("HTTP endpoint contains user information")
	}
	if value.Fragment != "" || value.RawFragment != "" {
		return httpContractError("HTTP endpoint contains a fragment")
	}
	if strings.ContainsAny(value.Host, "\r\n\t ") {
		return httpContractError("HTTP endpoint host contains whitespace")
	}
	return nil
}

func validateHTTPURLPort(value *url.URL) error {
	port := value.Port()
	if port == "" {
		return nil
	}
	number, err := strconv.ParseUint(port, 10, 16)
	if err != nil || number == 0 {
		return httpContractError("HTTP endpoint port is invalid")
	}
	return nil
}

func normalizedHTTPPort(value url.URL) string {
	if value.Port() != "" {
		return value.Port()
	}
	if value.Scheme == httpSchemeText {
		return httpDefaultPortText
	}
	return httpsDefaultPortText
}

var (
	_ Validatable            = HTTPEndpoint{}
	_ ValidatedJSONMarshaler = HTTPEndpoint{}
)
