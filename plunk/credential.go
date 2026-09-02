package plunk

import (
	"fmt"
	"io"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/secretstore"
)

type Credential struct{ value []byte }

func ParseCredential(value []byte) (Credential, error) {
	candidate := Credential{value: append([]byte(nil), value...)}
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
	if !hasPrefix(c.value, "sk_") || !validVisibleASCII(c.value, core.PlunkCredentialMinimumBytes, core.PlunkCredentialCustodyMaximumBytes) {
		return core.ErrPlunkContract
	}
	return nil
}

func (c *Credential) Close() error {
	if c == nil {
		return core.ErrPlunkContract
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
	if !validVisibleASCII(s.value, core.PlunkWebhookSecretMinimumBytes, core.PlunkWebhookSecretCustodyMaximumBytes) {
		return core.ErrPlunkContract
	}
	return nil
}

func (s *WebhookSecret) Close() error {
	if s == nil {
		return core.ErrPlunkContract
	}
	clear(s.value)
	*s = WebhookSecret{}
	return nil
}

func (WebhookSecret) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, core.RedactedValueText)
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
	_ core.Validatable = Credential{}
	_ core.Validatable = WebhookSecret{}
)
