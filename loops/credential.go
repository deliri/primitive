package loops

import (
	"encoding/base64"
	"errors"
	"fmt"
	"io"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/secretstore"
)

// Credential is one Loops API key. Loops publishes no key prefix, so the SDK
// admits only bounded RFC 6750 bearer material and invents no grammar.
// https://loops.so/docs/api-reference/intro
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
	if !validVisibleASCII(c.value, core.LoopsCredentialMinimumBytes, core.LoopsCredentialCustodyMaximumBytes) {
		return core.ErrLoopsContract
	}
	if err := (exchange.BearerAuthorization{Token: c.value}).Validate(); err != nil {
		return errors.Join(core.ErrLoopsContract, err)
	}
	return nil
}

func (c *Credential) Close() error {
	if c == nil {
		return core.ErrLoopsContract
	}
	clear(c.value)
	*c = Credential{}
	return nil
}

func (Credential) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, core.RedactedValueText)
}

// WebhookSecret retains only the decoded Standard Webhooks HMAC key. The
// dashboard spelling is an optional whsec_ prefix plus strict padded base64.
// https://loops.so/docs/webhooks
type WebhookSecret struct{ key []byte }

func ParseWebhookSecret(value []byte) (WebhookSecret, error) {
	// Go's strict base64 decoder still skips CR and LF; the printable-ASCII
	// gate keeps the admitted spelling exactly the bytes the dashboard shows.
	if !validVisibleASCII(value, 1, core.LoopsWebhookSecretCustodyMaximumBytes) {
		return WebhookSecret{}, core.ErrLoopsContract
	}
	encoded := value
	if hasPrefix(encoded, webhookSecretPrefix) {
		encoded = encoded[len(webhookSecretPrefix):]
	}
	key := make([]byte, base64.StdEncoding.DecodedLen(len(encoded)))
	count, err := base64.StdEncoding.Strict().Decode(key, encoded)
	candidate := WebhookSecret{key: key[:count]}
	if err != nil {
		clear(key)
		return WebhookSecret{}, errors.Join(core.ErrLoopsContract, err)
	}
	if err := candidate.Validate(); err != nil {
		clear(key)
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
	if len(s.key) < core.LoopsWebhookSecretMinimumKeyBytes || len(s.key) > core.LoopsWebhookSecretCustodyMaximumBytes {
		return core.ErrLoopsContract
	}
	return nil
}

func (s *WebhookSecret) Close() error {
	if s == nil {
		return core.ErrLoopsContract
	}
	clear(s.key)
	*s = WebhookSecret{}
	return nil
}

func (WebhookSecret) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, core.RedactedValueText)
}

// ownedCopy gives a receiver custody independent of the caller's value.
func (s WebhookSecret) ownedCopy() (WebhookSecret, error) {
	if err := s.Validate(); err != nil {
		return WebhookSecret{}, err
	}
	return WebhookSecret{key: append([]byte(nil), s.key...)}, nil
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
