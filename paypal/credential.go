package paypal

import (
	"fmt"
	"io"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/secretstore"
)

type AccessToken struct{ authorization exchange.BearerAuthorization }

func ParseAccessToken(value []byte) (AccessToken, error) {
	if len(value) == 0 || len(value) > core.PayPalAccessTokenCustodyMaximumBytes {
		return AccessToken{}, core.ErrPayPalContract
	}
	authorization := exchange.BearerAuthorization{Token: append([]byte(nil), value...)}
	if err := authorization.Validate(); err != nil {
		clear(authorization.Token)
		return AccessToken{}, contractError(err)
	}
	return AccessToken{authorization: authorization}, nil
}

func AccessTokenFromSecret(value secretstore.Value) (AccessToken, error) {
	material, err := value.CopyBytes()
	if err != nil {
		return AccessToken{}, contractError(err)
	}
	defer clear(material)
	return ParseAccessToken(material)
}

func (t AccessToken) Validate() error {
	if len(t.authorization.Token) == 0 || len(t.authorization.Token) > core.PayPalAccessTokenCustodyMaximumBytes || t.authorization.Validate() != nil {
		return core.ErrPayPalContract
	}
	return nil
}
func (t *AccessToken) Close() error {
	if t == nil {
		return core.ErrPayPalContract
	}
	clear(t.authorization.Token)
	*t = AccessToken{}
	return nil
}
func (AccessToken) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, core.RedactedValueText)
}

var _ core.Validatable = AccessToken{}
