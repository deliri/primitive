package exchange

import (
	"crypto/subtle"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/deliri/primitive/v2026/core"
)

const (
	// BearerAuthorizationTokenMaximumBytes bounds one OAuth-style bearer token.
	BearerAuthorizationTokenMaximumBytes = 4 * 1024
	// BearerAuthorizationScheme is the canonical HTTP authentication scheme
	// shared by RFC 6750 and provider-owned opaque bearer protocols.
	BearerAuthorizationScheme = "Bearer"
	// BearerAuthorizationHeaderMaximumBytes bounds the complete encoded value.
	BearerAuthorizationHeaderMaximumBytes = len(BearerAuthorizationScheme) + 1 + BearerAuthorizationTokenMaximumBytes
)

// BearerAuthorization is one caller-custodied RFC 6750 bearer token. The
// caller retains and clears Token after construction or authentication.
type BearerAuthorization struct {
	Token []byte
}

// Validate rejects absent, oversized, or non-b64token bearer material.
func (a BearerAuthorization) Validate() error {
	if len(a.Token) == 0 || len(a.Token) > BearerAuthorizationTokenMaximumBytes {
		return core.ErrExchangeContract
	}
	padding := false
	for index, value := range a.Token {
		if value == '=' {
			if index == 0 {
				return core.ErrExchangeContract
			}
			padding = true
			continue
		}
		if padding || !bearerTokenByte(value) {
			return core.ErrExchangeContract
		}
	}
	return nil
}

// Format prevents generic diagnostics from disclosing bearer material.
func (BearerAuthorization) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, core.RedactedValueText)
}

// NewBearerAuthorizationHeader constructs one redacted Authorization field.
func NewBearerAuthorizationHeader(authorization BearerAuthorization) (Header, error) {
	if err := authorization.Validate(); err != nil {
		return Header{}, err
	}
	value, err := NewHeaderValue(BearerAuthorizationScheme + " " + string(authorization.Token))
	if err != nil {
		return Header{}, err
	}
	name, err := StandardHeaderAuthorization.Name()
	if err != nil {
		return Header{}, err
	}
	header := Header{Name: name, Values: []HeaderValue{value}}
	return header, header.Validate()
}

// ReceiveBearerAuthorization returns one copied token from an exact standard
// Authorization header. The caller owns and clears the returned bytes.
func ReceiveBearerAuthorization(call SocketServerCall) (BearerAuthorization, error) {
	var zero BearerAuthorization
	if err := call.Validate(); err != nil {
		return zero, requestError(err)
	}
	name, err := StandardHeaderAuthorization.Name()
	if err != nil {
		return zero, requestError(err)
	}
	maximum, err := core.NewByteCount(uint64(BearerAuthorizationHeaderMaximumBytes))
	if err != nil {
		return zero, requestError(err)
	}
	value, err := call.UniqueHeader(name, maximum)
	if err != nil {
		return zero, requestError(err)
	}
	header, err := value.Value()
	if err != nil {
		return zero, requestError(err)
	}
	scheme, tokenText, found := strings.Cut(header, " ")
	if !found || !strings.EqualFold(scheme, BearerAuthorizationScheme) {
		return zero, requestError(core.ErrExchangeContract)
	}
	token := []byte(tokenText)
	authorization := BearerAuthorization{Token: token}
	if err := authorization.Validate(); err != nil {
		clear(token)
		return zero, requestError(err)
	}
	return authorization, nil
}

// BearerAuthorizationMatches compares two validated tokens without leaking the
// first differing byte through comparison timing.
func BearerAuthorizationMatches(left, right BearerAuthorization) (bool, error) {
	if err := errors.Join(left.Validate(), right.Validate()); err != nil {
		return false, err
	}
	return subtle.ConstantTimeCompare(left.Token, right.Token) == 1, nil
}

func bearerTokenByte(value byte) bool {
	return value >= 'A' && value <= 'Z' ||
		value >= 'a' && value <= 'z' ||
		value >= '0' && value <= '9' ||
		strings.ContainsRune("-._~+/", rune(value))
}

var (
	_ core.Validatable = BearerAuthorization{}
	_ fmt.Formatter    = BearerAuthorization{}
)
