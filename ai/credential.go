package ai

import (
	"errors"
	"fmt"
	"io"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/secretstore"
)

// Credential owns provider-bound secret bytes. Formatting is always redacted.
// Close clears these owned bytes; it cannot clear immutable copies required
// by Go's native HTTP header representation at the Exchange execution leaf.
// Source: https://pkg.go.dev/net/http#Header
type Credential struct {
	value    []byte
	provider Provider
}

// Provider exposes the closed authority binding without projecting key bytes.
func (c Credential) Provider() (Provider, error) {
	if err := c.Validate(); err != nil {
		return ProviderUnknown, err
	}
	return c.provider, nil
}

// ParseCredential validates the provider-specific header contract and copies
// caller material. OpenAI bearer authentication and Anthropic's supported key
// fallback keep separate admission rules.
// Sources: https://developers.openai.com/api/reference/overview#authentication
// https://platform.claude.com/docs/en/manage-claude/authentication
func ParseCredential(provider Provider, material []byte) (Credential, error) {
	if err := provider.Validate(); err != nil {
		return Credential{}, err
	}
	if err := validateMaterial(provider, material); err != nil {
		return Credential{}, errors.Join(core.ErrAIAuthentication, err)
	}
	return Credential{provider: provider, value: append([]byte(nil), material...)}, nil
}

// CredentialFromSecret copies borrowed secret-store material into credential
// custody, clearing the intermediate byte copy before returning.
func CredentialFromSecret(provider Provider, value secretstore.Value) (Credential, error) {
	material, err := value.CopyBytes()
	if err != nil {
		return Credential{}, errors.Join(core.ErrAIAuthentication, err)
	}
	defer clear(material)
	return ParseCredential(provider, material)
}

// Validate refuses absent, closed or malformed provider-bound material.
func (c Credential) Validate() error {
	if c.provider.Validate() != nil {
		return core.ErrAIAuthentication
	}
	if err := validateMaterial(c.provider, c.value); err != nil {
		return errors.Join(core.ErrAIAuthentication, err)
	}
	return nil
}

// Close clears owned byte material and revokes the credential. Copies of this
// value share its backing bytes and must not be used concurrently with Close.
// HTTPServer acquires its own copy and enforces in-flight custody independently.
func (c *Credential) Close() error {
	if c == nil {
		return core.ErrAIContract
	}
	clear(c.value)
	*c = Credential{}
	return nil
}

// Format renders only the compiler-owned redaction token.
func (Credential) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, core.RedactedValueText)
}

// Anthropic's API-key header carries opaque visible ASCII, not RFC 6750 bearer
// grammar. Header size is a native header custody agreement, not a stream quota.
// https://platform.claude.com/docs/en/manage-claude/authentication
func validateMaterial(provider Provider, material []byte) error {
	switch provider {
	case OpenAI:
		return (exchange.BearerAuthorization{Token: material}).Validate()
	case Anthropic:
		name, err := core.ParseHTTPHeaderName(core.AIAnthropicKeyHeader)
		if err != nil {
			return err
		}
		return (exchange.SecretHeader{Name: name, Value: material, Scheme: exchange.SecretHeaderLiteral}).Validate()
	default:
		return core.ErrAIAuthentication
	}
}
