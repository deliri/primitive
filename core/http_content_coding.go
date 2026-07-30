package core

import (
	"errors"
	"strings"
)

const (
	httpContentCodingIdentityText = "identity"
	// HTTPContentCodingMaximumBytes bounds one content-coding token.
	HTTPContentCodingMaximumBytes = 256
)

// HTTPContentCoding is one canonical HTTP content-coding token. Its zero value
// is unset. The domain is open because protocols may register extension
// codings.
type HTTPContentCoding struct {
	value string
}

// ParseHTTPContentCoding validates one token and stores its lowercase wire
// projection.
func ParseHTTPContentCoding(value string) (HTTPContentCoding, error) {
	if value == "" || len(value) > HTTPContentCodingMaximumBytes {
		return HTTPContentCoding{}, httpContractError("HTTP content coding has invalid length")
	}
	for index := range len(value) {
		if !isHTTPTokenByte(value[index]) {
			return HTTPContentCoding{}, httpContractError("HTTP content coding contains an invalid byte")
		}
	}
	return HTTPContentCoding{value: strings.ToLower(value)}, nil
}

// Validate rejects unset or noncanonical content codings.
func (c HTTPContentCoding) Validate() error {
	parsed, err := ParseHTTPContentCoding(c.value)
	if err != nil || parsed != c {
		return httpContractError("HTTP content coding is invalid")
	}
	return nil
}

// String returns the canonical wire token.
func (c HTTPContentCoding) String() string {
	return c.value
}

// IsZero reports whether no content coding is set.
func (c HTTPContentCoding) IsZero() bool {
	return c.value == ""
}

// MarshalJSON emits the canonical content-coding token.
func (c HTTPContentCoding) MarshalJSON() ([]byte, error) {
	if err := c.Validate(); err != nil {
		return nil, errors.Join(ErrJSONContract, err)
	}
	return marshalJSONString(c.String())
}

// UnmarshalJSON accepts one content-coding token without mutating on failure.
func (c *HTTPContentCoding) UnmarshalJSON(data []byte) error {
	if c == nil {
		return errors.Join(ErrJSONContract, httpContractError("nil HTTP content coding receiver"))
	}
	value, err := DecodeJSONStringToken(data)
	if err != nil {
		return err
	}
	decoded, err := ParseHTTPContentCoding(value)
	if err != nil {
		return errors.Join(ErrJSONContract, err)
	}
	*c = decoded
	return nil
}

// HTTPContentCodingIdentity returns the no-transformation coding.
func HTTPContentCodingIdentity() HTTPContentCoding {
	return HTTPContentCoding{value: httpContentCodingIdentityText}
}

var (
	_ Validatable            = HTTPContentCoding{}
	_ ValidatedJSONMarshaler = HTTPContentCoding{}
)
