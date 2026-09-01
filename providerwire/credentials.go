package providerwire

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

const (
	// StripeCredentialMinimumBytes is the smallest prefix plus nonempty key
	// material Primitive admits for Stripe server credentials.
	StripeCredentialMinimumBytes = 4
	// StripeCredentialMaximumBytes is Primitive's Stripe-specific credential
	// custody ceiling where Stripe publishes no smaller wire maximum.
	StripeCredentialMaximumBytes = 4 * 1024
	// PlunkCredentialMinimumBytes is the smallest documented sk_ prefix plus
	// nonempty key material.
	PlunkCredentialMinimumBytes = 4
	// PlunkCredentialMaximumBytes is Primitive's Plunk-specific credential
	// custody ceiling where Plunk publishes no smaller wire maximum.
	PlunkCredentialMaximumBytes = 4 * 1024
	// PlunkWebhookSecretMinimumBytes requires nonempty configured bearer
	// material for one customizable Plunk webhook.
	PlunkWebhookSecretMinimumBytes = 1
	// PlunkWebhookSecretMaximumBytes is the independent Plunk webhook bearer
	// custody ceiling.
	PlunkWebhookSecretMaximumBytes = 4 * 1024
	// TwilioAPIKeySecretBytes is Twilio's exact API-key secret extent.
	TwilioAPIKeySecretBytes = 32
	// TwilioAuthTokenBytes is Twilio's exact webhook Auth Token extent.
	TwilioAuthTokenBytes = 32
	// PayPalAccessTokenMaximumBytes is Primitive's PayPal-specific OAuth token
	// custody ceiling where PayPal publishes no smaller wire maximum.
	PayPalAccessTokenMaximumBytes = 4 * 1024
)

// StripeCredentialKind closes the two server-side credential families Stripe
// documents. Restricted keys are the least-privilege production default.
type StripeCredentialKind uint8

const (
	StripeCredentialUnknown StripeCredentialKind = iota
	StripeCredentialRestricted
	StripeCredentialSecret
	stripeCredentialLimit
)

type stripeCredentialKindFact struct {
	diagnostic string
	prefix     string
}

func stripeCredentialKindFacts() [stripeCredentialLimit]stripeCredentialKindFact {
	return [...]stripeCredentialKindFact{
		StripeCredentialUnknown:    {},
		StripeCredentialRestricted: {diagnostic: "restricted", prefix: "rk_"},
		StripeCredentialSecret:     {diagnostic: "secret", prefix: "sk_"},
	}
}

func (k StripeCredentialKind) Validate() error {
	if k <= StripeCredentialUnknown || k >= stripeCredentialLimit || stripeCredentialKindFacts()[k].prefix == "" {
		return core.ErrProviderWireContract
	}
	return nil
}

func (k StripeCredentialKind) IsValid() bool { return k.Validate() == nil }

func (k StripeCredentialKind) String() string {
	if err := k.Validate(); err != nil {
		return ""
	}
	return stripeCredentialKindFacts()[k].diagnostic
}

func (StripeCredentialKind) OffWireEnum() {}

// StripeCredential is one copied, redacted Stripe server credential.
type StripeCredential struct {
	key  []byte
	kind StripeCredentialKind
}

// ParseStripeCredential admits a restricted rk_ key or secret sk_ key.
func ParseStripeCredential(value []byte) (StripeCredential, error) {
	kind := StripeCredentialUnknown
	switch {
	case bytesHavePrefix(value, "rk_"):
		kind = StripeCredentialRestricted
	case bytesHavePrefix(value, "sk_"):
		kind = StripeCredentialSecret
	}
	if kind == StripeCredentialUnknown || !validStripeCredentialBytes(value) {
		return StripeCredential{}, core.ErrProviderWireContract
	}
	return StripeCredential{key: append([]byte(nil), value...), kind: kind}, nil
}

func (c StripeCredential) Validate() error {
	if err := c.kind.Validate(); err != nil || !validStripeCredentialBytes(c.key) {
		return core.ErrProviderWireContract
	}
	prefix := stripeCredentialKindFacts()[c.kind].prefix
	if !bytesHavePrefix(c.key, prefix) {
		return core.ErrProviderWireContract
	}
	return nil
}

func (c StripeCredential) Kind() StripeCredentialKind { return c.kind }

func (c *StripeCredential) Close() error {
	if c == nil {
		return core.ErrProviderWireContract
	}
	clear(c.key)
	*c = StripeCredential{}
	return nil
}

func (StripeCredential) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, core.RedactedValueText)
}

// PlunkCredential is one copied, redacted Plunk secret key.
type PlunkCredential struct{ key []byte }

func ParsePlunkCredential(value []byte) (PlunkCredential, error) {
	if !bytesHavePrefix(value, "sk_") || !validPlunkCredentialBytes(value) {
		return PlunkCredential{}, core.ErrProviderWireContract
	}
	return PlunkCredential{key: append([]byte(nil), value...)}, nil
}

func (c PlunkCredential) Validate() error {
	if !bytesHavePrefix(c.key, "sk_") || !validPlunkCredentialBytes(c.key) {
		return core.ErrProviderWireContract
	}
	return nil
}

func (c *PlunkCredential) Close() error {
	if c == nil {
		return core.ErrProviderWireContract
	}
	clear(c.key)
	*c = PlunkCredential{}
	return nil
}

func (PlunkCredential) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, core.RedactedValueText)
}

// PlunkWebhookSecret is one copied, redacted bearer configured on a Plunk
// webhook. It is independent from Plunk's outbound sk_ API credential.
type PlunkWebhookSecret struct{ token []byte }

func ParsePlunkWebhookSecret(value []byte) (PlunkWebhookSecret, error) {
	if !validCredentialBytes(value, PlunkWebhookSecretMinimumBytes, PlunkWebhookSecretMaximumBytes) {
		return PlunkWebhookSecret{}, core.ErrProviderWireContract
	}
	return PlunkWebhookSecret{token: append([]byte(nil), value...)}, nil
}

func (s PlunkWebhookSecret) Validate() error {
	if !validCredentialBytes(s.token, PlunkWebhookSecretMinimumBytes, PlunkWebhookSecretMaximumBytes) {
		return core.ErrProviderWireContract
	}
	return nil
}

func (s *PlunkWebhookSecret) Close() error {
	if s == nil {
		return core.ErrProviderWireContract
	}
	clear(s.token)
	*s = PlunkWebhookSecret{}
	return nil
}

func (PlunkWebhookSecret) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, core.RedactedValueText)
}

// TwilioAccountSID is one canonical Account SID.
type TwilioAccountSID string

func ParseTwilioAccountSID(value string) (TwilioAccountSID, error) {
	sid := TwilioAccountSID(value)
	if err := sid.Validate(); err != nil {
		return "", err
	}
	return sid, nil
}

func (s TwilioAccountSID) Validate() error {
	if !validTwilioSID(string(s), "AC") {
		return core.ErrProviderWireContract
	}
	return nil
}

func (s TwilioAccountSID) String() string { return string(s) }

// TwilioAPIKeySID is one canonical API-key SID.
type TwilioAPIKeySID string

func ParseTwilioAPIKeySID(value string) (TwilioAPIKeySID, error) {
	sid := TwilioAPIKeySID(value)
	if err := sid.Validate(); err != nil {
		return "", err
	}
	return sid, nil
}

func (s TwilioAPIKeySID) Validate() error {
	if !validTwilioSID(string(s), "SK") {
		return core.ErrProviderWireContract
	}
	return nil
}

func (s TwilioAPIKeySID) String() string { return string(s) }

// TwilioCredential binds the account resource authority to one API key.
type TwilioCredential struct {
	AccountSID TwilioAccountSID
	APIKeySID  TwilioAPIKeySID
	secret     []byte
}

func NewTwilioCredential(account TwilioAccountSID, key TwilioAPIKeySID, secret []byte) (TwilioCredential, error) {
	candidate := TwilioCredential{AccountSID: account, APIKeySID: key, secret: append([]byte(nil), secret...)}
	if err := candidate.Validate(); err != nil {
		clear(candidate.secret)
		return TwilioCredential{}, err
	}
	return candidate, nil
}

func (c TwilioCredential) Validate() error {
	if err := errors.Join(c.AccountSID.Validate(), c.APIKeySID.Validate()); err != nil ||
		len(c.secret) != TwilioAPIKeySecretBytes || !asciiAlphanumeric(c.secret) {
		return core.ErrProviderWireContract
	}
	return nil
}

func (c *TwilioCredential) Close() error {
	if c == nil {
		return core.ErrProviderWireContract
	}
	clear(c.secret)
	*c = TwilioCredential{}
	return nil
}

func (TwilioCredential) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, core.RedactedValueText)
}

// PayPalAccessToken is one copied OAuth 2 bearer capability.
type PayPalAccessToken struct{ authorization exchange.BearerAuthorization }

func ParsePayPalAccessToken(value []byte) (PayPalAccessToken, error) {
	if len(value) == 0 || len(value) > PayPalAccessTokenMaximumBytes {
		return PayPalAccessToken{}, core.ErrProviderWireContract
	}
	authorization := exchange.BearerAuthorization{Token: append([]byte(nil), value...)}
	if err := authorization.Validate(); err != nil {
		clear(authorization.Token)
		return PayPalAccessToken{}, contractError(err)
	}
	return PayPalAccessToken{authorization: authorization}, nil
}

func (t PayPalAccessToken) Validate() error {
	if len(t.authorization.Token) == 0 || len(t.authorization.Token) > PayPalAccessTokenMaximumBytes {
		return core.ErrProviderWireContract
	}
	if err := t.authorization.Validate(); err != nil {
		return contractError(err)
	}
	return nil
}

func (t *PayPalAccessToken) Close() error {
	if t == nil {
		return core.ErrProviderWireContract
	}
	clear(t.authorization.Token)
	*t = PayPalAccessToken{}
	return nil
}

func (PayPalAccessToken) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, core.RedactedValueText)
}

func validTwilioSID(value, prefix string) bool {
	return len(value) == 34 && strings.HasPrefix(value, prefix) && hexText(value[2:])
}

func hexText(value string) bool {
	for index := range len(value) {
		character := value[index]
		if character < '0' || character > '9' {
			if character < 'a' || character > 'f' {
				if character < 'A' || character > 'F' {
					return false
				}
			}
		}
	}
	return true
}

func validStripeCredentialBytes(value []byte) bool {
	return validCredentialBytes(value, StripeCredentialMinimumBytes, StripeCredentialMaximumBytes)
}

func validPlunkCredentialBytes(value []byte) bool {
	return validCredentialBytes(value, PlunkCredentialMinimumBytes, PlunkCredentialMaximumBytes)
}

func validCredentialBytes(value []byte, minimum, maximum int) bool {
	if len(value) < minimum || len(value) > maximum {
		return false
	}
	for _, character := range value {
		if character < 0x21 || character > 0x7e {
			return false
		}
	}
	return true
}

func asciiAlphanumeric(value []byte) bool {
	for _, character := range value {
		if character >= '0' && character <= '9' ||
			character >= 'A' && character <= 'Z' ||
			character >= 'a' && character <= 'z' {
			continue
		}
		return false
	}
	return true
}

func bytesHavePrefix(value []byte, prefix string) bool {
	return len(value) >= len(prefix) && string(value[:len(prefix)]) == prefix
}

var (
	_ core.Validatable = StripeCredentialKind(0)
	_ core.OffWireEnum = StripeCredentialKind(0)
	_ core.Validatable = StripeCredential{}
	_ core.Validatable = PlunkCredential{}
	_ core.Validatable = PlunkWebhookSecret{}
	_ core.Validatable = TwilioAccountSID("")
	_ core.Validatable = TwilioAPIKeySID("")
	_ core.Validatable = TwilioCredential{}
	_ core.Validatable = PayPalAccessToken{}
)
