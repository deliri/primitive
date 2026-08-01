package cloudidentity

import (
	"bytes"
	"fmt"
	"io"

	"github.com/deliri/primitive/v2026/core"
)

// bearerPrefix is the RFC 6750 section 2.1 credentials prefix. It is a
// one-package fact: Cloudidentity is the only Primitive package that produces
// an outbound Authorization value.
const bearerPrefix = "Bearer "

// GoogleCloudCommandOutputMaximumBytes bounds one token plus the longest
// line ending emitted by a provider command.
const GoogleCloudCommandOutputMaximumBytes = TokenMaximumBytes + 2

// Token is one opaque provider-acquired outbound identity bearer. Its value
// has no assertion accessor and every generic formatting surface is redacted.
type Token struct {
	value    string
	provider Provider
}

func newToken(provider Provider, value string) (Token, error) {
	token := Token{value: value, provider: provider}
	if err := token.Validate(); err != nil {
		return Token{}, err
	}
	return token, nil
}

// ParseGoogleCloudCommandOutput validates the complete stdout of one
// caller-owned Google Cloud credential command. The command lifecycle, exit
// status, and output capture remain caller-owned; Cloudidentity owns the
// bounded token syntax, provider provenance, and redacted disclosure.
func ParseGoogleCloudCommandOutput(output []byte) (Token, error) {
	if len(output) == 0 || len(output) > GoogleCloudCommandOutputMaximumBytes {
		return Token{}, core.ErrCloudIdentityContract
	}
	value := output
	if bytes.HasSuffix(value, []byte{'\n'}) {
		value = value[:len(value)-1]
		if bytes.HasSuffix(value, []byte{'\r'}) {
			value = value[:len(value)-1]
		}
	}
	return newToken(ProviderGoogleCloud, string(value))
}

// Provider returns the authority that issued the token.
func (t Token) Provider() Provider { return t.provider }

// Validate checks provenance identity, extent, and RFC 6750 token68 syntax. It
// does not parse or verify JWT claims.
func (t Token) Validate() error {
	if err := t.provider.Validate(); err != nil {
		return err
	}
	if len(t.value) == 0 || len(t.value) > TokenMaximumBytes ||
		!validBearerToken(t.value) {
		return core.ErrCloudIdentityContract
	}
	return nil
}

// BearerValue explicitly crosses the authorization-header disclosure
// boundary.
func (t Token) BearerValue() (string, error) {
	if err := t.Validate(); err != nil {
		return "", err
	}
	return bearerPrefix + t.value, nil
}

// Format redacts the token for every formatting verb.
func (Token) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, core.RedactedValueText)
}

// validBearerToken reports whether value is one RFC 6750 token68 production:
// one or more unreserved, plus, or slash bytes followed only by padding.
func validBearerToken(value string) bool {
	padding := false
	for index := range len(value) {
		if value[index] == '=' {
			if index == 0 {
				return false
			}
			padding = true
			continue
		}
		if padding || !bearerTokenByte(value[index]) {
			return false
		}
	}
	return true
}

func bearerTokenByte(value byte) bool {
	if value >= 'a' && value <= 'z' ||
		value >= 'A' && value <= 'Z' ||
		value >= '0' && value <= '9' {
		return true
	}
	switch value {
	case '-', '.', '_', '~', '+', '/':
		return true
	default:
		return false
	}
}

var _ core.Validatable = Token{}
