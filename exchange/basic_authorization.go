package exchange

import (
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"unicode"
	"unicode/utf8"

	"github.com/deliri/primitive/v2026/core"
)

// BasicAuthorizationIdentity is one bounded UTF-8 Basic authentication
// identity. It excludes the colon delimiter and control characters before a
// secret is accessed or a header is constructed.
type BasicAuthorizationIdentity string

// ParseBasicAuthorizationIdentity admits one Basic authentication identity.
func ParseBasicAuthorizationIdentity(value string) (BasicAuthorizationIdentity, error) {
	identity := BasicAuthorizationIdentity(value)
	if err := identity.Validate(); err != nil {
		return "", err
	}
	return identity, nil
}

func (i BasicAuthorizationIdentity) Validate() error {
	value := []byte(i)
	if len(value) == 0 || len(value) > BasicAuthorizationIdentityMaximumBytes ||
		!utf8.Valid(value) || invalidBasicIdentity(value) {
		return core.ErrExchangeContract
	}
	return nil
}

func (i BasicAuthorizationIdentity) String() string { return string(i) }

func (i BasicAuthorizationIdentity) MarshalJSON() ([]byte, error) {
	if err := i.Validate(); err != nil {
		return nil, errors.Join(core.ErrJSONContract, err)
	}
	return core.MarshalCanonicalJSONString(i.String())
}

func (i *BasicAuthorizationIdentity) UnmarshalJSON(data []byte) error {
	if i == nil {
		return errors.Join(core.ErrJSONContract, core.ErrExchangeContract)
	}
	value, err := core.DecodeJSONStringToken(data)
	if err != nil {
		return err
	}
	identity, err := ParseBasicAuthorizationIdentity(value)
	if err != nil {
		return errors.Join(core.ErrJSONContract, err)
	}
	*i = identity
	return nil
}

const (
	// BasicAuthorizationIdentityMaximumBytes bounds one Basic identity.
	BasicAuthorizationIdentityMaximumBytes = 255
	// BasicAuthorizationSecretMaximumBytes bounds one Basic secret.
	BasicAuthorizationSecretMaximumBytes = 1024
	basicAuthorizationScheme             = "Basic "
	basicAuthorizationRawMaximumBytes    = BasicAuthorizationIdentityMaximumBytes + 1 + BasicAuthorizationSecretMaximumBytes
	basicAuthorizationBase64MaximumBytes = (basicAuthorizationRawMaximumBytes + 2) / 3 * 4
	// BasicAuthorizationHeaderMaximumBytes bounds the complete encoded value.
	BasicAuthorizationHeaderMaximumBytes = len(basicAuthorizationScheme) + basicAuthorizationBase64MaximumBytes
)

// BasicAuthorizationRequest supplies caller-custodied UTF-8 credentials for
// one standard Authorization header. The caller retains and clears both byte
// slices after construction.
type BasicAuthorizationRequest struct {
	Identity BasicAuthorizationIdentity
	Secret   []byte
}

// BasicAuthorizationReceiveCall supplies one real HTTP server request.
type BasicAuthorizationReceiveCall struct {
	Request *http.Request
}

func (call BasicAuthorizationReceiveCall) Validate() error {
	if call.Request == nil {
		return requestError(core.ErrExchangeContract)
	}
	headerName, err := StandardHeaderAuthorization.Name()
	if err != nil {
		return requestError(err)
	}
	values := call.Request.Header.Values(headerName.String())
	if len(values) != 1 || len(values[0]) > BasicAuthorizationHeaderMaximumBytes {
		return requestError(core.ErrExchangeContract)
	}
	return nil
}

func (r BasicAuthorizationRequest) Validate() error {
	if err := r.Identity.Validate(); err != nil {
		return err
	}
	if len(r.Secret) == 0 || len(r.Secret) > BasicAuthorizationSecretMaximumBytes || !utf8.Valid(r.Secret) {
		return core.ErrExchangeContract
	}
	if invalidBasicSecret(r.Secret) {
		return core.ErrExchangeContract
	}
	return nil
}

// Format prevents generic diagnostics from disclosing credentials.
func (BasicAuthorizationRequest) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, core.RedactedValueText)
}

// NewBasicAuthorizationHeader constructs the standard redacted header while
// clearing every temporary raw and encoded buffer it owns.
func NewBasicAuthorizationHeader(request BasicAuthorizationRequest) (Header, error) {
	if err := request.Validate(); err != nil {
		return Header{}, err
	}
	var rawStorage [basicAuthorizationRawMaximumBytes]byte
	raw := rawStorage[:0]
	raw = append(raw, request.Identity.String()...)
	raw = append(raw, ':')
	raw = append(raw, request.Secret...)
	defer clear(rawStorage[:])
	var encodedStorage [basicAuthorizationBase64MaximumBytes]byte
	encoded := encodedStorage[:base64.StdEncoding.EncodedLen(len(raw))]
	base64.StdEncoding.Encode(encoded, raw)
	defer clear(encodedStorage[:])
	value, err := NewHeaderValue(basicAuthorizationScheme + string(encoded))
	if err != nil {
		return Header{}, errors.Join(core.ErrExchangeContract, err)
	}
	name, err := StandardHeaderAuthorization.Name()
	if err != nil {
		return Header{}, err
	}
	header := Header{Name: name, Values: []HeaderValue{value}}
	if err := header.Validate(); err != nil {
		return Header{}, err
	}
	return header, nil
}

// ReceiveBasicAuthorization decodes and validates one standard Basic header.
// The caller owns and clears the returned secret after authentication.
func ReceiveBasicAuthorization(call BasicAuthorizationReceiveCall) (BasicAuthorizationRequest, error) {
	var zero BasicAuthorizationRequest
	if err := call.Validate(); err != nil {
		return zero, err
	}
	identityText, secretText, ok := call.Request.BasicAuth()
	if !ok {
		return zero, requestError(core.ErrExchangeContract)
	}
	identity, err := ParseBasicAuthorizationIdentity(identityText)
	if err != nil {
		return zero, requestError(err)
	}
	secret := []byte(secretText)
	received := BasicAuthorizationRequest{Identity: identity, Secret: secret}
	if err := received.Validate(); err != nil {
		clear(secret)
		return zero, requestError(err)
	}
	return received, nil
}

func invalidBasicIdentity(value []byte) bool {
	for _, character := range string(value) {
		if character == ':' || unicode.IsControl(character) {
			return true
		}
	}
	return false
}

func invalidBasicSecret(value []byte) bool {
	for _, character := range string(value) {
		if unicode.IsControl(character) {
			return true
		}
	}
	return false
}

var (
	_ core.Validatable            = BasicAuthorizationIdentity("")
	_ core.ValidatedJSONMarshaler = BasicAuthorizationIdentity("")
	_ core.Validatable            = BasicAuthorizationRequest{}
	_ core.Validatable            = BasicAuthorizationReceiveCall{}
)
