package awsidentity

import (
	"fmt"
	"io"

	"github.com/deliri/primitive/v2026/core"
)

const bearerPrefix = "Bearer "

// Token is one opaque AWS-acquired outbound identity bearer. Its value has no
// assertion accessor and every generic formatting surface is redacted.
type Token struct{ value *string }

func newToken(value string) (Token, error) {
	token := Token{value: &value}
	if err := token.Validate(); err != nil {
		return Token{}, err
	}
	return token, nil
}

// Validate checks extent and RFC 6750 token68 syntax without parsing claims.
func (t Token) Validate() error {
	if t.value == nil || !validBearerToken(*t.value) {
		return core.ErrAWSIdentityContract
	}
	return nil
}

// BearerValue explicitly crosses the authorization-header disclosure boundary.
func (t Token) BearerValue() (string, error) {
	if err := t.Validate(); err != nil {
		return "", err
	}
	return bearerPrefix + *t.value, nil
}

// Format redacts the token for every formatting verb.
func (Token) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, core.RedactedValueText)
}

func validBearerToken(value string) bool {
	if value == "" || len(value) > TokenMaximumBytes {
		return false
	}
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
	if value >= 'a' && value <= 'z' || value >= 'A' && value <= 'Z' || value >= '0' && value <= '9' {
		return true
	}
	switch value {
	case '-', '.', '_', '~', '+', '/':
		return true
	default:
		return false
	}
}

var (
	_ core.Validatable = Token{}
	_ fmt.Formatter    = Token{}
)
