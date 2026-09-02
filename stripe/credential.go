package stripe

import (
	"fmt"
	"io"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/secretstore"
)

type CredentialKind uint8

const (
	CredentialKindUnknown CredentialKind = iota
	CredentialKindRestricted
	CredentialKindSecret
	credentialKindLimit
)

func (k CredentialKind) Validate() error {
	if k <= CredentialKindUnknown || k >= credentialKindLimit {
		return core.ErrStripeContract
	}
	return nil
}

func (k CredentialKind) IsValid() bool { return k.Validate() == nil }
func (k CredentialKind) String() string {
	if !k.IsValid() {
		return ""
	}
	return [...]string{"", "restricted_key", "secret_key"}[k]
}
func (CredentialKind) OffWireEnum() {}

type Credential struct {
	value []byte
	kind  CredentialKind
}

func ParseCredential(value []byte) (Credential, error) {
	kind := CredentialKindUnknown
	switch {
	case hasPrefix(value, restrictedKeyPrefix):
		kind = CredentialKindRestricted
	case hasPrefix(value, secretKeyPrefix):
		kind = CredentialKindSecret
	}
	candidate := Credential{value: append([]byte(nil), value...), kind: kind}
	if err := candidate.Validate(); err != nil {
		clear(candidate.value)
		return Credential{}, err
	}
	return candidate, nil
}

func CredentialFromSecret(value secretstore.Value) (Credential, error) {
	material, err := value.CopyBytes()
	if err != nil {
		return Credential{}, contractError(err)
	}
	defer clear(material)
	return ParseCredential(material)
}

func (c Credential) Validate() error {
	if err := c.kind.Validate(); err != nil || !validCredential(c.value) {
		return core.ErrStripeContract
	}
	prefix := restrictedKeyPrefix
	if c.kind == CredentialKindSecret {
		prefix = secretKeyPrefix
	}
	if !hasPrefix(c.value, prefix) {
		return core.ErrStripeContract
	}
	return nil
}

func (c Credential) Kind() CredentialKind { return c.kind }

func (c *Credential) Close() error {
	if c == nil {
		return core.ErrStripeContract
	}
	clear(c.value)
	*c = Credential{}
	return nil
}

func (Credential) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, core.RedactedValueText)
}

type WebhookSecret struct{ value []byte }

func ParseWebhookSecret(value []byte) (WebhookSecret, error) {
	candidate := WebhookSecret{value: append([]byte(nil), value...)}
	if err := candidate.Validate(); err != nil {
		clear(candidate.value)
		return WebhookSecret{}, err
	}
	return candidate, nil
}

func WebhookSecretFromSecret(value secretstore.Value) (WebhookSecret, error) {
	material, err := value.CopyBytes()
	if err != nil {
		return WebhookSecret{}, contractError(err)
	}
	defer clear(material)
	return ParseWebhookSecret(material)
}

func (s WebhookSecret) Validate() error {
	if !hasPrefix(s.value, "whsec_") || !validVisibleASCII(s.value, core.StripeWebhookSecretMinimumBytes, core.StripeWebhookSecretCustodyMaximumBytes) {
		return core.ErrStripeContract
	}
	return nil
}

func (s *WebhookSecret) Close() error {
	if s == nil {
		return core.ErrStripeContract
	}
	clear(s.value)
	*s = WebhookSecret{}
	return nil
}

func (WebhookSecret) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, core.RedactedValueText)
}

func validCredential(value []byte) bool {
	return validVisibleASCII(value, core.StripeCredentialMinimumBytes, core.StripeCredentialCustodyMaximumBytes)
}

func validVisibleASCII(value []byte, minimum, maximum int) bool {
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

func hasPrefix(value []byte, prefix string) bool {
	return len(value) >= len(prefix) && string(value[:len(prefix)]) == prefix
}

var (
	_ core.Validatable = CredentialKind(0)
	_ core.OffWireEnum = CredentialKind(0)
	_ core.Validatable = Credential{}
	_ core.Validatable = WebhookSecret{}
)
